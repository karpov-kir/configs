package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Order is not decoration. `project-mcp.sh` renders the declaration's server order into a project's
// `.codex/config.toml`, and that file is compared byte for byte against a reinstall's output.
// Re-ordering the servers turns an untouched region into one this tool refuses as edited.

// A Go map answers its keys in a random order, and `encoding/json` marshals them sorted, so neither
// can be the representation. Values stay raw for the same reason a level down. A project's
// `.mcp.json` holds other tools' entries, and round-tripping one through a map sorts its keys and
// re-escapes its strings. The result is a diff across a file this tool was asked to add a server to.

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

// SetValue encodes a Go value and sets it, so a caller building an object hands over a Go value and
// leaves the encoding here. It goes through EncodeJSON, so what it writes is what a config file may hold.
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

// EncodeJSON is `json.Marshal` with HTML escaping off, and it trims the trailing newline.
//
// The default escaping guards a browser. It writes the `&&` in the launcher command as a pair of
// `\u0026` escapes, one string to a parser and a different file to `cmp`. These configs are compared
// byte for byte against a reinstall's output, so escaping no caller asked for reads as an edit.
func EncodeJSON(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(out.String(), "\n")), nil
}
