package modelpolicy

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

func checkValue(decoder *json.Decoder, shape reflect.Type, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON nesting exceeds 64 levels")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("null is not a model policy value")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("expected object key")
			}
			if keys[key] {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			keys[key] = true
			child, err := fieldShape(shape, key)
			if err != nil {
				return err
			}
			if err := checkValue(decoder, child, depth+1); err != nil {
				return err
			}
		}
	case '[':
		if shape.Kind() != reflect.Slice {
			return fmt.Errorf("unexpected JSON array")
		}
		for decoder.More() {
			if err := checkValue(decoder, shape.Elem(), depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	_, err = decoder.Token()
	if err != nil {
		return err
	}
	if depth == 0 {
		if _, err = decoder.Token(); err != io.EOF {
			if err == nil {
				return fmt.Errorf("trailing JSON value")
			}
			return err
		}
	}
	return nil
}

func fieldShape(shape reflect.Type, key string) (reflect.Type, error) {
	if shape.Kind() == reflect.Pointer {
		shape = shape.Elem()
	}
	if shape.Kind() == reflect.Map {
		return shape.Elem(), nil
	}
	if shape.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unexpected JSON object")
	}
	for i := 0; i < shape.NumField(); i++ {
		field := shape.Field(i)
		if strings.Split(field.Tag.Get("json"), ",")[0] == key {
			return field.Type, nil
		}
	}
	return nil, fmt.Errorf("unknown JSON field %q", key)
}
