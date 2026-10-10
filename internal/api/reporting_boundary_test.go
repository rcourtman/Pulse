package api

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the production Go classifiers, not a second copy of their patterns.
// The shell classifier is exercised in scripts/repo-boundary-regression.py,
// where the boundary workflow supplies its declared ripgrep dependency.
// An exact shared-pipeline exemption must not admit neighbouring implementation
// files or let the exempt helper import private licensing code.
func TestReportingSubjectBoundaryExemptionIsExact(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join("..", "..")
	for _, tc := range []struct {
		name, file, content, goMarker string
	}{
		{name: "shared subject pipeline"},
		{
			name: "new reporting implementation", file: "reporting_subject_alerts_extra.go", content: "package api\n",
			goMarker: "untracked paid-domain file reporting_subject_alerts_extra.go",
		},
		{
			name: "exact path not prefix", file: "reporting_subject_alerts.go.extra.go", content: "package api\n",
			goMarker: "untracked paid-domain file reporting_subject_alerts.go.extra.go",
		},
		{
			name: "private root import still denied", file: "reporting_subject_alerts.go",
			content:  "package api\nimport _ \"github.com/rcourtman/pulse-go-rewrite/internal/license\"\n",
			goMarker: "direct internal/license import",
		},
		{
			name: "licensing bridge still required", file: "reporting_subject_alerts.go",
			content:  "package api\nimport _ \"github.com/rcourtman/pulse-go-rewrite/pkg/licensing\"\n",
			goMarker: "reporting_subject_alerts.go",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"internal/api"} {
				err := filepath.WalkDir(filepath.Join(repo, dir), func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
						return nil
					}
					rel, err := filepath.Rel(repo, path)
					if err != nil {
						return err
					}
					copyReportingBoundaryFile(t, path, filepath.Join(root, rel))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			apiDir := filepath.Join(root, "internal/api")
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(apiDir, tc.file), []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// The running test binary scans fixture bytes; injected files are
			// never compiled or executed. The anchored selector prevents recursion.
			runReportingBoundaryCommand(t, apiDir, tc.goMarker, binary,
				"-test.run=^(TestPaidDomainBoundaryAudit|TestNoInternalLicenseImportFromAPIProd|TestPkgLicensingImportBoundary)$", "-test.v")
		})
	}
}

func copyReportingBoundaryFile(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func runReportingBoundaryCommand(t *testing.T, dir, rejection, command string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if rejection == "" {
		if err != nil {
			t.Fatalf("shared pipeline rejected by %s: %v\n%s", command, err, output)
		}
		return
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || !strings.Contains(string(output), rejection) {
		t.Fatalf("%s rejection = %v / %s, want exit 1 containing %q", command, err, output, rejection)
	}
}
