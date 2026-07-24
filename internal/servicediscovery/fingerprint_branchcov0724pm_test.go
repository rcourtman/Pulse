package servicediscovery

import (
	"testing"
)

// TestBranchcov0724pmGenerateDockerFingerprint covers the loop bodies for
// Ports, Mounts, and Labels (fingerprint.go:26-42) that the existing suite
// never reaches, plus stability and sensitivity assertions.
func TestBranchcov0724pmGenerateDockerFingerprint(t *testing.T) {
	// Fully-populated container — exercises the Ports, Mounts, and Labels
	// range bodies (the uncovered arms at 75.0%).
	full := &DockerContainer{
		Name:   "web",
		Image:  "nginx:1.25",
		Ports:  []DockerPort{{PrivatePort: 8080, Protocol: "tcp"}, {PrivatePort: 443, Protocol: "tcp"}},
		Mounts: []DockerMount{{Source: "/host/z", Destination: "/z"}, {Source: "/host/a", Destination: "/a"}},
		Labels: map[string]string{"zoo": "1", "alpha": "2"},
	}

	t.Run("populated-extracts-and-sorts", func(t *testing.T) {
		fp := GenerateDockerFingerprint("host1", full)
		if fp.ResourceID != "web" {
			t.Fatalf("ResourceID = %q, want %q", fp.ResourceID, "web")
		}
		if fp.TargetID != "host1" {
			t.Fatalf("TargetID = %q, want %q", fp.TargetID, "host1")
		}
		if fp.ImageName != "nginx:1.25" {
			t.Fatalf("ImageName = %q, want %q", fp.ImageName, "nginx:1.25")
		}
		if fp.SchemaVersion != FingerprintSchemaVersion {
			t.Fatalf("SchemaVersion = %d, want %d", fp.SchemaVersion, FingerprintSchemaVersion)
		}
		// Ports formatted then sorted: input [8080, 443] -> ["8080/tcp","443/tcp"] -> sorted ["443/tcp","8080/tcp"]
		wantPorts := []string{"443/tcp", "8080/tcp"}
		if len(fp.Ports) != len(wantPorts) {
			t.Fatalf("Ports = %v, want %v", fp.Ports, wantPorts)
		}
		for i, w := range wantPorts {
			if fp.Ports[i] != w {
				t.Fatalf("Ports[%d] = %q, want %q (full: %v)", i, fp.Ports[i], w, fp.Ports)
			}
		}
		// Mount destinations sorted: input ["/z","/a"] -> sorted ["/a","/z"]
		wantMounts := []string{"/a", "/z"}
		if len(fp.MountPaths) != len(wantMounts) {
			t.Fatalf("MountPaths = %v, want %v", fp.MountPaths, wantMounts)
		}
		for i, w := range wantMounts {
			if fp.MountPaths[i] != w {
				t.Fatalf("MountPaths[%d] = %q, want %q", i, fp.MountPaths[i], w)
			}
		}
		// Label keys sorted: map keys ["zoo","alpha"] -> sorted ["alpha","zoo"]
		wantEnvKeys := []string{"alpha", "zoo"}
		if len(fp.EnvKeys) != len(wantEnvKeys) {
			t.Fatalf("EnvKeys = %v, want %v", fp.EnvKeys, wantEnvKeys)
		}
		for i, w := range wantEnvKeys {
			if fp.EnvKeys[i] != w {
				t.Fatalf("EnvKeys[%d] = %q, want %q (full: %v)", i, fp.EnvKeys[i], w, fp.EnvKeys)
			}
		}
		if len(fp.Hash) != 16 {
			t.Fatalf("Hash length = %d, want 16 (truncated SHA256)", len(fp.Hash))
		}
	})

	t.Run("empty-optional-fields", func(t *testing.T) {
		bare := &DockerContainer{Name: "bare", Image: "alpine:latest"}
		fp := GenerateDockerFingerprint("node", bare)
		if len(fp.Ports) != 0 {
			t.Fatalf("Ports = %v, want empty", fp.Ports)
		}
		if len(fp.MountPaths) != 0 {
			t.Fatalf("MountPaths = %v, want empty", fp.MountPaths)
		}
		if len(fp.EnvKeys) != 0 {
			t.Fatalf("EnvKeys = %v, want empty", fp.EnvKeys)
		}
		if fp.Hash == "" {
			t.Fatalf("Hash must be non-empty even with no optional fields")
		}
	})

	t.Run("stability-same-input-same-hash", func(t *testing.T) {
		fp1 := GenerateDockerFingerprint("h", full)
		fp2 := GenerateDockerFingerprint("h", full)
		if fp1.Hash != fp2.Hash {
			t.Fatalf("same input must produce same hash: %q vs %q", fp1.Hash, fp2.Hash)
		}
	})

	t.Run("sensitivity-image-change", func(t *testing.T) {
		base := GenerateDockerFingerprint("host1", full)
		modified := *full
		modified.Image = "nginx:1.26"
		changed := GenerateDockerFingerprint("host1", &modified)
		if base.Hash == changed.Hash {
			t.Fatalf("changing Image must change hash (both = %q)", base.Hash)
		}
	})

	t.Run("sensitivity-port-change", func(t *testing.T) {
		base := GenerateDockerFingerprint("host1", full)
		modified := *full
		modified.Ports = []DockerPort{{PrivatePort: 9090, Protocol: "tcp"}, {PrivatePort: 443, Protocol: "tcp"}}
		changed := GenerateDockerFingerprint("host1", &modified)
		if base.Hash == changed.Hash {
			t.Fatalf("changing Ports must change hash (both = %q)", base.Hash)
		}
	})
}

