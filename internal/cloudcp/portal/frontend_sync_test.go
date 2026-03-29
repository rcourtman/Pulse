package portal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type portalFrontendManifest struct {
	SourceHash  string   `json:"source_hash"`
	BuildInputs []string `json:"build_inputs"`
}

func TestPulseAccountFrontendBundleStaysInSync(t *testing.T) {
	manifestPath := filepath.Join("dist", "build_manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read portal frontend manifest: %v", err)
	}

	var manifest portalFrontendManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode portal frontend manifest: %v", err)
	}
	if manifest.SourceHash == "" {
		t.Fatal("portal frontend manifest missing source_hash")
	}
	if len(manifest.BuildInputs) == 0 {
		t.Fatal("portal frontend manifest missing build_inputs")
	}

	requiredInputs := map[string]struct{}{
		"build_config.mjs": {},
		"src/styles.css":   {},
	}

	hash := sha256.New()
	for _, relativePath := range manifest.BuildInputs {
		delete(requiredInputs, relativePath)
		hash.Write([]byte(relativePath))
		hash.Write([]byte("\n"))
		content, err := os.ReadFile(filepath.Join("frontend", relativePath))
		if err != nil {
			t.Fatalf("read portal frontend source %s: %v", relativePath, err)
		}
		hash.Write(content)
		hash.Write([]byte("\n"))
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != manifest.SourceHash {
		t.Fatalf(
			"portal frontend build drift detected; run `npm --prefix internal/cloudcp/portal/frontend run build`\nmanifest=%s\nactual=%s",
			manifest.SourceHash,
			actualHash,
		)
	}
	if len(requiredInputs) > 0 {
		t.Fatalf("portal frontend manifest missing canonical inputs: %v", mapsKeys(requiredInputs))
	}
}

func mapsKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
