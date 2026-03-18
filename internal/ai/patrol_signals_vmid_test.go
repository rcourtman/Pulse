package ai

import (
	"strings"
	"testing"
)

func TestVMIDValueUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		initial     VMIDValue
		want        VMIDValue
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty payload",
			data:    []byte{},
			want:    "",
			wantErr: false,
		},
		{
			name:    "null payload",
			data:    []byte("null"),
			want:    "",
			wantErr: false,
		},
		{
			name:    "string payload",
			data:    []byte(`"  104  "`),
			want:    "104",
			wantErr: false,
		},
		{
			name:    "numeric payload",
			data:    []byte("205"),
			want:    "205",
			wantErr: false,
		},
		{
			name:        "invalid payload",
			data:        []byte("true"),
			initial:     "unchanged",
			want:        "unchanged",
			wantErr:     true,
			errContains: "invalid VMID value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.initial
			err := (&value).UnmarshalJSON(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error to contain %q, got %q", tc.errContains, err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if value != tc.want {
				t.Fatalf("expected VMID %q, got %q", tc.want, value)
			}
		})
	}
}
