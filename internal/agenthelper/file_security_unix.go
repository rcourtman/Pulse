//go:build unix

package agenthelper

import (
	"errors"
	"os"
	"syscall"
)

func openFileNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}

func StrictRootOwnedFile(file *os.File) error {
	return FileOwnedByUID(0)(file)
}

func FileOwnedByUID(uid uint32) func(*os.File) error {
	return func(file *os.File) error {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uid {
			return errors.New("file ownership does not match the required UID")
		}
		return nil
	}
}
