package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Object is a JSON object held in the order its document wrote it, with every value's own bytes kept
// verbatim.
//
// Order is not decoration here. The declaration's server order is what `project-mcp.sh` renders into
// a project's `.codex/config.toml`, and that file is compared byte for byte against what a reinstall
// would write: re-ordering the servers turns an untouched region into one this tool refuses as
// edited. A Go map answers its keys in a random order and `encoding/json` marshals them sorted, so
// neither can be the representation.
//
// Values stay raw for the same reason a level down. A project's `.mcp.json` holds other tools'
// entries, and round-tripping one through a map would sort its keys and re-escape its strings — a
// diff across a file this tool was asked to add one server to.
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
	// Whatever follows the object is not part of it. The test is that the decoder is at EOF, NOT that a
	// second value decodes: a decode error means something is there and does not parse, which is the
	// same document-is-not-one-object defect wearing a different hat. Read the other way round,
	// `THIS IS NOT JSONC` appended to a declaration was accepted and the file read as the object above
	// it, with every suite over it green.
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

// NewObject is an empty object, filled in whatever order the caller wants it written.
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

// Set keeps an existing key where it already sits, so replacing a value never moves it.
func (o *Object) Set(key string, value json.RawMessage) {
	if _, held := o.values[key]; !held {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// SetValue encodes a Go value and sets it, so a caller building an object states values rather than
// bytes. It goes through EncodeJSON, so what it writes is what a config file may hold.
func (o *Object) SetValue(key string, value any) error {
	encoded, err := EncodeJSON(value)
	if err != nil {
		return err
	}
	o.Set(key, encoded)
	return nil
}

func (o *Object) Delete(key string) {
	if _, held := o.values[key]; !held {
		return
	}
	delete(o.values, key)
	for index, name := range o.keys {
		if name == key {
			o.keys = append(o.keys[:index], o.keys[index+1:]...)
			return
		}
	}
}

// MarshalJSON writes the object compactly, in key order. Callers wanting the indented form hand the
// result to `json.Indent`, whose output matches what the Node version of this tool wrote.
func (o *Object) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for index, key := range o.keys {
		if index > 0 {
			out.WriteByte(',')
		}
		name, err := EncodeJSON(key)
		if err != nil {
			return nil, err
		}
		out.Write(name)
		out.WriteByte(':')
		if err = json.Compact(&out, o.values[key]); err != nil {
			return nil, err
		}
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// EncodeJSON is `json.Marshal` with HTML escaping off and no trailing newline.
//
// The default escaping is what a browser needs, not what a config file does: it writes the `&&` in
// the launcher command as a pair of `\u0026` escapes, which is the same string to a parser and a
// different file to `cmp`. These configs are compared byte for byte against what a reinstall would
// write, so an escaping nobody asked for reads as a file somebody edited.
func EncodeJSON(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(out.String(), "\n")), nil
}
