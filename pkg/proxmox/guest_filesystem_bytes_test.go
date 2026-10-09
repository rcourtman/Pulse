package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVMFilesystemBytesRejectUnknownOrContradictoryUsage(t *testing.T) {
	for name, counters := range map[string]string{
		"missing-used":               `"total-bytes":1000`,
		"null-used":                  `"total-bytes":1000,"used-bytes":null`,
		"empty-used":                 `"total-bytes":1000,"used-bytes":""`,
		"negative-used":              `"total-bytes":1000,"used-bytes":-1`,
		"negative-string":            `"total-bytes":1000,"used-bytes":"-1.0"`,
		"fractional-used":            `"total-bytes":1000,"used-bytes":0.5`,
		"fractional-string":          `"total-bytes":1000,"used-bytes":"1e-1"`,
		"used-exceeds-total":         `"total-bytes":1000,"used-bytes":1001`,
		"signed-overflow":            `"total-bytes":9223372036854775808,"used-bytes":1`,
		"uint-overflow":              `"total-bytes":18446744073709551616,"used-bytes":1`,
		"exponent-overflow":          `"total-bytes":"1e1000000","used-bytes":1`,
		"invalid-zero-exponent":      `"total-bytes":1000,"used-bytes":"0e9999999999999999"`,
		"negative-total":             `"total-bytes":-1000,"used-bytes":0`,
		"fractional-total":           `"total-bytes":"1000.5","used-bytes":0`,
		"invalid-privileged":         `"total-bytes":1000,"total-bytes-privileged":-1,"used-bytes":0`,
		"missing-privileged-used":    `"total-bytes-privileged":1000`,
		"null-privileged-used":       `"total-bytes":0,"total-bytes-privileged":1000,"used-bytes":null`,
		"missing-total-nonzero-used": `"used-bytes":10`,
		"duplicate-used":             `"total-bytes":1000,"used-bytes":1001,"used-bytes":0`,
		"escaped-duplicate":          `"total-bytes":1000,"used-bytes":1001,"\u0075sed-bytes":0`,
		"case-conflict":              `"total-bytes":1000,"used-bytes":1001,"Used-Bytes":0`,
		"case-only":                  `"total-bytes":1000,"Used-Bytes":0`,
		"duplicate-total":            `"total-bytes":1,"total-bytes":1000,"used-bytes":100`,
		"case-total":                 `"Total-Bytes":1000,"used-bytes":0`,
		"bool-used":                  `"total-bytes":1000,"used-bytes":false`,
		"nan-used":                   `"total-bytes":1000,"used-bytes":"NaN"`,
	} {
		t.Run(name, func(t *testing.T) {
			var fs VMFileSystem
			if err := json.Unmarshal([]byte(`{"mountpoint":"/","type":"ext4",`+counters+`}`), &fs); err == nil {
				t.Fatalf("untrustworthy byte counters became a filesystem reading: %+v", fs)
			}
		})
	}
}

