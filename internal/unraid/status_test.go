package unraid

import "testing"

func TestNormalizeNativeIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "ata-_", want: ""},
		{input: " ATA - ", want: ""},
		{input: "_", want: ""},
		{input: "ata-Samsung_SSD_S6BC", want: "ata-Samsung_SSD_S6BC"},
		{input: "S6BCNG0R213032E", want: "S6BCNG0R213032E"},
	}

	for _, test := range tests {
		if got := NormalizeNativeIdentity(test.input); got != test.want {
			t.Errorf("NormalizeNativeIdentity(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestIsExplicitMissingMember(t *testing.T) {
	t.Parallel()

	if !IsExplicitMissingMember("DISK_NP_MISSING") {
		t.Fatal("DISK_NP_MISSING must identify an assigned missing member")
	}
	for _, status := range []string{"DISK_NP", "DISK_NP_DSBL", "DISK_OK", ""} {
		if IsExplicitMissingMember(status) {
			t.Errorf("%q must not identify an assigned missing member", status)
		}
	}
}

func TestHasFilesystemEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{input: "", want: false},
		{input: "  ", want: false},
		{input: "auto", want: false},
		{input: " AUTO ", want: false},
		{input: "xfs", want: true},
		{input: "luks:xfs", want: true},
		{input: "btrfs", want: true},
	}

	for _, test := range tests {
		if got := HasFilesystemEvidence(test.input); got != test.want {
			t.Errorf("HasFilesystemEvidence(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}
