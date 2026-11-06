package windowsupdatepacket

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExactProductionResponses(t *testing.T) {
	for _, control := range []string{"valid", "source", "traversal", "checksum", "changed-body", "missing", "extra", "signature", "negative-signature", "public-key", "sshsig", "trailing", "symlink"} {
		t.Run(control, func(t *testing.T) {
			directory := t.TempDir()
			source := strings.Repeat("1", 40)
			public, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			body := make([]byte, 1024*1024)
			signature := base64.StdEncoding.EncodeToString(ed25519.Sign(private, body))
			_, wrong, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			badSignature := base64.StdEncoding.EncodeToString(ed25519.Sign(wrong, body))
			packet := Packet{Source: source, PublicKey: base64.StdEncoding.EncodeToString(public), Responses: map[string]Response{
				"initial":       {File: "initial.exe", Checksum: Digest(body), Signature: signature, SSHSIG: "captured-header"},
				"valid":         {File: "candidate.exe", Checksum: Digest(body), Signature: signature, SSHSIG: "captured-header"},
				"bad-signature": {File: "candidate.exe", Checksum: Digest(body), Signature: badSignature, SSHSIG: "captured-header"},
			}}
			initial := packet.Responses["initial"]
			switch control {
			case "source":
				packet.Source = strings.Repeat("2", 40)
			case "traversal":
				initial.File = "../initial.exe"
			case "checksum":
				initial.Checksum = strings.Repeat("0", 64)
			case "signature":
				initial.Signature = badSignature
			case "public-key":
				packet.PublicKey = "invalid"
			case "sshsig":
				initial.SSHSIG = ""
			}
			packet.Responses["initial"] = initial
			switch control {
			case "missing":
				delete(packet.Responses, "valid")
			case "extra":
				packet.Responses["other"] = initial
			case "negative-signature":
				r := packet.Responses["bad-signature"]
				r.Signature = signature
				packet.Responses["bad-signature"] = r
			}
			for _, name := range []string{"initial.exe", "candidate.exe"} {
				if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if control == "changed-body" {
				body[0] = 1
				if err := os.WriteFile(filepath.Join(directory, "initial.exe"), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if control == "symlink" {
				if err := os.Remove(filepath.Join(directory, "initial.exe")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("candidate.exe", filepath.Join(directory, "initial.exe")); err != nil {
					t.Skip("symlink creation unavailable on this runtime")
				}
			}
			encoded, err := json.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			if control == "trailing" {
				encoded = append(encoded, []byte("{}")...)
			}
			if err := os.WriteFile(filepath.Join(directory, "packet.json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			_, _, err = Load(directory, source)
			if (err == nil) != (control == "valid") {
				t.Fatalf("control %s admission error=%v", control, err)
			}
		})
	}
}
