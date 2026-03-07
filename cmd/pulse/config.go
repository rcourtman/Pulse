package main

import (
	"io"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

func readBoundedRegularFile(path string, maxBytes int64) ([]byte, error) {
	return pulsecli.ReadBoundedRegularFile(path, maxBytes)
}

func readBoundedHTTPBody(reader io.Reader, declaredLength, maxBytes int64, source string) ([]byte, error) {
	return pulsecli.ReadBoundedHTTPBody(reader, declaredLength, maxBytes, source)
}

func getPassphrase(prompt string, confirm bool) string {
	return pulsecli.GetPassphrase(configDeps, prompt, confirm)
}