// TestBranchcov0724pmHasSchemaChanged covers the nil-other arm
// (fingerprint.go:76-78) that the existing suite never reaches, plus both
// verdicts.
func TestBranchcov0724pmHasSchemaChanged(t *testing.T) {
	fp := &ContainerFingerprint{SchemaVersion: FingerprintSchemaVersion}

	t.Run("nil-other-returns-false", func(t *testing.T) {
		if fp.HasSchemaChanged(nil) {
			t.Fatalf("HasSchemaChanged(nil) must return false")
		}
	})

	t.Run("same-schema-returns-false", func(t *testing.T) {
		other := &ContainerFingerprint{SchemaVersion: FingerprintSchemaVersion}
		if fp.HasSchemaChanged(other) {
			t.Fatalf("HasSchemaChanged with same schema must return false")
		}
	})

	t.Run("different-schema-returns-true", func(t *testing.T) {
		other := &ContainerFingerprint{SchemaVersion: FingerprintSchemaVersion + 1}
		if !fp.HasSchemaChanged(other) {
			t.Fatalf("HasSchemaChanged with different schema must return true")
		}
	})
}

// TestBranchcov0724pmGenerateLXCFingerprint covers the IsOCI, Template, and
// Tags arms (fingerprint.go:114-131) that the existing suite never reaches,
// plus stability, sensitivity, and the intentional IP-address exclusion.
func TestBranchcov0724pmGenerateLXCFingerprint(t *testing.T) {
	base := &Container{
		VMID:       101,
		Name:       "lxc1",
		OSTemplate: "debian-12-standard",
		OSName:     "debian",
		CPUs:       2,
		MaxMemory:  4096,
		MaxDisk:    8,
	}

	t.Run("identity-fields-populated", func(t *testing.T) {
		fp := GenerateLXCFingerprint("node1", base)
		if fp.ResourceID != "101" {
			t.Fatalf("ResourceID = %q, want %q", fp.ResourceID, "101")
		}
		if fp.TargetID != "node1" {
			t.Fatalf("TargetID = %q, want %q", fp.TargetID, "node1")
		}
		if fp.ImageName != "debian-12-standard" {
			t.Fatalf("ImageName = %q, want %q", fp.ImageName, "debian-12-standard")
		}
		if fp.SchemaVersion != FingerprintSchemaVersion {
			t.Fatalf("SchemaVersion = %d, want %d", fp.SchemaVersion, FingerprintSchemaVersion)
		}
	})

	t.Run("stability-same-input-same-hash", func(t *testing.T) {
		fp1 := GenerateLXCFingerprint("node1", base)
		fp2 := GenerateLXCFingerprint("node1", base)
		if fp1.Hash != fp2.Hash {
			t.Fatalf("same input must produce same hash: %q vs %q", fp1.Hash, fp2.Hash)
		}
	})

	t.Run("oci-flag-changes-hash", func(t *testing.T) {
		baseFP := GenerateLXCFingerprint("node1", base)
		oci := *base
		oci.IsOCI = true
		ociFP := GenerateLXCFingerprint("node1", &oci)
		if baseFP.Hash == ociFP.Hash {
			t.Fatalf("setting IsOCI must change hash (both = %q)", baseFP.Hash)
		}
	})

	t.Run("template-flag-changes-hash", func(t *testing.T) {
		baseFP := GenerateLXCFingerprint("node1", base)
		tmpl := *base
		tmpl.Template = true
		tmplFP := GenerateLXCFingerprint("node1", &tmpl)
		if baseFP.Hash == tmplFP.Hash {
			t.Fatalf("setting Template must change hash (both = %q)", baseFP.Hash)
		}
	})

	t.Run("tags-change-hash-and-order-independent", func(t *testing.T) {
		baseFP := GenerateLXCFingerprint("node1", base)
		tagged := *base
		tagged.Tags = []string{"web", "api"}
		taggedFP := GenerateLXCFingerprint("node1", &tagged)
		if baseFP.Hash == taggedFP.Hash {
			t.Fatalf("adding Tags must change hash (both = %q)", baseFP.Hash)
		}
		reordered := *base
		reordered.Tags = []string{"api", "web"}
		reorderedFP := GenerateLXCFingerprint("node1", &reordered)
		if taggedFP.Hash != reorderedFP.Hash {
			t.Fatalf("different tag order must produce same hash: %q vs %q", taggedFP.Hash, reorderedFP.Hash)
		}
	})

	t.Run("ip-addresses-excluded-from-hash", func(t *testing.T) {
		baseFP := GenerateLXCFingerprint("node1", base)
		withIPs := *base
		withIPs.IPAddresses = []string{"10.0.0.5", "192.168.1.10"}
		ipFP := GenerateLXCFingerprint("node1", &withIPs)
		if baseFP.Hash != ipFP.Hash {
			t.Fatalf("IP addresses must be excluded from hash: base=%q withIPs=%q", baseFP.Hash, ipFP.Hash)
		}
	})

	t.Run("empty-input-does-not-panic", func(t *testing.T) {
		fp := GenerateLXCFingerprint("node", &Container{})
		if fp.Hash == "" {
			t.Fatalf("Hash must be non-empty for zero-value Container")
		}
		if fp.ResourceID != "0" {
			t.Fatalf("ResourceID = %q, want %q for VMID=0", fp.ResourceID, "0")
		}
	})

	t.Run("vmid-change-changes-hash", func(t *testing.T) {
		baseFP := GenerateLXCFingerprint("node1", base)
		modified := *base
		modified.VMID = 102
		modifiedFP := GenerateLXCFingerprint("node1", &modified)
		if baseFP.Hash == modifiedFP.Hash {
			t.Fatalf("changing VMID must change hash (both = %q)", baseFP.Hash)
		}
	})
}

