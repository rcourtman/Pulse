package tools

import "strings"

// resolveContainerArg returns the preferred container argument value.
// "container" is canonical; "app_container" is a legacy compatibility alias.
func resolveContainerArg(args map[string]interface{}) string {
	if value, ok := args["container"].(string); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	if value, ok := args["app_container"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

// setContainerResponseFields includes canonical response fields.
func setContainerResponseFields(response map[string]interface{}, container string) {
	if container == "" {
		return
	}
	response["container"] = container
}
