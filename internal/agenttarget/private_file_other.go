//go:build !unix

package agenttarget

import "os"

func openPrivateFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Non-Unix platforms retain the pathname symlink check, and verify that
	// it still names the opened regular file before reading any bytes.
	info, err := file.Stat()
	if err == nil {
		var pathInfo os.FileInfo
		pathInfo, err = os.Lstat(path)
		if err == nil && (pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, pathInfo)) {
			err = os.ErrInvalid
		}
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
