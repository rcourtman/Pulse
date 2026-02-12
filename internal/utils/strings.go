package utils

import "strings"

// AddToken inserts a lowercased, trimmed value into a token set.
// Empty or whitespace-only values are ignored.
func AddToken(tokens map[string]struct{}, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	tokens[strings.ToLower(trimmed)] = struct{}{}
}

// LastSegment returns the substring after the last occurrence of sep.
// Returns "" if sep is not found or is at the end of the string.
func LastSegment(value string, sep byte) string {
	if value == "" {
		return ""
	}
	idx := strings.LastIndexByte(value, sep)
	if idx == -1 || idx+1 >= len(value) {
		return ""
	}
	return value[idx+1:]
}

// TrailingDigits returns the trailing numeric suffix of a string.
// Returns "" if the string does not end with digits.
func TrailingDigits(value string) string {
	if value == "" {
		return ""
	}
	i := len(value)
	for i > 0 {
		c := value[i-1]
		if c < '0' || c > '9' {
			break
		}
		i--
	}
	if i == len(value) {
		return ""
	}
	return value[i:]
}
