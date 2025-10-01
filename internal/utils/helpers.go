package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Float64Ptr returns a pointer to the given float64 value
func Float64Ptr(v float64) *float64 {
	return &v
}

// GenerateID generates a unique ID with the given prefix
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// WriteJSONResponse writes a JSON response to the http.ResponseWriter
func WriteJSONResponse(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	// Use Marshal instead of Encoder for better performance with large payloads
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.Write(jsonData)
	return err
}

// WriteJSONError writes a JSON error response to the http.ResponseWriter
func WriteJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// DecodeJSONBody decodes a JSON request body into the given interface
func DecodeJSONBody(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	return nil
}
