package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var errAlertConfigUpdateNotObject = errors.New("alert configuration update must be a JSON object")

// ApplyAlertConfigUpdate decodes a client's alert configuration update onto
// the stored configuration. A top-level key the client sent replaces that
// setting whole; a key it left out keeps the stored value.
//
// Clients only send what they know about. The settings page has no control
// for flapping detection, alert TTL cleanup or custom rules, and an older
// browser bundle or API script predates any field added since. Decoding the
// body into a zero AlertConfig used to turn every one of those settings off
// on each save.
//
// The body is decoded on its own, so the keys it carries mean exactly what
// encoding/json makes of them. Each setting it left out is copied from a JSON
// round trip of stored, so the result shares no maps or slices with it.
func ApplyAlertConfigUpdate(stored AlertConfig, update []byte) (AlertConfig, error) {
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(update, &sent); err != nil {
		return AlertConfig{}, err
	}
	if sent == nil {
		return AlertConfig{}, errAlertConfigUpdateNotObject
	}
	var result AlertConfig
	if err := json.Unmarshal(update, &result); err != nil {
		return AlertConfig{}, err
	}

	kept, err := CloneAlertConfig(stored)
	if err != nil {
		return AlertConfig{}, err
	}

	resultValue := reflect.ValueOf(&result).Elem()
	keptValue := reflect.ValueOf(kept)
	for i := 0; i < resultValue.NumField(); i++ {
		if !alertConfigKeySent(sent, alertConfigJSONKey(resultValue.Type().Field(i))) {
			resultValue.Field(i).Set(keptValue.Field(i))
		}
	}
	return result, nil
}

// CloneAlertConfig returns a copy of config that shares no maps, slices or
// pointers with it, by the same JSON round trip config takes through
// alerts.json.
func CloneAlertConfig(config AlertConfig) (AlertConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return AlertConfig{}, fmt.Errorf("encode alert config: %w", err)
	}
	var clone AlertConfig
	if err := json.Unmarshal(data, &clone); err != nil {
		return AlertConfig{}, fmt.Errorf("decode alert config: %w", err)
	}
	return clone, nil
}

// alertConfigJSONKey returns the key encoding/json reads a field from.
func alertConfigJSONKey(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		return field.Name
	}
	return name
}

// alertConfigKeySent matches keys the way encoding/json matches them to
// fields: exactly, or else case-insensitively.
func alertConfigKeySent(sent map[string]json.RawMessage, key string) bool {
	if _, ok := sent[key]; ok {
		return true
	}
	for sentKey := range sent {
		if strings.EqualFold(sentKey, key) {
			return true
		}
	}
	return false
}
