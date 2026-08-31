//go:build unix

package agenthelper

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestStrictRootOwnedFileRejectsUnprivilegedOwner(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test process is root")
	}
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := StrictRootOwnedFile(file); err == nil {
		t.Fatal("unprivileged artifact owner accepted")
	}
}

func TestOpenFileNoFollowDoesNotBlockOnFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.fifo")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	type openResult struct {
		file *os.File
		err  error
	}
	opened := make(chan openResult, 1)
	go func() {
		file, err := openFileNoFollow(path)
		opened <- openResult{file: file, err: err}
	}()
	select {
	case result := <-opened:
		if result.file != nil {
			result.file.Close()
		}
		if result.err != nil {
			t.Fatalf("openFileNoFollow FIFO: %v", result.err)
		}
	case <-time.After(250 * time.Millisecond):
		// Pair a writer with a legacy blocking reader so the regression fails
		// without leaking a blocked goroutine or file descriptor.
		writerDone := make(chan *os.File, 1)
		go func() {
			writer, _ := os.OpenFile(path, os.O_WRONLY, 0)
			writerDone <- writer
		}()
		result := <-opened
		writer := <-writerDone
		if result.file != nil {
			result.file.Close()
		}
		if writer != nil {
			writer.Close()
		}
		t.Fatal("openFileNoFollow blocked on a FIFO")
	}
}

func TestUpdateStageRejectsFIFOWithoutBlockingAndRecovers(t *testing.T) {
	for _, fifoName := range []string{"pulse-agent", "pulse-agent.sig"} {
		t.Run(fifoName, func(t *testing.T) {
			provider, quarantine, _, statePath := testUpdateActivator(t, func([]byte, string) error { return nil })
			artifactID := "fifo-release"
			artifact := testELF("new-signed-binary")
			artifactDir := filepath.Join(quarantine, artifactID)
			if err := os.Mkdir(artifactDir, 0o700); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{
				"pulse-agent":     artifact,
				"pulse-agent.sig": []byte("valid-signature"),
			} {
				path := filepath.Join(artifactDir, name)
				if name == fifoName {
					if err := unix.Mkfifo(path, 0o600); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}

			request := UpdateStageRequest{ArtifactID: artifactID, SHA256: sha256Hex(artifact), Version: "1.1.0"}
			result := make(chan error, 1)
			go func() {
				_, err := provider.Stage(context.Background(), request)
				result <- err
			}()
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("FIFO quarantine artifact was accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("FIFO quarantine artifact blocked the privileged update helper")
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(quarantine), "staging", artifactID)); !os.IsNotExist(err) {
				t.Fatalf("invalid FIFO artifact mutated staging: %v", err)
			}
			if _, err := os.Stat(statePath); !os.IsNotExist(err) {
				t.Fatalf("invalid FIFO artifact mutated durable state: %v", err)
			}

			fifoPath := filepath.Join(artifactDir, fifoName)
			if err := os.Remove(fifoPath); err != nil {
				t.Fatal(err)
			}
			replacement := artifact
			if fifoName == "pulse-agent.sig" {
				replacement = []byte("valid-signature")
			}
			if err := os.WriteFile(fifoPath, replacement, 0o600); err != nil {
				t.Fatal(err)
			}
			promoteUpdate(t, provider, artifactID, request.SHA256)
		})
	}
}
