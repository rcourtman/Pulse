package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
)

var errAmbiguousAdminJSON = errors.New("request body must contain one unambiguous JSON object")

// decodeAdminJSONObject preserves the standard decoder's singleton aliases and
// unknown fields, but never chooses between competing values for a control.
// Callers supply small request structs (no embedded fields or custom decoders).
// Validate before unmarshalling so an invalid request cannot partially apply.
func decodeAdminJSONObject(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	if r.Body == nil {
		return errAmbiguousAdminJSON
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' || !json.Valid(data) {
		return errAmbiguousAdminJSON
	}
	if err := validateAdminJSONMembers(data, reflect.TypeOf(dst)); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func validateAdminJSONMembers(data []byte, target reflect.Type) error {
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	data = bytes.TrimSpace(data)
	switch target.Kind() {
	case reflect.Struct, reflect.Map:
		if data[0] != '{' {
			return nil // The ordinary decoder reports incompatible field types.
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		if _, err := decoder.Token(); err != nil {
			return err
		}
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name := key.(string) // json.Valid already checked the object syntax.
			canonical, child := adminJSONField(target, name)
			if child != nil {
				if seen[canonical] {
					// Never echo names or values: map keys can be private group names.
					return errAmbiguousAdminJSON
				}
				seen[canonical] = true
			}
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			if child != nil {
				if err := validateAdminJSONMembers(value, child); err != nil {
					return err
				}
			}
		}
	case reflect.Slice, reflect.Array:
		if data[0] != '[' {
			return nil
		}
		var values []json.RawMessage
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		for _, value := range values {
			if err := validateAdminJSONMembers(value, target.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func adminJSONField(target reflect.Type, name string) (string, reflect.Type) {
	if target.Kind() == reflect.Map {
		// Role/group names are data, not struct-field aliases. Distinct case is
		// meaningful; only exact (including JSON-escaped) duplicates compete.
		return name, target.Elem()
	}
	var foldedName string
	var foldedType reflect.Type
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if !field.IsExported() {
			continue
		}
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if key == "-" {
			continue
		}
		if key == "" {
			key = field.Name
		}
		if key == name {
			return key, field.Type
		}
		if foldedType == nil && strings.EqualFold(key, name) {
			foldedName, foldedType = key, field.Type
		}
	}
	return foldedName, foldedType
}
