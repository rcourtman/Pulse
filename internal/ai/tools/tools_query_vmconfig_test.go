package tools

import (
	"testing"
)

func toolsvmconfigBoolPtr(b bool) *bool { return &b }

func toolsvmconfigBoolEqual(got, want *bool) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func toolsvmconfigDisksEqual(got, want []GuestDiskConfig) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestParseVMConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantOS    string
		wantBoot  *bool
		wantDisks []GuestDiskConfig
	}{
		{
			name:      "nil map returns zero values",
			config:    nil,
			wantOS:    "",
			wantBoot:  nil,
			wantDisks: nil,
		},
		{
			name:      "empty map returns zero values",
			config:    map[string]interface{}{},
			wantOS:    "",
			wantBoot:  nil,
			wantDisks: nil,
		},
		{
			name:     "ostype only",
			config:   map[string]interface{}{"ostype": "l26"},
			wantOS:   "l26",
			wantBoot: nil,
		},
		{
			name:     "onboot true via one string",
			config:   map[string]interface{}{"onboot": "1"},
			wantOS:   "",
			wantBoot: toolsvmconfigBoolPtr(true),
		},
		{
			name:     "onboot false via zero string",
			config:   map[string]interface{}{"onboot": "0"},
			wantOS:   "",
			wantBoot: toolsvmconfigBoolPtr(false),
		},
		{
			name:   "single scsi disk",
			config: map[string]interface{}{"scsi0": "local-lvm:vm-100-disk-0,size=32G"},
			wantDisks: []GuestDiskConfig{
				{Key: "scsi0", Value: "local-lvm:vm-100-disk-0,size=32G"},
			},
		},
		{
			name: "multiple disks sorted by key ascending",
			config: map[string]interface{}{
				"virtio0": "storage:vm-100-virtio-0",
				"scsi1":   "storage:vm-100-scsi-1",
				"ide0":    "storage:vm-100-ide-0",
				"sata0":   "storage:vm-100-sata-0",
			},
			wantDisks: []GuestDiskConfig{
				{Key: "ide0", Value: "storage:vm-100-ide-0"},
				{Key: "sata0", Value: "storage:vm-100-sata-0"},
				{Key: "scsi1", Value: "storage:vm-100-scsi-1"},
				{Key: "virtio0", Value: "storage:vm-100-virtio-0"},
			},
		},
		{
			name: "all disk prefixes collected and sorted",
			config: map[string]interface{}{
				"scsi0":     "a",
				"virtio0":   "b",
				"sata0":     "c",
				"ide0":      "d",
				"unused0":   "e",
				"efidisk0":  "f",
				"tpmstate0": "g",
			},
			wantDisks: []GuestDiskConfig{
				{Key: "efidisk0", Value: "f"},
				{Key: "ide0", Value: "d"},
				{Key: "sata0", Value: "c"},
				{Key: "scsi0", Value: "a"},
				{Key: "tpmstate0", Value: "g"},
				{Key: "unused0", Value: "e"},
				{Key: "virtio0", Value: "b"},
			},
		},
		{
			name:     "mixed ostype onboot and disk",
			config:   map[string]interface{}{"ostype": "win11", "onboot": "yes", "scsi0": "local:disk-0"},
			wantOS:   "win11",
			wantBoot: toolsvmconfigBoolPtr(true),
			wantDisks: []GuestDiskConfig{
				{Key: "scsi0", Value: "local:disk-0"},
			},
		},
		{
			name:   "disk with empty value skipped",
			config: map[string]interface{}{"scsi0": "", "scsi1": "real"},
			wantDisks: []GuestDiskConfig{
				{Key: "scsi1", Value: "real"},
			},
		},
		{
			name:   "disk key casing normalized to lower",
			config: map[string]interface{}{"SCSI0": "local:disk-0"},
			wantDisks: []GuestDiskConfig{
				{Key: "scsi0", Value: "local:disk-0"},
			},
		},
		{
			name:     "ostype key matched case insensitively",
			config:   map[string]interface{}{"OSTYPE": "l26"},
			wantOS:   "l26",
			wantBoot: nil,
		},
		{
			name:   "key surrounding whitespace trimmed",
			config: map[string]interface{}{"  ostype  ": "win10", "  scsi0  ": "local:disk"},
			wantOS: "win10",
			wantDisks: []GuestDiskConfig{
				{Key: "scsi0", Value: "local:disk"},
			},
		},
		{
			name:   "disk value whitespace trimmed",
			config: map[string]interface{}{"scsi0": "   local:disk-0   "},
			wantDisks: []GuestDiskConfig{
				{Key: "scsi0", Value: "local:disk-0"},
			},
		},
		{
			name:      "non-disk non-special keys ignored",
			config:    map[string]interface{}{"net0": "virtio,bridge=vmbr0", "memory": 4096, "cores": 4},
			wantOS:    "",
			wantBoot:  nil,
			wantDisks: nil,
		},
		{
			name:     "onboot uncoercible value yields nil",
			config:   map[string]interface{}{"onboot": "maybe"},
			wantOS:   "",
			wantBoot: nil,
		},
		{
			name:   "disk nil interface value rendered as literal string",
			config: map[string]interface{}{"scsi0": nil},
			wantDisks: []GuestDiskConfig{
				{Key: "scsi0", Value: "<nil>"},
			},
		},
		{
			name:     "onboot integer coerced via sprint",
			config:   map[string]interface{}{"onboot": 1},
			wantOS:   "",
			wantBoot: toolsvmconfigBoolPtr(true),
		},
		{
			name:   "single disk not sorted when only one",
			config: map[string]interface{}{"virtio0": "v"},
			wantDisks: []GuestDiskConfig{
				{Key: "virtio0", Value: "v"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOS, gotBoot, gotDisks := parseVMConfig(tc.config)
			if gotOS != tc.wantOS {
				t.Errorf("osType = %q, want %q", gotOS, tc.wantOS)
			}
			if !toolsvmconfigBoolEqual(gotBoot, tc.wantBoot) {
				t.Errorf("onboot mismatch: got=%v, want=%v", gotBoot, tc.wantBoot)
			}
			if !toolsvmconfigDisksEqual(gotDisks, tc.wantDisks) {
				t.Errorf("disks = %+v, want %+v", gotDisks, tc.wantDisks)
			}
		})
	}
}

func TestIsVMConfigDiskKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"scsi prefix", "scsi0", true},
		{"scsi exact", "scsi", true},
		{"virtio prefix", "virtio0", true},
		{"virtio exact", "virtio", true},
		{"sata prefix", "sata2", true},
		{"sata exact", "sata", true},
		{"ide prefix", "ide0", true},
		{"ide exact", "ide", true},
		{"unused prefix", "unused0", true},
		{"unused exact", "unused", true},
		{"efidisk prefix", "efidisk0", true},
		{"efidisk exact", "efidisk", true},
		{"tpmstate prefix", "tpmstate0", true},
		{"tpmstate exact", "tpmstate", true},
		{"net is not a disk key", "net0", false},
		{"memory is not a disk key", "memory", false},
		{"cores is not a disk key", "cores", false},
		{"ostype is not a disk key", "ostype", false},
		{"onboot is not a disk key", "onboot", false},
		{"rootfs is not a disk key", "rootfs", false},
		{"disk0 is not a recognized disk key", "disk0", false},
		{"empty string is not a disk key", "", false},
		{"uppercase scsi not matched case sensitive", "SCSI0", false},
		{"uppercase ide not matched case sensitive", "IDE0", false},
		{"prefix match has no word boundary ide in ideology", "ideology", true},
		{"prefix match has no word boundary scsi in scsiabc", "scsiabc", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVMConfigDiskKey(tc.key); got != tc.want {
				t.Errorf("isVMConfigDiskKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestParseOnbootValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  *bool
	}{
		{"empty string", "", nil},
		{"string one", "1", toolsvmconfigBoolPtr(true)},
		{"string yes lowercase", "yes", toolsvmconfigBoolPtr(true)},
		{"string yes uppercase", "YES", toolsvmconfigBoolPtr(true)},
		{"string yes mixed case", "Yes", toolsvmconfigBoolPtr(true)},
		{"string true lowercase", "true", toolsvmconfigBoolPtr(true)},
		{"string true uppercase", "TRUE", toolsvmconfigBoolPtr(true)},
		{"string true mixed case", "True", toolsvmconfigBoolPtr(true)},
		{"string zero", "0", toolsvmconfigBoolPtr(false)},
		{"string no lowercase", "no", toolsvmconfigBoolPtr(false)},
		{"string no uppercase", "NO", toolsvmconfigBoolPtr(false)},
		{"string false lowercase", "false", toolsvmconfigBoolPtr(false)},
		{"string false uppercase", "FALSE", toolsvmconfigBoolPtr(false)},
		{"whitespace padded one", " 1 ", toolsvmconfigBoolPtr(true)},
		{"whitespace padded yes", " yes ", toolsvmconfigBoolPtr(true)},
		{"whitespace padded false", " false ", toolsvmconfigBoolPtr(false)},
		{"string two unrecognized", "2", nil},
		{"string maybe unrecognized", "maybe", nil},
		{"string on not recognized actual behavior", "on", nil},
		{"integer one coerced to true", 1, toolsvmconfigBoolPtr(true)},
		{"integer zero coerced to false", 0, toolsvmconfigBoolPtr(false)},
		{"integer two unrecognized", 2, nil},
		{"boolean true coerced", true, toolsvmconfigBoolPtr(true)},
		{"boolean false coerced", false, toolsvmconfigBoolPtr(false)},
		{"nil interface returns nil", nil, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseOnbootValue(tc.value)
			if !toolsvmconfigBoolEqual(got, tc.want) {
				t.Errorf("parseOnbootValue(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
