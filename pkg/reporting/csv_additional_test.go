package reporting

import "testing"

func TestFormatValue(t *testing.T) {
	if got := formatValue(1024, "bytes"); got != "1.00 KiB" {
		t.Fatalf("expected 1.00 KiB, got %q", got)
	}
	if got := formatValue(1.234, "%"); got != "1.23" {
		t.Fatalf("expected 1.23, got %q", got)
	}
}
