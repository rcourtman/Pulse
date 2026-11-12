package agenttarget

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestObserverPrivateFileReadsCompleteBoundedContents(t *testing.T) {
	for _, limit := range []int64{maxConfigBytes, maxTokenBytes} {
		for _, size := range []int64{0, limit - 1, limit, limit + 1} {
			t.Run(fmt.Sprintf("limit-%d/size-%d", limit, size), func(t *testing.T) {
				reader := &observerCountingReader{Reader: strings.NewReader(strings.Repeat("x", int(size)))}
				// Model a stream whose stat size understates its served bytes. The
				// complete read must be bounded independently of descriptor size.
				data, err := readPrivateFileContents(reader, "observer token", limit)
				if size > limit {
					if err == nil || data != nil || !strings.Contains(err.Error(), "size limit") {
						t.Fatal("oversized contents were accepted or a prefix escaped")
					}
				} else if err != nil || int64(len(data)) != size {
					t.Fatalf("complete file read failed: size=%d error=%v", len(data), err)
				}
				if reader.bytes > limit+1 {
					t.Fatalf("read %d bytes beyond %d-byte boundary", reader.bytes, limit+1)
				}
			})
		}
	}
	t.Run("far-over-limit", func(t *testing.T) {
		reader := &observerCountingReader{Reader: strings.NewReader(strings.Repeat("x", maxTokenBytes*3))}
		if data, err := readPrivateFileContents(reader, "observer token", maxTokenBytes); err == nil || data != nil || reader.bytes != maxTokenBytes+1 {
			t.Fatalf("unbounded or partial credential read: bytes=%d error=%v", reader.bytes, err)
		}
	})
	t.Run("late-read-error", func(t *testing.T) {
		failure := errors.New("synthetic read failure")
		reader := io.MultiReader(strings.NewReader("synthetic-prefix"), observerErrorReader{failure})
		if data, err := readPrivateFileContents(reader, "observer token", maxTokenBytes); !errors.Is(err, failure) || data != nil {
			t.Fatal("partial authority escaped after a read failure")
		}
	})
}

type observerCountingReader struct {
	io.Reader
	bytes int64
}

func (r *observerCountingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytes += int64(n)
	return n, err
}

type observerErrorReader struct{ err error }

func (r observerErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestObserverPrivateFileRechecksOpenedHandle(t *testing.T) {
	for _, change := range []string{"grown", "permissions", "closed"} {
		t.Run(change, func(t *testing.T) {
			if change == "permissions" && runtime.GOOS == "windows" {
				t.Skip("Unix permission policy is not used on Windows")
			}
			path := filepath.Join(t.TempDir(), "observer.token")
			writePrivateFile(t, path, "synthetic-token")
			var opened *os.File
			calls := 0
			data, err := readPrivateRegularFileWithOpener(path, "observer token", maxTokenBytes, func(path string) (*os.File, error) {
				calls++
				switch change {
				case "grown":
					if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxTokenBytes+1), 0600); err != nil {
						t.Fatal(err)
					}
				case "permissions":
					if err := os.Chmod(path, 0644); err != nil {
						t.Fatal(err)
					}
				}
				file, openErr := openPrivateFile(path)
				opened = file
				if change == "closed" && openErr == nil {
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
				}
				return file, openErr
			})
			if calls != 1 || err == nil || data != nil {
				t.Fatalf("changed file became authority: opens=%d error=%v", calls, err)
			}
			if opened != nil {
				if _, err := opened.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("refused file descriptor was not closed")
				}
			}
		})
	}
}

func TestObserverPrivateFileRejectsKnownInvalidPathBeforeOpen(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "oversized", "permissions"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "permissions" && runtime.GOOS == "windows" {
				t.Skip("Unix permission policy is not used on Windows")
			}
			path := filepath.Join(t.TempDir(), "observer.token")
			switch kind {
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				writePrivateFile(t, path, strings.Repeat("x", maxTokenBytes+1))
			case "permissions":
				writePrivateFile(t, path, "synthetic-token")
				if err := os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
			}
			data, err := readPrivateRegularFileWithOpener(path, "observer token", maxTokenBytes, func(string) (*os.File, error) {
				t.Fatal("known invalid path was opened")
				return nil, os.ErrInvalid
			})
			if err == nil || data != nil {
				t.Fatal("invalid private file was admitted")
			}
		})
	}
}

func TestLoadObserversUsesCompleteBoundedFiles(t *testing.T) {
	dir := t.TempDir()
	configPath, tokenPath := filepath.Join(dir, "observers.json"), filepath.Join(dir, "observer.token")
	encodedPath, err := json.Marshal(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	config := `{"version":1,"observers":[{"name":"isolated","url":"https://observer.example.test/base","tokenFile":` + string(encodedPath) + `,"provisionProxmox":false}]}`
	for _, tc := range []struct {
		name, config, token string
		wantError           bool
	}{
		{"small", config, "synthetic-token\n", false},
		{"exact-token-bound", config, strings.Repeat("t", maxTokenBytes), false},
		{"oversized-token", config, strings.Repeat("t", maxTokenBytes+1), true},
		{"exact-config-bound", config + strings.Repeat(" ", maxConfigBytes-len(config)), "synthetic-token", false},
		{"oversized-config", config + strings.Repeat(" ", maxConfigBytes-len(config)+1), "synthetic-token", true},
		{"second-object", config + `{}`, "synthetic-token", true},
		{"trailing-invalid", config + `!`, "synthetic-token", true},
		{"truncated-config", strings.TrimSuffix(config, "}"), "synthetic-token", true},
		{"unknown-inline-token", strings.Replace(config, `"provisionProxmox":false`, `"token":"not-allowed"`, 1), "synthetic-token", true},
		{"empty-token", config, " \n\t", true},
		{"internal-token-whitespace", config, "synthetic token", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writePrivateFile(t, configPath, tc.config)
			writePrivateFile(t, tokenPath, tc.token)
			beforeConfig, _ := os.Stat(configPath)
			beforeToken, _ := os.Stat(tokenPath)
			got, err := LoadObservers(configPath, "https://primary.example.test")
			if (err != nil) != tc.wantError {
				t.Fatalf("load error=%v, want rejection=%t", err, tc.wantError)
			}
			if tc.wantError {
				if got != nil {
					t.Fatal("failed load released partial destinations")
				}
			} else if len(got) != 1 || got[0].Token != strings.TrimSpace(tc.token) || got[0].URL != "https://observer.example.test/base" || got[0].ProvisionProxmox {
				t.Fatal("valid observer choice or complete token changed")
			}
			for path, expected := range map[string]struct {
				content string
				info    os.FileInfo
			}{configPath: {tc.config, beforeConfig}, tokenPath: {tc.token, beforeToken}} {
				content, readErr := os.ReadFile(path)
				info, statErr := os.Stat(path)
				if readErr != nil || statErr != nil || string(content) != expected.content || !os.SameFile(info, expected.info) || info.Mode() != expected.info.Mode() || !info.ModTime().Equal(expected.info.ModTime()) {
					t.Fatal("loading/refusal changed saved observer files")
				}
			}
		})
	}
}
