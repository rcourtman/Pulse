package tlsutil

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"time"
)

// CertificateTrustStatus is the bounded trust result published for an HTTPS
// availability observation. It separates an intentionally self-signed leaf
// from an otherwise untrusted chain so operators can distinguish the two.
type CertificateTrustStatus string

const (
	CertificateTrustTrusted     CertificateTrustStatus = "trusted"
	CertificateTrustSelfSigned  CertificateTrustStatus = "self-signed"
	CertificateTrustUntrusted   CertificateTrustStatus = "untrusted"
	CertificateTrustExpired     CertificateTrustStatus = "expired"
	CertificateTrustNotYetValid CertificateTrustStatus = "not-yet-valid"
)

// CertificateObservation is the canonical, secret-free TLS leaf projection
// shared by local availability checks, remote probe-agent reports and unified
// resource consumers.
type CertificateObservation struct {
	Subject           string                 `json:"subject,omitempty"`
	Issuer            string                 `json:"issuer,omitempty"`
	SerialNumber      string                 `json:"serialNumber,omitempty"`
	DNSNames          []string               `json:"dnsNames,omitempty"`
	ObservedAt        time.Time              `json:"observedAt"`
	NotBefore         time.Time              `json:"notBefore"`
	NotAfter          time.Time              `json:"notAfter"`
	FingerprintSHA256 string                 `json:"fingerprintSha256"`
	TrustStatus       CertificateTrustStatus `json:"trustStatus"`
	ChainValid        bool                   `json:"chainValid"`
	HostnameValid     bool                   `json:"hostnameValid"`
	SelfSigned        bool                   `json:"selfSigned"`
	TrustError        string                 `json:"trustError,omitempty"`
}

// Clone returns an independent observation for registry and report snapshots.
func (o *CertificateObservation) Clone() *CertificateObservation {
	if o == nil {
		return nil
	}
	out := *o
	out.DNSNames = append([]string(nil), o.DNSNames...)
	return &out
}

// ObservePeerCertificate derives the leaf identity, validity and trust posture
// from a completed TLS connection. The caller may use an intentionally
// unverified transport for reachability, while this function performs the
// explicit trust and hostname checks reported to the operator.
func ObservePeerCertificate(state *tls.ConnectionState, serverName string, now time.Time) *CertificateObservation {
	if state == nil || len(state.PeerCertificates) == 0 {
		return nil
	}
	leaf := state.PeerCertificates[0]
	if leaf == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	serverName = strings.TrimSpace(serverName)

	fingerprint := sha256.Sum256(leaf.Raw)
	observation := &CertificateObservation{
		Subject:           certificateName(leaf.Subject.CommonName, leaf.Subject.String()),
		Issuer:            certificateName(leaf.Issuer.CommonName, leaf.Issuer.String()),
		SerialNumber:      strings.ToUpper(leaf.SerialNumber.Text(16)),
		DNSNames:          append([]string(nil), leaf.DNSNames...),
		ObservedAt:        now,
		NotBefore:         leaf.NotBefore.UTC(),
		NotAfter:          leaf.NotAfter.UTC(),
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
		SelfSigned:        certificateIsSelfSigned(leaf),
	}

	hostnameErr := verifyCertificateHostname(leaf, serverName)
	observation.HostnameValid = hostnameErr == nil
	chainErr := verifyCertificateChain(state.PeerCertificates, now)
	observation.ChainValid = chainErr == nil

	switch {
	case now.Before(leaf.NotBefore):
		observation.TrustStatus = CertificateTrustNotYetValid
	case now.After(leaf.NotAfter):
		observation.TrustStatus = CertificateTrustExpired
	case observation.SelfSigned:
		observation.TrustStatus = CertificateTrustSelfSigned
	case observation.ChainValid && observation.HostnameValid:
		observation.TrustStatus = CertificateTrustTrusted
	default:
		observation.TrustStatus = CertificateTrustUntrusted
	}
	observation.TrustError = certificateTrustError(chainErr, hostnameErr, observation.SelfSigned)
	return observation
}

func certificateName(commonName, distinguishedName string) string {
	if value := strings.TrimSpace(commonName); value != "" {
		return value
	}
	return strings.TrimSpace(distinguishedName)
}

func certificateIsSelfSigned(certificate *x509.Certificate) bool {
	if certificate == nil || certificate.RawSubject == nil || certificate.RawIssuer == nil {
		return false
	}
	if !strings.EqualFold(hex.EncodeToString(certificate.RawSubject), hex.EncodeToString(certificate.RawIssuer)) {
		return false
	}
	return certificate.CheckSignatureFrom(certificate) == nil
}

func verifyCertificateHostname(certificate *x509.Certificate, serverName string) error {
	if certificate == nil {
		return errors.New("certificate is missing")
	}
	if serverName == "" {
		return errors.New("target hostname is missing")
	}
	if ip := net.ParseIP(serverName); ip != nil {
		return certificate.VerifyHostname(ip.String())
	}
	return certificate.VerifyHostname(serverName)
}

func verifyCertificateChain(certificates []*x509.Certificate, now time.Time) error {
	if len(certificates) == 0 || certificates[0] == nil {
		return errors.New("certificate chain is missing")
	}
	intermediates := x509.NewCertPool()
	for _, certificate := range certificates[1:] {
		if certificate != nil {
			intermediates.AddCert(certificate)
		}
	}
	_, err := certificates[0].Verify(x509.VerifyOptions{
		Intermediates: intermediates,
		CurrentTime:   now,
	})
	return err
}

func certificateTrustError(chainErr, hostnameErr error, selfSigned bool) string {
	parts := make([]string, 0, 2)
	if chainErr != nil && !selfSigned {
		parts = append(parts, "trust chain: "+chainErr.Error())
	}
	if hostnameErr != nil {
		parts = append(parts, "hostname: "+hostnameErr.Error())
	}
	return strings.Join(parts, ". ")
}
