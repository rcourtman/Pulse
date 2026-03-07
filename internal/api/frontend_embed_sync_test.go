package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestEmbeddedFrontendAssetsStayInSync(t *testing.T) {
	rootDist := filepath.Join("..", "..", "frontend-modern", "dist")
	embedDist := filepath.Join("frontend-modern", "dist")

	if _, err := os.Stat(rootDist); os.IsNotExist(err) {
		t.Skip("frontend-modern/dist is not present")
	}

	rootFiles := hashFrontendFiles(t, rootDist)
	embedFiles := hashFrontendFiles(t, embedDist)

	if len(rootFiles) != len(embedFiles) {
		t.Fatalf("frontend embed drift: root has %d files, embed has %d files", len(rootFiles), len(embedFiles))
	}

	missing := make([]string, 0)
	mismatched := make([]string, 0)
	unexpected := make([]string, 0)

	for rel, sum := range rootFiles {
		embedSum, ok := embedFiles[rel]
		if !ok {
			missing = append(missing, rel)
			continue
		}
		if embedSum != sum {
			mismatched = append(mismatched, rel)
		}
	}

	for rel := range embedFiles {
		if _, ok := rootFiles[rel]; !ok {
			unexpected = append(unexpected, rel)
		}
	}

	sort.Strings(missing)
	sort.Strings(mismatched)
	sort.Strings(unexpected)

	if len(missing) > 0 || len(mismatched) > 0 || len(unexpected) > 0 {
		t.Fatalf(
			"frontend embed drift detected; run `npm --prefix frontend-modern run build`\nmissing=%v\nmismatched=%v\nunexpected=%v",
			missing,
			mismatched,
			unexpected,
		)
	}
}

func hashFrontendFiles(t *testing.T, root string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}

		files[filepath.ToSlash(rel)] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	if err != nil {
		t.Fatalf("hash frontend files from %s: %v", root, err)
	}

	return files
}
