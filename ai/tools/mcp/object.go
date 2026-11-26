package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Order is not decoration. `mcp-sync` registers the declaration's servers in the order the file
// wrote them, so a reader of its output finds them where the declaration put them.

// A Go map answers its keys in a random order, so it cannot be the representation. Values stay raw,
// so a field this package does not model survives for `mcp-sync` to refuse by name.

// Object is a JSON object held in the order its document wrote it, with every value's own bytes kept
// verbatim.
type Object struct {
	keys   []string
	values map[string]json.RawMessage
}

// ParseObject reads one JSON object, keeping its key order. A duplicate key takes the position of its
// first appearance and the value of its last, which is what every JSON reader in this family does
// with one.
func ParseObject(raw []byte) (*Object, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, isDelimiter := opening.(json.Delim); !isDelimiter || delimiter != '{' {
		return nil, fmt.Errorf("expected a JSON object, found %v", opening)
	}
	object := NewObject()
	for decoder.More() {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return nil, tokenErr
		}
		key, isString := token.(string)
		if !isString {
			return nil, fmt.Errorf("expected an object key, found %v", token)
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		object.Set(key, value)
	}
	if _, err = decoder.Token(); err != nil {
		return nil, err
	}
	// An earlier check decoded a second value, and it let `THIS IS NOT JSONC` appended to a declaration
	// through. The file then read as the leading object alone, with every suite over it green.

	// Whatever follows the object is not part of it, so the decoder must be at EOF here. A decode error
	// means something is there and fails to parse, which is the same defect of a document that holds
	// more than one object.
	var trailing json.RawMessage
	switch err = decoder.Decode(&trailing); {
	case errors.Is(err, io.EOF):
		return object, nil
	case err == nil:
		return nil, fmt.Errorf("expected one JSON object, found another value after it")
	default:
		return nil, fmt.Errorf("expected one JSON object, and what follows it is not JSON: %w", err)
	}
}

// NewObject is an empty object. The caller fills it in whatever order it wants the object written.
func NewObject() *Object {
	return &Object{values: map[string]json.RawMessage{}}
}

// Keys in document order.
func (o *Object) Keys() []string {
	return o.keys
}

func (o *Object) Get(key string) (json.RawMessage, bool) {
	value, held := o.values[key]
	return value, held
}

// Set keeps an existing key at its current index, so a replaced value keeps its place.
func (o *Object) Set(key string, value json.RawMessage) {
	if _, held := o.values[key]; !held {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}
