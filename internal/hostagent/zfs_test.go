package hostagent

import (
	"testing"
)

func TestParseZpoolStatusMembers(t *testing.T) {
	output := `  pool: tank
 state: ONLINE
  scan: none requested
config:

	NAME                                          STATE     READ WRITE CKSUM
	tank                                          ONLINE       0     0     0
	  mirror-0                                    ONLINE       0     0     0
	    /dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500001Z-part3  ONLINE       0     0     0
	    /dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500002Z-part3  ONLINE       0     0     0
	logs
	  /dev/nvme0n1p1                              ONLINE       0     0     0
	cache
	  /dev/sdc                                    ONLINE       0     0     0

errors: No known data errors
`

	got := parseZpoolStatusMembers("tank", output)
	want := []string{
		"/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500001Z-part3",
		"/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500002Z-part3",
		"/dev/nvme0n1p1",
		"/dev/sdc",
	}
	if len(got) != len(want) {
		t.Fatalf("member count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("member[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseZpoolStatusMembersSkipsPoolAndVdevKeywords(t *testing.T) {
	output := `config:

	NAME        STATE     READ WRITE CKSUM
	rpool       ONLINE       0     0     0
	  raidz2-0  ONLINE       0     0     0
	    sda     ONLINE       0     0     0
	    sdb     ONLINE       0     0     0
	    sdc     ONLINE       0     0     0
	  spares
	    sdd     AVAIL
errors: No known data errors
`
	got := parseZpoolStatusMembers("rpool", output)
	want := []string{"sda", "sdb", "sdc", "sdd"}
	if len(got) != len(want) {
		t.Fatalf("member count: got %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("member[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeZFSMemberKeysCoverage(t *testing.T) {
	got := normalizeZFSMemberKeys("/dev/disk/by-id/ata-Samsung_SSD_870_EVO_1TB_S5Y2NX0R500001Z-part3")
	expected := []string{
		"/dev/disk/by-id/ata-samsung_ssd_870_evo_1tb_s5y2nx0r500001z-part3",
		"ata-samsung_ssd_870_evo_1tb_s5y2nx0r500001z-part3",
		"ata-samsung_ssd_870_evo_1tb_s5y2nx0r500001z",
		"s5y2nx0r500001z",
	}
	gotSet := map[string]struct{}{}
	for _, k := range got {
		gotSet[k] = struct{}{}
	}
	for _, want := range expected {
		if _, ok := gotSet[want]; !ok {
			t.Fatalf("missing key %q in %v", want, got)
		}
	}
}

func TestStripZFSPartitionSuffix(t *testing.T) {
	cases := map[string]string{
		"sda":                  "sda",
		"sda3":                 "sda",
		"nvme0n1":              "nvme0n1",
		"nvme0n1p1":            "nvme0n1",
		"nvme10n1p3":           "nvme10n1",
		"ata-Foo_SERIAL-part3": "ata-Foo_SERIAL",
		"":                     "",
	}
	for in, want := range cases {
		if got := stripZFSPartitionSuffix(in); got != want {
			t.Fatalf("stripZFSPartitionSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}
