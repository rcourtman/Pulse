//go:build windows

package collectorlifecycle

import "os"

func credentialFileModePrivate(_ os.FileInfo) bool {
	// Windows does not enforce Unix permission bits. validateCredentialFileOwner
	// applies the equivalent owner and protected-DACL policy after this check.
	return true
}
