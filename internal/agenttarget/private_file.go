package agenttarget

import (
	"fmt"
	"io"
	"os"
)

func readPrivateRegularFile(path, label string, maxBytes int64) ([]byte, error) {
	return readPrivateRegularFileWithOpener(path, label, maxBytes, openPrivateFile)
}

// Check obvious invalid paths before opening, then validate the descriptor we
// actually read. Unix opens neither follow a final symlink nor wait on a FIFO
// that replaced a regular file after the pathname check. These are local file
// checks, not a deadline for regular-file I/O on an unavailable filesystem.
func readPrivateRegularFileWithOpener(path, label string, maxBytes int64, open func(string) (*os.File, error)) ([]byte, error) {
	if err := requirePrivateRegularFile(path, label, maxBytes); err != nil {
		return nil, err
	}
	file, err := open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", label, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened %s file: %w", label, err)
	}
	if err := validatePrivateRegularFile(info, label, maxBytes); err != nil {
		return nil, err
	}
	return readPrivateFileContents(file, label, maxBytes)
}

// The stat size is only an early refusal, not an allocation or EOF promise:
// files can grow and some regular files report less than they actually serve.
// Return no partial credential/configuration, including on a late read error.
func readPrivateFileContents(reader io.Reader, label string, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s file exceeds the %d-byte size limit", label, maxBytes)
	}
	return data, nil
}
