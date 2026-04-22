package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/updatesignature"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  release_update_key.go public-key --private-key <base64-ed25519-key-or-seed>")
	fmt.Fprintln(os.Stderr, "  release_update_key.go public-key-ssh --private-key <base64-ed25519-key-or-seed> [--comment <comment>]")
	fmt.Fprintln(os.Stderr, "  release_update_key.go openssh-private-key --private-key <base64-ed25519-key-or-seed> [--comment <comment>]")
	fmt.Fprintln(os.Stderr, "  release_update_key.go fingerprint (--private-key <base64-ed25519-key-or-seed> | --public-key <base64-ed25519-public-key>)")
	fmt.Fprintln(os.Stderr, "  release_update_key.go sign --private-key <base64-ed25519-key-or-seed> --file <path>")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "public-key":
		publicKeyCmd := flag.NewFlagSet("public-key", flag.ExitOnError)
		privateKey := publicKeyCmd.String("private-key", "", "base64-encoded Ed25519 private key or seed")
		_ = publicKeyCmd.Parse(os.Args[2:])

		key, err := updatesignature.DecodePrivateKey(*privateKey)
		if err != nil {
			fail(err)
		}
		encoded, err := updatesignature.PublicKeyString(key)
		if err != nil {
			fail(err)
		}
		fmt.Println(encoded)
	case "public-key-ssh":
		publicKeyCmd := flag.NewFlagSet("public-key-ssh", flag.ExitOnError)
		privateKey := publicKeyCmd.String("private-key", "", "base64-encoded Ed25519 private key or seed")
		comment := publicKeyCmd.String("comment", "", "optional SSH key comment")
		_ = publicKeyCmd.Parse(os.Args[2:])

		key, err := updatesignature.DecodePrivateKey(*privateKey)
		if err != nil {
			fail(err)
		}
		encoded, err := updatesignature.AuthorizedPublicKeyString(key, *comment)
		if err != nil {
			fail(err)
		}
		fmt.Println(encoded)
	case "openssh-private-key":
		privateKeyCmd := flag.NewFlagSet("openssh-private-key", flag.ExitOnError)
		privateKey := privateKeyCmd.String("private-key", "", "base64-encoded Ed25519 private key or seed")
		comment := privateKeyCmd.String("comment", "", "optional SSH key comment")
		_ = privateKeyCmd.Parse(os.Args[2:])

		key, err := updatesignature.DecodePrivateKey(*privateKey)
		if err != nil {
			fail(err)
		}
		encoded, err := updatesignature.OpenSSHPrivateKeyPEM(key, *comment)
		if err != nil {
			fail(err)
		}
		fmt.Print(encoded)
	case "fingerprint":
		fingerprintCmd := flag.NewFlagSet("fingerprint", flag.ExitOnError)
		privateKey := fingerprintCmd.String("private-key", "", "base64-encoded Ed25519 private key or seed")
		publicKey := fingerprintCmd.String("public-key", "", "base64-encoded Ed25519 public key or PKIX public key")
		_ = fingerprintCmd.Parse(os.Args[2:])

		if (*privateKey == "") == (*publicKey == "") {
			usage()
		}

		var (
			derivedPublicKey ed25519.PublicKey
			err              error
		)
		if *privateKey != "" {
			key, decodeErr := updatesignature.DecodePrivateKey(*privateKey)
			if decodeErr != nil {
				fail(decodeErr)
			}
			var ok bool
			derivedPublicKey, ok = key.Public().(ed25519.PublicKey)
			if !ok {
				fail(fmt.Errorf("failed to derive Ed25519 public key"))
			}
		} else {
			derivedPublicKey, err = decodePublicKey(*publicKey)
			if err != nil {
				fail(err)
			}
		}

		sum := sha256.Sum256(derivedPublicKey)
		fmt.Printf("SHA256:%s\n", base64.StdEncoding.EncodeToString(sum[:]))
	case "sign":
		signCmd := flag.NewFlagSet("sign", flag.ExitOnError)
		privateKey := signCmd.String("private-key", "", "base64-encoded Ed25519 private key or seed")
		filePath := signCmd.String("file", "", "path to the file to sign")
		_ = signCmd.Parse(os.Args[2:])

		if *filePath == "" {
			usage()
		}
		key, err := updatesignature.DecodePrivateKey(*privateKey)
		if err != nil {
			fail(err)
		}
		signature, err := updatesignature.SignFile(*filePath, key)
		if err != nil {
			fail(err)
		}
		fmt.Println(signature)
	default:
		usage()
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 public key: %w", err)
	}
	if len(decoded) == ed25519.PublicKeySize {
		return ed25519.PublicKey(decoded), nil
	}

	publicKey, err := x509.ParsePKIXPublicKey(decoded)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ed25519 public key: %w", err)
	}
	ed25519PublicKey, ok := publicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("trusted key is not an Ed25519 public key")
	}
	return ed25519PublicKey, nil
}
