package tools

import "strings"

// resolveContainerArg returns the canonical container argument value.
// The returned bool is true when deprecated app_container was provided.
func resolveContainerArg(args map[string]interface{}) (string, bool) {
	if value, ok := args["container"].(string); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value, false
		}
	}
	if value, ok := args["app_container"].(string); ok {
		if strings.TrimSpace(value) != "" {
			return "", true
		}
	}
	return "", false
}

// setContainerResponseFields includes canonical response fields.
func setContainerResponseFields(response map[string]interface{}, container string) {
	if container == "" {
		return
	}
	response["container"] = container
}
