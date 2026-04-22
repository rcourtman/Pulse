package agenttls

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

const testPEMCertificate = `-----BEGIN CERTIFICATE-----
MIICuDCCAaACCQDptFpSdDdFNjANBgkqhkiG9w0BAQsFADAeMRwwGgYDVQQDDBNw
dWxzZS10ZXN0LWNhLmxvY2FsMB4XDTI2MDMxNDA4NTgyN1oXDTI3MDMxNDA4NTgy
N1owHjEcMBoGA1UEAwwTcHVsc2UtdGVzdC1jYS5sb2NhbDCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBANWmj5xXF1pDWKqbScN6VtU1PX3e9DuyDnegnAuR
UA7QIqgyQ7gfPZtAABr0kaV993mZZw92XkdXeF+9eClRBnVoJmISdwiBpB6oE8w/
H6tfnG34JUjvXN39/B66mAeuBd/erAxj4fXuH+ohA3AWZcotCYS2anOAbyRPo8BU
DGm79VBp5/s/uZ8bGe5LiSPxFXOp7kBk2sDWI77Y0UNwuc/wzO+GrE0GGXnbxcRW
9ICRPq7pked0BO2oBaeMRmvo7npAn9+w+0EDVi1qqw5xoYposYgsR76uLSYhQgaL
5ZgUYlCW7Vvp5ve/tmxPXuae8y3OIrOT7WFWfm8GAa9ZneMCAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAdpFuEiVPhYcJe/kkfPuHwv68Dx+/5jFXMkLQFIZnnC5Umkph
ubtFPrce9BLqLQBGdhQ4IkaEA9QDSZDTUbzZLtw3G6tHgl63H4kuB5ZbXgEVPmNT
07i8Obt4uUgIhfx/EzyCaZpfoQnXHmHm2xxg6QiP4v2TUQdBkLpD5mzVTwYOw9GF
w8AuCKd92UTs4/0ikTMdK0M4zwhF0JAhibyMNBRXfg1c96KyCFYSSNeERQFy5Fqo
TREsx8ScXgne7V+lLwLa8CTjUAcvCVq6SIqKbjSEZ1V5UpzvwBh52/cWCa6Rafd5
ARKc3gwyVxyCX3h21kFcEU2rt7C7/RcXBCyWzQ==
-----END CERTIFICATE-----
`

func TestNewClientTLSConfig_Defaults(t *testing.T) {
	cfg, err := NewClientTLSConfig("", false, "")
	if err != nil {
		t.Fatalf("NewClientTLSConfig: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %#v, want TLS1.2", cfg.MinVersion)
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify=false by default")
	}
	if cfg.RootCAs != nil {
		t.Fatal("expected RootCAs to remain nil without a custom bundle")
	}
}

func TestNewClientTLSConfig_LoadsCustomCABundle(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "pulse-ca.pem")
	if err := os.WriteFile(certPath, []byte(testPEMCertificate), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	cfg, err := NewClientTLSConfig(certPath, false, "")
	if err != nil {
		t.Fatalf("NewClientTLSConfig: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be populated for a custom CA bundle")
	}
}

func TestNewClientTLSConfig_RejectsInvalidBundle(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(certPath, []byte("not-a-cert"), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	if _, err := NewClientTLSConfig(certPath, false, ""); err == nil {
		t.Fatal("expected invalid CA bundle to fail")
	}
}

func TestNewClientTLSConfig_UsesPinnedFingerprint(t *testing.T) {
	block, _ := pem.Decode([]byte(testPEMCertificate))
	if block == nil {
		t.Fatal("failed to decode test certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}

	sum := sha256.Sum256(cert.Raw)
	cfg, err := NewClientTLSConfig("", false, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("NewClientTLSConfig: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("expected pinned fingerprint mode to bypass CA verification in favor of explicit pinning")
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("expected VerifyPeerCertificate to be configured for fingerprint pinning")
	}
	if err := cfg.VerifyPeerCertificate([][]byte{cert.Raw}, nil); err != nil {
		t.Fatalf("VerifyPeerCertificate: %v", err)
	}
	if err := cfg.VerifyPeerCertificate([][]byte{[]byte("mismatch")}, nil); err == nil {
		t.Fatal("expected mismatched certificate fingerprint to be rejected")
	}
}
