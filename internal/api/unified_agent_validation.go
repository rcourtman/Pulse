package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentidentity"
)

type agentArtifactChangeStamp struct{ seconds, nanoseconds int64 }

type unifiedAgentValidationEntry struct {
	change                    agentArtifactChangeStamp
	info                      os.FileInfo
	version, target, checksum string
}

// Cache only artifact facts, including inode and filesystem change time. Expected release and
// target are part of the admission key. Authentication decisions are not cached.
func (r *Router) openValidatedUnifiedAgent(path, version, target string) (*os.File, os.FileInfo, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, "", errors.New("agent artifact is not a regular file")
	}
	change, changeKnown := agentArtifactChangeTime(info)
	if value, ok := r.unifiedAgentValidationCache.Load(path); ok && changeKnown {
		entry := value.(unifiedAgentValidationEntry)
		if entry.change == change && entry.version == version && entry.target == target && entry.info.Size() == info.Size() && entry.info.ModTime().Equal(info.ModTime()) && os.SameFile(entry.info, info) {
			keep = true
			return file, info, entry.checksum, nil
		}
	}
	checksum, err := inspectUnifiedAgentBinary(file, info.Size(), version, target)
	if err != nil {
		return nil, nil, "", err
	}
	after, err := file.Stat()
	if err != nil {
		return nil, nil, "", err
	}
	afterChange, afterKnown := agentArtifactChangeTime(after)
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) || (changeKnown && (!afterKnown || afterChange != change)) {
		return nil, nil, "", errors.New("agent artifact changed during validation")
	}
	if changeKnown {
		r.unifiedAgentValidationCache.Store(path, unifiedAgentValidationEntry{info: info, change: change, version: version, target: target, checksum: checksum})
	}
	keep = true
	return file, info, checksum, nil
}

func inspectUnifiedAgentBinary(reader io.ReaderAt, size int64, version, target string) (string, error) {
	if err := agentidentity.ValidateReader(reader, version, target); err != nil {
		return "", err
	}
	hash := sha256.New()
	canonical, legacy, err := scanUnifiedAgentBinaryContract(io.TeeReader(io.NewSectionReader(reader, 0, size), hash))
	if err != nil {
		return "", fmt.Errorf("scan binary contract: %w", err)
	}
	if legacy {
		return "", fmt.Errorf("references deprecated host endpoint %s", legacyUnifiedAgentReportPath)
	}
	if !canonical {
		return "", fmt.Errorf("missing canonical host endpoint %s", canonicalUnifiedAgentReportPath)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
