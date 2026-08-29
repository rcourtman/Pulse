//go:build !unix

package agenthelper

import (
	"errors"
	"os"
)

func openFileNoFollow(string) (*os.File, error) {
	return nil, errors.New("privileged update activation is supported only on Unix")
}

func StrictRootOwnedFile(*os.File) error {
	return errors.New("root ownership validation is supported only on Unix")
}

func FileOwnedByUID(uint32) func(*os.File) error {
	return func(*os.File) error {
		return errors.New("file ownership validation is supported only on Unix")
	}
}
