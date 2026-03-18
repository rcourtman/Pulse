package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestAIConfig_DoesNotExposeLegacyJSONFields(t *testing.T) {
	typeOfConfig := reflect.TypeOf(AIConfig{})
	forbidden := map[string]struct{}{
		"provider":               {},
		"api_key":                {},
		"base_url":               {},
		"autonomous_mode":        {},
		"patrol_schedule_preset": {},
	}

	for i := 0; i < typeOfConfig.NumField(); i++ {
		field := typeOfConfig.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		jsonName := strings.Split(jsonTag, ",")[0]
		if _, blocked := forbidden[jsonName]; blocked {
			t.Fatalf("AIConfig should not include deprecated JSON field %q (Go field: %s)", jsonName, field.Name)
		}
	}
}
