package reporting

import (
	"strings"
	"unicode"
)

// spreadsheetText keeps externally supplied text from becoming a spreadsheet
// formula. CSV quoting alone only protects field boundaries, not cell types.
// Prefix the original value (without trimming or rewriting it) with the usual
// spreadsheet text marker. Only text cells use this helper: generated numbers,
// including negative measurements, must remain numbers.
func spreadsheetText(value string) string {
	if value == "" {
		return value
	}
	// Some importers treat these leading controls specially, even before text.
	switch value[0] {
	case '\t', '\r', '\n':
		return "'" + value
	}
	// Importers may ignore whitespace or a byte-order mark before a formula.
	leading := strings.TrimLeftFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\uFEFF'
	})
	if strings.HasPrefix(leading, "=") || strings.HasPrefix(leading, "+") ||
		strings.HasPrefix(leading, "-") || strings.HasPrefix(leading, "@") ||
		strings.HasPrefix(leading, "＝") || strings.HasPrefix(leading, "＋") ||
		strings.HasPrefix(leading, "－") || strings.HasPrefix(leading, "＠") {
		return "'" + value
	}
	return value
}