// TestBranchcov0724pmGenerateVMFingerprint covers the Template and Tags arms
// (fingerprint.go:167-179) that the existing suite never reaches, plus
// stability, sensitivity, and the intentional IP-address exclusion.
func TestBranchcov0724pmGenerateVMFingerprint(t *testing.T) {
	base := &VM{
		VMID:      201,
		Name:      "vm1",
		OSName:    "ubuntu",
		OSVersion: "22.04",
		CPUs:      4,
		MaxMemory: 8192,
		MaxDisk:   32,
	}

	t.Run("identity-fields-populated", func(t *testing.T) {
		fp := GenerateVMFingerprint("node1", base)
		if fp.ResourceID != "201" {
			t.Fatalf("ResourceID = %q, want %q", fp.ResourceID, "201")
		}
		if fp.TargetID != "node1" {
			t.Fatalf("TargetID = %q, want %q", fp.TargetID, "node1")
		}
		if fp.ImageName != "ubuntu" {
			t.Fatalf("ImageName = %q, want %q (OSName)", fp.ImageName, "ubuntu")
		}
	})

	t.Run("stability-same-input-same-hash", func(t *testing.T) {
		fp1 := GenerateVMFingerprint("node1", base)
		fp2 := GenerateVMFingerprint("node1", base)
		if fp1.Hash != fp2.Hash {
			t.Fatalf("same input must produce same hash: %q vs %q", fp1.Hash, fp2.Hash)
		}
	})

	t.Run("template-flag-changes-hash", func(t *testing.T) {
		baseFP := GenerateVMFingerprint("node1", base)
		tmpl := *base
		tmpl.Template = true
		tmplFP := GenerateVMFingerprint("node1", &tmpl)
		if baseFP.Hash == tmplFP.Hash {
			t.Fatalf("setting Template must change hash (both = %q)", baseFP.Hash)
		}
	})

	t.Run("tags-change-hash-and-order-independent", func(t *testing.T) {
		baseFP := GenerateVMFingerprint("node1", base)
		tagged := *base
		tagged.Tags = []string{"prod", "db"}
		taggedFP := GenerateVMFingerprint("node1", &tagged)
		if baseFP.Hash == taggedFP.Hash {
			t.Fatalf("adding Tags must change hash (both = %q)", baseFP.Hash)
		}
		reordered := *base
		reordered.Tags = []string{"db", "prod"}
		reorderedFP := GenerateVMFingerprint("node1", &reordered)
		if taggedFP.Hash != reorderedFP.Hash {
			t.Fatalf("different tag order must produce same hash: %q vs %q", taggedFP.Hash, reorderedFP.Hash)
		}
	})

	t.Run("ip-addresses-excluded-from-hash", func(t *testing.T) {
		baseFP := GenerateVMFingerprint("node1", base)
		withIPs := *base
		withIPs.IPAddresses = []string{"10.0.0.5"}
		ipFP := GenerateVMFingerprint("node1", &withIPs)
		if baseFP.Hash != ipFP.Hash {
			t.Fatalf("IP addresses must be excluded from hash: base=%q withIPs=%q", baseFP.Hash, ipFP.Hash)
		}
	})

	t.Run("empty-input-does-not-panic", func(t *testing.T) {
		fp := GenerateVMFingerprint("node", &VM{})
		if fp.Hash == "" {
			t.Fatalf("Hash must be non-empty for zero-value VM")
		}
		if fp.ResourceID != "0" {
			t.Fatalf("ResourceID = %q, want %q for VMID=0", fp.ResourceID, "0")
		}
	})

	t.Run("osversion-change-changes-hash", func(t *testing.T) {
		baseFP := GenerateVMFingerprint("node1", base)
		modified := *base
		modified.OSVersion = "24.04"
		modifiedFP := GenerateVMFingerprint("node1", &modified)
		if baseFP.Hash == modifiedFP.Hash {
			t.Fatalf("changing OSVersion must change hash (both = %q)", baseFP.Hash)
		}
	})
}

