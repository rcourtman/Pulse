package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/updatesignature"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  release_update_key.go public-key --private-key <base64-ed25519-key-or-seed>")
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
