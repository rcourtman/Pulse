package proxmox

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestVMFilesystemMetadataRejectsNoncanonicalFields(t *testing.T) {
	fields := []struct{ name, value, replacement string }{
		{"name", `"/dev/vdb1"`, `"C:\\"`},
		{"type", `"ext4"`, `"tmpfs"`},
		{"mountpoint", `"/data"`, `"/proc"`},
		{"disk", `[{"dev":"/dev/vdb1"}]`, `[{"dev":"/dev/vda1"}]`},
	}
	for _, field := range fields {
		for _, alias := range []string{strings.ToUpper(field.name), strings.ToUpper(field.name[:1]) + field.name[1:]} {
			t.Run(field.name+"/"+alias, func(t *testing.T) {
				base := `"total-bytes":9000,"used-bytes":8820`
				for _, peer := range fields {
					if peer.name != field.name {
						base += fmt.Sprintf(",%q:%s", peer.name, peer.value)
					}
				}
				canonical := fmt.Sprintf("%q:%s", field.name, field.value)
				variant := fmt.Sprintf("%q:%s", alias, field.replacement)
				for name, suffix := range map[string]string{
					"alias-only":      variant,
					"canonical-first": canonical + "," + variant,
					"canonical-last":  variant + "," + canonical,
				} {
					t.Run(name, func(t *testing.T) {
						var fs VMFileSystem
						if err := json.Unmarshal([]byte("{"+base+","+suffix+"}"), &fs); err == nil {
							t.Fatalf("case-variant metadata became a complete filesystem reading: %+v", fs)
						}
					})
				}
			})
		}
	}
}

func TestVMFilesystemMetadataPreservesCanonicalCompatibility(t *testing.T) {
	for name, payload := range map[string]string{
		"canonical": `{"name":"/dev/vdb1","type":"ext4","mountpoint":"/data","disk":[{"dev":"/dev/vdb1"}],"total-bytes":9000,"used-bytes":0}`,
		// JSON escapes decode to the exact canonical spelling, not an alias.
		"escaped-canonical": `{"\u006eame":"/dev/vdb1","\u0074ype":"ext4","\u006dountpoint":"/data","\u0064isk":[{"dev":"/dev/vdb1"}],"total-bytes":"9e3","used-bytes":0}`,
		"additive-metadata": `{"name":"/dev/vdb1","type":"ext4","mountpoint":"/data","disk":[{"dev":"/dev/vdb1"}],"total-bytes":9000,"used-bytes":0,"future-field":{"TYPE":"ignored"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var fs VMFileSystem
			if err := json.Unmarshal([]byte(payload), &fs); err != nil {
				t.Fatal(err)
			}
			if fs.Name != "/dev/vdb1" || fs.Type != "ext4" || fs.Mountpoint != "/data" || len(fs.DiskRaw) != 1 || fs.TotalBytes != 9000 || fs.UsedBytes != 0 {
				t.Fatalf("canonical identity or measured zero changed: %+v", fs)
			}
		})
	}
}
