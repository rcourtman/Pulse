package dockeragent

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/updatesignature"
)

// verifySignature checks if the provided binary data matches the signature
// using the trusted release public keys embedded into release builds.
func verifySignature(binaryData []byte, signatureBase64 string) error {
	return updatesignature.VerifyBytes(binaryData, signatureBase64)
}

// verifyFileSignature reads the file and verifies its signature.
func verifyFileSignature(path string, signatureBase64 string) error {
	return updatesignature.VerifyFile(path, signatureBase64)
}
