package monitoring

import (
	"fmt"
	"strings"
	"testing"
)

func TestPVEBackupWarningUnknownEvidenceAndBoundedEndpoint(t *testing.T) {
	warning := pveBackupAccessWarning(nil, "/nodes/"+strings.Repeat("界", 300)+"\n/storage", fmt.Errorf("403 permission denied; synthetic-secret"))
	if !strings.Contains(warning, "cause is unconfirmed") || strings.Contains(warning, "HTTP 403") || strings.Contains(warning, "synthetic-secret") {
		t.Fatalf("text-only error became HTTP evidence: %q", warning)
	}
	if strings.ContainsAny(warning, "\r\n") || !strings.Contains(warning, "…") || len([]rune(warning)) > 1100 {
		t.Fatalf("endpoint not safely bounded: %q", warning)
	}
}
