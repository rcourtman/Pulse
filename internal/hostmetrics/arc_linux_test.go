//go:build linux

package hostmetrics

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadARCStats(t *testing.T) {
	const sample = `5 1 0x01 87 5704 54903991946
name                            type data
hits                            4    1234567
misses                          4    234567
c_max                           4    33554432000
`
	for _, tc := range []struct {
		name, counter string
		want          uint64
		missing       bool
		wantErr       bool
	}{
		{name: "observed size", counter: "16873652224", want: 16873652224},
		{name: "zero size", counter: "0"},
		{name: "no size row"},
		{name: "no file", missing: true},
		{name: "invalid size", counter: "not-a-counter", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "arcstats")
			if !tc.missing {
				data := sample
				if tc.counter != "" {
					data += fmt.Sprintf("size                            4    %s\n", tc.counter)
				}
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if got, err := readARCStats(path); (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("ARC bytes = %d, err=%v; want %d, want error=%t", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
