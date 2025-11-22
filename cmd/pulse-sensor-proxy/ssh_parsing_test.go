package main

import (
	"testing"
)

func TestParseClusterNodes(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    []string
		wantErr bool
	}{
		{
			name: "standard output",
			output: `Membership information
----------------------
    Nodeid      Votes Name
0x00000001          1 192.168.1.10
0x00000002          1 192.168.1.11`,
			want: []string{"192.168.1.10", "192.168.1.11"},
		},
		{
			name: "output with QDevice flags",
			output: `Membership information
----------------------
    Nodeid      Votes Name
0x00000001          1 A,V,NMW 192.168.1.10
0x00000002          1 A,V,NMW 192.168.1.11`,
			want: []string{"192.168.1.10", "192.168.1.11"},
		},
		{
			name: "output with local suffix",
			output: `Membership information
----------------------
    Nodeid      Votes Name
0x00000001          1 192.168.1.10 (local)
0x00000002          1 192.168.1.11`,
			want: []string{"192.168.1.10", "192.168.1.11"},
		},
		{
			name: "mixed output",
			output: `Membership information
----------------------
    Nodeid      Votes Name
0x00000001          1 A,V,NMW 192.168.1.10 (local)
0x00000002          1 192.168.1.11`,
			want: []string{"192.168.1.10", "192.168.1.11"},
		},
		{
			name:    "no nodes",
			output:  "some random text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClusterNodes(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseClusterNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseClusterNodes() got = %v, want %v", got, tt.want)
				} else {
					for i := range got {
						if got[i] != tt.want[i] {
							t.Errorf("parseClusterNodes() got[%d] = %v, want %v", i, got[i], tt.want[i])
						}
					}
				}
			}
		})
	}
}
