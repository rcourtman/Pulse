package main

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func normalizeImportPayload(raw []byte) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", fmt.Errorf("configuration payload is empty")
	}

	// 1) If it's base64, keep as-is (this is what ConfigPersistence.ImportConfig expects).
	// 2) If it's base64-of-base64 (common when passing through systems that base64-encode values),
	//    unwrap one layer.
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		decodedTrimmed := strings.TrimSpace(string(decoded))
		if looksLikeBase64(decodedTrimmed) {
			return decodedTrimmed, nil
		}
		return trimmed, nil
	}

	// Otherwise treat it as raw encrypted bytes and base64-encode it.
	return base64.StdEncoding.EncodeToString(raw), nil
}

func looksLikeBase64(s string) bool {
	if s == "" {
		return false
	}
	// Allow whitespace that often appears in wrapped output.
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, s)

	if compact == "" || len(compact)%4 != 0 {
		return false
	}
	for i := 0; i < len(compact); i++ {
		c := compact[i]
		isAlphaNum := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if isAlphaNum || c == '+' || c == '/' || c == '=' {
			continue
		}
		return false
	}
	return true
}
