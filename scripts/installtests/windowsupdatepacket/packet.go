// Package windowsupdatepacket transfers production-handler responses between
// the Linux server and Windows consumer checks in one exact-source CI run.
// It is test support, not an alternate download implementation or release packet.
package windowsupdatepacket

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// Published-shape versions exercise required release sidecars. These are
// disposable source fixtures, not fetched tags, release inputs or releases.
const Before = "v6.0.5-rc.1"
const After = "v6.0.5-rc.2"

type Response struct {
	File      string `json:"file"`
	Checksum  string `json:"checksum"`
	Signature string `json:"signature"`
	SSHSIG    string `json:"sshsig"`
}

type Packet struct {
	Source    string              `json:"source"`
	PublicKey string              `json:"public_key"`
	Responses map[string]Response `json:"responses"`
}

func Digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Load admits only the fixed cases and files. Neither artifact metadata nor a
// response can select a host, executable path, request header or signing key.
func Load(directory, source string) (*Packet, map[string][]byte, error) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(source) {
		return nil, nil, fmt.Errorf("missing exact source identity")
	}
	file, err := os.Open(filepath.Join(directory, "packet.json"))
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	var packet Packet
	decoder := json.NewDecoder(io.LimitReader(file, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&packet); err != nil {
		return nil, nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, nil, fmt.Errorf("trailing packet content")
	}
	if packet.Source != source || len(packet.Responses) != 3 {
		return nil, nil, fmt.Errorf("packet source or case inventory differs")
	}
	key, err := base64.StdEncoding.DecodeString(packet.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, nil, fmt.Errorf("invalid disposable public key")
	}
	bodies := make(map[string][]byte)
	for name, filename := range map[string]string{
		"initial": "initial.exe", "valid": "candidate.exe", "bad-signature": "candidate.exe",
	} {
		response, ok := packet.Responses[name]
		if !ok || response.File != filename || response.SSHSIG == "" {
			return nil, nil, fmt.Errorf("missing fixed response %s", name)
		}
		path := filepath.Join(directory, filename)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() < 1024*1024 || info.Size() > 100*1024*1024 {
			return nil, nil, fmt.Errorf("invalid response file for %s", name)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		if len(body) < 1024*1024 || len(body) > 100*1024*1024 || Digest(body) != response.Checksum {
			return nil, nil, fmt.Errorf("response body identity differs for %s", name)
		}
		signature, err := base64.StdEncoding.DecodeString(response.Signature)
		if err != nil || len(signature) != ed25519.SignatureSize {
			return nil, nil, fmt.Errorf("invalid response signature encoding")
		}
		valid := ed25519.Verify(key, body, signature)
		if valid != (name != "bad-signature") {
			return nil, nil, fmt.Errorf("signature control differs for %s", name)
		}
		bodies[name] = body
	}
	return &packet, bodies, nil
}
