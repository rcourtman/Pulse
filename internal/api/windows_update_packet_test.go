package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/updatesignature"
	packet "github.com/rcourtman/pulse-go-rewrite/scripts/installtests/windowsupdatepacket"
	"github.com/stretchr/testify/require"
)

// This producer is deliberately Linux: the server imports Unix-only packages.
// Windows consumes the exact response bytes/headers, not a second implementation
// of filename selection, binary validation, signatures or GitHub fallback.
func TestWindowsSignedUpdatePacket(t *testing.T) {
	destination := os.Getenv("PULSE_WINDOWS_UPDATE_PACKET")
	if destination == "" {
		t.Skip("explicit disposable Windows packet output required")
	}
	source := os.Getenv("EXPECTED_SOURCE_SHA")
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{40}$`), source)
	_, err := os.Stat(destination)
	require.True(t, os.IsNotExist(err), "packet destination must be new")
	require.NoError(t, os.MkdirAll(destination, 0700))
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicText := base64.StdEncoding.EncodeToString(public)
	keyPEM, err := updatesignature.OpenSSHPrivateKeyPEM(private, "disposable-windows-update-fixture")
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "fixture-key")
	require.NoError(t, os.WriteFile(keyFile, []byte(keyPEM), 0600))
	authorizedKey, err := updatesignature.AuthorizedPublicKeyString(private, "")
	require.NoError(t, err)
	allowedSigners := filepath.Join(t.TempDir(), "allowed-signers")
	require.NoError(t, os.WriteFile(allowedSigners, []byte("fixture "+authorizedKey+"\n"), 0600))
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	originalVersion := updates.BuildVersion
	t.Cleanup(func() { updates.BuildVersion = originalVersion })
	result := packet.Packet{Source: source, PublicKey: publicText, Responses: make(map[string]packet.Response)}
	for _, input := range []struct{ name, version, file string }{
		{"initial", packet.Before, "initial.exe"}, {"valid", packet.After, "candidate.exe"},
	} {
		require.True(t, isPublishedReleaseAssetVersion(input.version), "must exercise mandatory published-binary signature admission")
		updates.BuildVersion = input.version
		router, directory := setupUnifiedAgentRouter(t)
		router.serverVersion = input.version
		router.installScriptClient = newTestInstallScriptClient(t, http.MethodGet, "", 0, "", fmt.Errorf("GitHub fallback is forbidden in this fixture"))
		// This fresh directory contains ONLY the canonical .exe and its
		// sidecars. No extensionless compatibility alias can hide #1820.
		t.Setenv("PULSE_BIN_DIR", filepath.Join(directory, "bin"))
		binaryPath := filepath.Join(directory, "bin", "pulse-agent-windows-amd64.exe")
		flags := exec.Command("bash", "scripts/release_ldflags.sh", "agent", "--version", input.version, "--update-public-keys", publicText)
		flags.Dir = root
		ldflags, err := flags.Output()
		require.NoError(t, err)
		build := exec.Command("go", "build", "-buildvcs=false", "-trimpath", "-ldflags="+string(bytes.TrimSpace(ldflags)), "-o", binaryPath, "./cmd/pulse-agent")
		build.Dir = root
		build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
		output, err := build.CombinedOutput()
		require.NoError(t, err, "%s", output)
		body, err := os.ReadFile(binaryPath)
		require.NoError(t, err)
		signature, err := updatesignature.SignBytes(body, private)
		require.NoError(t, err)
		sign := exec.Command("ssh-keygen", "-Y", "sign", "-f", keyFile, "-n", "file", binaryPath)
		output, err = sign.CombinedOutput()
		require.NoError(t, err, "%s", output)
		// ssh-keygen writes .sig; retain that SSHSIG at the production .sshsig
		// path, then restore the distinct Ed25519 detached-signature sidecar.
		sshSignature, err := os.ReadFile(binaryPath + ".sig")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(binaryPath+".sshsig", sshSignature, 0600))
		verify := exec.Command("ssh-keygen", "-Y", "verify", "-f", allowedSigners, "-I", "fixture", "-n", "file", "-s", binaryPath+".sshsig")
		verify.Stdin = bytes.NewReader(body)
		output, err = verify.CombinedOutput()
		require.NoError(t, err, "%s", output)
		require.NoError(t, os.WriteFile(binaryPath+".sig", []byte(signature), 0600))
		capture := func() packet.Response {
			request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent?arch=windows-amd64&serverVersion="+input.version, nil)
			response := httptest.NewRecorder()
			router.handleDownloadUnifiedAgent(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			require.Empty(t, response.Header().Get("X-Served-From"))
			require.Equal(t, body, response.Body.Bytes())
			return packet.Response{File: input.file,
				Checksum: response.Header().Get(checksumHeaderName), Signature: response.Header().Get(signatureHeaderName), SSHSIG: response.Header().Get(sshSignatureHeaderName)}
		}
		result.Responses[input.name] = capture()
		require.NoError(t, os.WriteFile(filepath.Join(destination, input.file), body, 0600))
		if input.name == "valid" {
			// Cryptographically wrong signature with matching executable/checksum:
			// producer presence admission must not stand in for client verification.
			_, otherKey, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			badSignature, err := updatesignature.SignBytes(body, otherKey)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(binaryPath+".sig", []byte(badSignature), 0600))
			result.Responses["bad-signature"] = capture()
		}
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(destination, "packet.json"), encoded, 0600))
	_, _, err = packet.Load(destination, source)
	require.NoError(t, err)
	t.Log("production canonical .exe responses captured; valid/invalid signatures distinguished; GitHub fallback unused")
}
