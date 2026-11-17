//go:build linux

package api

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// A subprocess bounds the regression: reverting to os.ReadFile must fail this
// test promptly, not leave a blocked FIFO reader in the package's process.
func TestOIDCCABundleRevalidationRefusesFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trust.pipe")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOIDCCABundleRevalidationFIFOChild$", "-test.timeout=8s")
	child.Env = append(os.Environ(), "PULSE_TEST_OIDC_CA_FIFO="+path)
	output, err := child.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("CA revalidation did not reject the FIFO within its subprocess deadline: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("FIFO refusal subprocess: %v\n%s", err, output)
	}
}

func TestOIDCCABundleRevalidationFIFOChild(t *testing.T) {
	path := os.Getenv("PULSE_TEST_OIDC_CA_FIFO")
	if path == "" {
		t.Skip("only executed by the bounded FIFO subprocess")
	}
	hash, err := hashCABundle(path)
	if hash != "" || !errors.Is(err, securityutil.ErrUnsafeStorageFile) {
		t.Fatal("FIFO did not return the existing unsafe-file refusal")
	}
}
