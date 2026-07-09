package hostagent

import (
	"io/fs"
	"os"
	"testing"
	"time"
)

// smartctlNoDataJSON is a syntactically valid smartctl JSON envelope with no
// usable SMART payload, as produced when a probe addresses a device with the
// wrong -d type.
const smartctlNoDataJSON = `{
	"device": {"name": "/dev/sda", "type": "sat", "protocol": "ATA"}
}`

const smartctlUntypedHealthOnlyJSON = `{
	"device": {"name": "/dev/sda", "type": "scsi", "protocol": "SCSI"},
	"model_name": "WDC_WD80EFPX-68C4ZN0",
	"serial_number": "WD-SAT-TEMP-1",
	"smart_status": {"passed": true}
}`

const smartctlSATTemperatureAttributeJSON = `{
	"device": {"name": "/dev/sda", "type": "sat", "protocol": "ATA"},
	"model_name": "WDC_WD80EFPX-68C4ZN0",
	"serial_number": "WD-SAT-TEMP-1",
	"smart_status": {"passed": true},
	"ata_smart_attributes": {
		"table": [
			{"id": 194, "name": "Temperature_Celsius", "raw": {"value": 0, "string": "32"}}
		]
	}
}`

type fakeDirEntry struct {
	name string
}

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return false }
func (e fakeDirEntry) Type() fs.FileMode          { return 0 }
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func stubLinuxSysfs(t *testing.T, entries []string, files map[string]string) {
	t.Helper()

	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	origEvalSymlinks := smartctlEvalSymlinks
	origNow := timeNow
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		readDir = origReadDir
		smartctlReadFile = origReadFile
		smartctlEvalSymlinks = origEvalSymlinks
		timeNow = origNow
	})

	runtimeGOOS = "linux"
	timeNow = func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) }
	readDir = func(string) ([]os.DirEntry, error) {
		dirEntries := make([]os.DirEntry, 0, len(entries))
		for _, name := range entries {
			dirEntries = append(dirEntries, fakeDirEntry{name: name})
		}
		return dirEntries, nil
	}
	smartctlReadFile = func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return []byte(content), nil
		}
		return nil, fs.ErrNotExist
	}
	smartctlEvalSymlinks = func(string) (string, error) { return "", fs.ErrNotExist }
}