// TestBranchcov0724pmGenerateHostFingerprint covers the Tags arm
// (fingerprint.go:217-222) that the existing suite never reaches, plus
// stability, sensitivity, and the intentional Status exclusion.
func TestBranchcov0724pmGenerateHostFingerprint(t *testing.T) {
	base := &Host{
		ID:            "agent-1",
		Hostname:      "server1",
		Platform:      "linux",
		OSName:        "Ubuntu",
		OSVersion:     "22.04",
		KernelVersion: "6.1.0",
		Architecture:  "amd64",
		CPUCount:      8,
	}

	t.Run("identity-fields-populated", func(t *testing.T) {
		fp := GenerateHostFingerprint(base)
		if fp.ResourceID != "agent-1" {
			t.Fatalf("ResourceID = %q, want %q", fp.ResourceID, "agent-1")
		}
		if fp.TargetID != "agent-1" {
			t.Fatalf("TargetID = %q, want %q (same as ID for hosts)", fp.TargetID, "agent-1")
		}
		if fp.ImageName != "Ubuntu" {
			t.Fatalf("ImageName = %q, want %q (OSName)", fp.ImageName, "Ubuntu")
		}
	})

	t.Run("stability-same-input-same-hash", func(t *testing.T) {
		fp1 := GenerateHostFingerprint(base)
		fp2 := GenerateHostFingerprint(base)
		if fp1.Hash != fp2.Hash {
			t.Fatalf("same input must produce same hash: %q vs %q", fp1.Hash, fp2.Hash)
		}
	})

	t.Run("tags-change-hash-and-order-independent", func(t *testing.T) {
		baseFP := GenerateHostFingerprint(base)
		tagged := *base
		tagged.Tags = []string{"rack-a", "prod"}
		taggedFP := GenerateHostFingerprint(&tagged)
		if baseFP.Hash == taggedFP.Hash {
			t.Fatalf("adding Tags must change hash (both = %q)", baseFP.Hash)
		}
		reordered := *base
		reordered.Tags = []string{"prod", "rack-a"}
		reorderedFP := GenerateHostFingerprint(&reordered)
		if taggedFP.Hash != reorderedFP.Hash {
			t.Fatalf("different tag order must produce same hash: %q vs %q", taggedFP.Hash, reorderedFP.Hash)
		}
	})

	t.Run("status-excluded-from-hash", func(t *testing.T) {
		baseFP := GenerateHostFingerprint(base)
		modified := *base
		modified.Status = "offline"
		modifiedFP := GenerateHostFingerprint(&modified)
		if baseFP.Hash != modifiedFP.Hash {
			t.Fatalf("Status must be excluded from hash: base=%q modified=%q", baseFP.Hash, modifiedFP.Hash)
		}
	})

	t.Run("empty-input-does-not-panic", func(t *testing.T) {
		fp := GenerateHostFingerprint(&Host{})
		if fp.Hash == "" {
			t.Fatalf("Hash must be non-empty for zero-value Host")
		}
		if fp.ResourceID != "" {
			t.Fatalf("ResourceID = %q, want empty for zero-value Host", fp.ResourceID)
		}
	})

	t.Run("hostname-change-changes-hash", func(t *testing.T) {
		baseFP := GenerateHostFingerprint(base)
		modified := *base
		modified.Hostname = "server2"
		modifiedFP := GenerateHostFingerprint(&modified)
		if baseFP.Hash == modifiedFP.Hash {
			t.Fatalf("changing Hostname must change hash (both = %q)", baseFP.Hash)
		}
	})

	t.Run("cpucount-change-changes-hash", func(t *testing.T) {
		baseFP := GenerateHostFingerprint(base)
		modified := *base
		modified.CPUCount = 16
		modifiedFP := GenerateHostFingerprint(&modified)
		if baseFP.Hash == modifiedFP.Hash {
			t.Fatalf("changing CPUCount must change hash (both = %q)", baseFP.Hash)
		}
	})
}