func TestVMFilesystemBytesPreserveExactCompatibleReadings(t *testing.T) {
	for name, tc := range map[string]struct {
		counters    string
		total, used uint64
	}{
		"integer":             {`"total-bytes":1000,"used-bytes":400`, 1000, 400},
		"explicit-zero":       {`"total-bytes":1000,"used-bytes":0`, 1000, 0},
		"full":                {`"total-bytes":1000,"used-bytes":1000`, 1000, 1000},
		"numeric-strings":     {`"total-bytes":" 1000 ","used-bytes":"400"`, 1000, 400},
		"decimal":             {`"total-bytes":1000.0,"used-bytes":"400.00"`, 1000, 400},
		"scientific":          {`"total-bytes":1e3,"used-bytes":"4.00e2"`, 1000, 400},
		"small-exponent":      {`"total-bytes":"100000e-2","used-bytes":"4000e-1"`, 1000, 400},
		"hexadecimal":         {`"total-bytes":"0x3e8","used-bytes":"0X190"`, 1000, 400},
		"large-exact-integer": {`"total-bytes":9007199254740993,"used-bytes":9007199254740991`, 9007199254740993, 9007199254740991},
		"large-exact-decimal": {`"total-bytes":"9007199254740993.0","used-bytes":"9007199254740991.0"`, 9007199254740993, 9007199254740991},
		"signed-boundary":     {`"total-bytes":9223372036854775807,"used-bytes":9223372036854775806`, math.MaxInt64, math.MaxInt64 - 1},
		"privileged-fallback": {`"total-bytes":null,"total-bytes-privileged":1000,"used-bytes":400`, 1000, 400},
		"zero-total-fallback": {`"total-bytes":0,"total-bytes-privileged":"1000.0","used-bytes":400`, 1000, 400},
		"unformatted":         {`"total-bytes":0`, 0, 0},
		"metadata-only":       {`"disk":[{"dev":"/dev/vda"}]`, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			var fs VMFileSystem
			if err := json.Unmarshal([]byte(`{"mountpoint":"/","type":"ext4",`+tc.counters+`}`), &fs); err != nil {
				t.Fatal(err)
			}
			if fs.TotalBytes != tc.total || fs.UsedBytes != tc.used {
				t.Fatalf("bytes = %d/%d, want %d/%d", fs.TotalBytes, fs.UsedBytes, tc.total, tc.used)
			}
		})
	}
}

func TestVMFilesystemBytesWireAdmissionKeepsValidPeersAndObjectPrecision(t *testing.T) {
	for name, tc := range map[string]struct {
		result      string
		total, used uint64
		wantError   bool
	}{
		"array-peers":      {`[{"mountpoint":"/bad","type":"ext4","total-bytes":1000},{"mountpoint":"/negative","type":"ext4","total-bytes":1000,"used-bytes":-1},{"mountpoint":"/overfull","type":"ext4","total-bytes":1000,"used-bytes":1001},{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":0}]`, 1000, 0, false},
		"object-precision": {`{"mountpoint":"C:\\","type":"ntfs","total-bytes":9007199254740993,"used-bytes":9007199254740991,"disk":{"bus-type":"scsi","target":2}}`, 9007199254740993, 9007199254740991, false},
		"object-duplicate": {`{"mountpoint":"C:\\","type":"ntfs","total-bytes":1000,"used-bytes":1001,"used-bytes":0}`, 0, 0, true},
		"object-missing":   {`{"mountpoint":"C:\\","type":"ntfs","total-bytes":1000}`, 0, 0, true},
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				calls.Add(1)
				if strings.HasSuffix(r.URL.Path, "get-fsinfo") {
					fmt.Fprintf(w, `{"data":{"result":%s}}`, tc.result)
					return
				}
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			c := backupTestClient(t, server.URL)
			readings, err := c.GetVMFSInfo(context.Background(), "node", 105)
			if tc.wantError {
				if err == nil || len(readings) != 0 {
					t.Fatalf("invalid object admitted: %+v %v", readings, err)
				}
			} else {
				if err != nil || len(readings) != 1 {
					t.Fatalf("valid readings lost: %+v %v", readings, err)
				}
				if readings[0].TotalBytes != tc.total || readings[0].UsedBytes != tc.used {
					t.Fatalf("wire bytes rounded or defaulted: %+v", readings[0])
				}
				if name == "object-precision" && readings[0].Disk != "scsi-2" {
					t.Fatalf("disk metadata compatibility changed: %+v", readings[0])
				}
			}
			// Bad counters are a completed reply with unusable readings. They do not
			// change serial-command admission, replay the request or start a cooldown.
			if _, err := c.GetVMAgentVersion(context.Background(), "node", 105); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 {
				t.Fatalf("wire requests = %d, want two single attempts", calls.Load())
			}
		})
	}
}
