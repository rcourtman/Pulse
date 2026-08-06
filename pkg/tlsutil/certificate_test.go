package tlsutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestObservePeerCertificateClassifiesSelfSignedAndValidity(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	leaf := issueTestCertificate(t, pkix.Name{CommonName: "pulse.local"}, []string{"pulse.local"}, now.Add(-time.Hour), now.Add(30*24*time.Hour))
	state := &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}

	observation := ObservePeerCertificate(state, "pulse.local", now)
	if observation == nil {
		t.Fatal("expected certificate observation")
	}
	if observation.TrustStatus != CertificateTrustSelfSigned || !observation.SelfSigned {
		t.Fatalf("trust = %q selfSigned=%v, want self-signed", observation.TrustStatus, observation.SelfSigned)
	}
	if !observation.HostnameValid {
		t.Fatalf("hostname should validate: %s", observation.TrustError)
	}
	if observation.NotAfter != leaf.NotAfter.UTC() || len(observation.FingerprintSHA256) != 64 {
		t.Fatalf("unexpected certificate projection: %+v", observation)
	}

	expired := issueTestCertificate(t, pkix.Name{CommonName: "pulse.local"}, []string{"pulse.local"}, now.Add(-48*time.Hour), now.Add(-time.Hour))
	expiredObservation := ObservePeerCertificate(&tls.ConnectionState{PeerCertificates: []*x509.Certificate{expired}}, "pulse.local", now)
	if expiredObservation == nil || expiredObservation.TrustStatus != CertificateTrustExpired {
		t.Fatalf("expired trust = %+v, want expired", expiredObservation)
	}
}

func TestObservePeerCertificateReportsHostnameMismatch(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	leaf := issueTestCertificate(t, pkix.Name{CommonName: "pulse.local"}, []string{"pulse.local"}, now.Add(-time.Hour), now.Add(30*24*time.Hour))

	observation := ObservePeerCertificate(&tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}, "other.local", now)
	if observation == nil || observation.HostnameValid {
		t.Fatalf("hostname result = %+v, want mismatch", observation)
	}
	if observation.TrustError == "" {
		t.Fatal("expected bounded hostname error")
	}
}

func TestCertificateObservationCloneOwnsDNSNames(t *testing.T) {
	original := &CertificateObservation{DNSNames: []string{"pulse.local"}}
	cloned := original.Clone()
	cloned.DNSNames[0] = "changed.local"
	if original.DNSNames[0] != "pulse.local" {
		t.Fatal("clone mutated original DNS names")
	}
}

func issueTestCertificate(t *testing.T, subject pkix.Name, dnsNames []string, notBefore, notAfter time.Time) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               subject,
		Issuer:                subject,
		DNSNames:              dnsNames,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}
