package servicediscovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GenerateDockerFingerprint creates a fingerprint from Docker container metadata.
// The fingerprint captures key metadata that indicates when a container has changed
// in ways that would affect discovery results (image, ports, mounts, env keys).
func GenerateDockerFingerprint(hostID string, container *DockerContainer) *ContainerFingerprint {
	fp := &ContainerFingerprint{
		ResourceID:    container.Name,
		HostID:        hostID,
		SchemaVersion: FingerprintSchemaVersion,
		GeneratedAt:   time.Now(),
		ImageName:     container.Image,
	}

	// Extract port mappings (private port + protocol)
	for _, p := range container.Ports {
		fp.Ports = append(fp.Ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Protocol))
	}
	sort.Strings(fp.Ports)

	// Extract mount paths (container destination paths, not host paths)
	for _, m := range container.Mounts {
		fp.MountPaths = append(fp.MountPaths, m.Destination)
	}
	sort.Strings(fp.MountPaths)

	// Extract environment variable keys from labels (if present)
	// Note: We don't have direct access to env vars in DockerContainer,
	// but labels often contain relevant configuration hints
	for key := range container.Labels {
		fp.EnvKeys = append(fp.EnvKeys, key)
	}
	sort.Strings(fp.EnvKeys)

	// Generate the hash
	fp.Hash = fp.computeHash()
	return fp
}

// computeHash generates a truncated SHA256 hash of the fingerprint components.
// Includes schema version so algorithm changes produce different hashes.
func (fp *ContainerFingerprint) computeHash() string {
	h := sha256.New()
	// Include schema version first so algorithm changes are detected
	h.Write([]byte(strconv.Itoa(fp.SchemaVersion)))
	h.Write([]byte(fp.ImageID))
	h.Write([]byte(fp.ImageName))
	h.Write([]byte(fp.CreatedAt))
	h.Write([]byte(strings.Join(fp.Ports, ",")))
	h.Write([]byte(strings.Join(fp.MountPaths, ",")))
	h.Write([]byte(strings.Join(fp.EnvKeys, ",")))
	return hex.EncodeToString(h.Sum(nil))[:16] // Short hash is sufficient
}

// HasChanged compares two fingerprints and returns true if they differ.
// Also returns true if the schema version changed (algorithm updated).
func (fp *ContainerFingerprint) HasChanged(other *ContainerFingerprint) bool {
	if other == nil {
		return true
	}
	return fp.Hash != other.Hash
}

// HasSchemaChanged returns true if the fingerprint was generated with a different schema.
func (fp *ContainerFingerprint) HasSchemaChanged(other *ContainerFingerprint) bool {
	if other == nil {
		return false
	}
	return fp.SchemaVersion != other.SchemaVersion
}

// String returns a human-readable representation of the fingerprint.
func (fp *ContainerFingerprint) String() string {
	return fmt.Sprintf("Fingerprint{id=%s, host=%s, hash=%s, image=%s, ports=%v}",
		fp.ResourceID, fp.HostID, fp.Hash, fp.ImageName, fp.Ports)
}

// GenerateLXCFingerprint creates a fingerprint from LXC container metadata.
// Tracks: VMID, name, OS template, resource allocation, and tags.
func GenerateLXCFingerprint(nodeID string, container *Container) *ContainerFingerprint {
	fp := &ContainerFingerprint{
		ResourceID:    strconv.Itoa(container.VMID),
		HostID:        nodeID,
		SchemaVersion: FingerprintSchemaVersion,
		GeneratedAt:   time.Now(),
		ImageName:     container.OSTemplate, // OS template is like the "image" for LXCs
	}

	// Build components for hashing
	var components []string

	// Core identity
	components = append(components, strconv.Itoa(container.VMID))
	components = append(components, container.Name)
	components = append(components, container.OSTemplate)
	components = append(components, container.OSName)

	// Resource allocation (changes here might affect what's running)
	components = append(components, strconv.Itoa(container.CPUs))
	components = append(components, strconv.FormatUint(container.MaxMemory, 10))
	components = append(components, strconv.FormatUint(container.MaxDisk, 10))

	// OCI container flag (different container type)
	if container.IsOCI {
		components = append(components, "oci:true")
	}

	// Template flag (templates shouldn't trigger discovery)
	if container.Template {
		components = append(components, "template:true")
	}

	// Note: IP addresses intentionally excluded - DHCP churn causes false positives

	// Tags (user might tag based on what's running)
	if len(container.Tags) > 0 {
		sortedTags := make([]string, len(container.Tags))
		copy(sortedTags, container.Tags)
		sort.Strings(sortedTags)
		components = append(components, sortedTags...)
	}

	// Generate hash
	h := sha256.New()
	h.Write([]byte(strings.Join(components, "|")))
	fp.Hash = hex.EncodeToString(h.Sum(nil))[:16]

	return fp
}

// GenerateVMFingerprint creates a fingerprint from VM metadata.
// Tracks: VMID, name, OS, resource allocation, and tags.
func GenerateVMFingerprint(nodeID string, vm *VM) *ContainerFingerprint {
	fp := &ContainerFingerprint{
		ResourceID:    strconv.Itoa(vm.VMID),
		HostID:        nodeID,
		SchemaVersion: FingerprintSchemaVersion,
		GeneratedAt:   time.Now(),
		ImageName:     vm.OSName, // OS name is the closest to an "image" for VMs
	}

	// Build components for hashing
	var components []string

	// Core identity
	components = append(components, strconv.Itoa(vm.VMID))
	components = append(components, vm.Name)
	components = append(components, vm.OSName)
	components = append(components, vm.OSVersion)

	// Resource allocation
	components = append(components, strconv.Itoa(vm.CPUs))
	components = append(components, strconv.FormatUint(vm.MaxMemory, 10))
	components = append(components, strconv.FormatUint(vm.MaxDisk, 10))

	// Template flag (templates shouldn't trigger discovery)
	if vm.Template {
		components = append(components, "template:true")
	}

	// Note: IP addresses intentionally excluded - DHCP churn causes false positives

	// Tags
	if len(vm.Tags) > 0 {
		sortedTags := make([]string, len(vm.Tags))
		copy(sortedTags, vm.Tags)
		sort.Strings(sortedTags)
		components = append(components, sortedTags...)
	}

	// Generate hash
	h := sha256.New()
	h.Write([]byte(strings.Join(components, "|")))
	fp.Hash = hex.EncodeToString(h.Sum(nil))[:16]

	return fp
}

// GenerateK8sPodFingerprint creates a fingerprint from Kubernetes pod metadata.
// Tracks: UID, name, namespace, labels, owner (deployment/statefulset/etc), and container images.
func GenerateK8sPodFingerprint(clusterID string, pod *KubernetesPod) *ContainerFingerprint {
	fp := &ContainerFingerprint{
		ResourceID:    pod.UID,
		HostID:        clusterID,
		SchemaVersion: FingerprintSchemaVersion,
		GeneratedAt:   time.Now(),
	}

	// Build components for hashing
	var components []string

	// Core identity
	components = append(components, pod.UID)
	components = append(components, pod.Name)
	components = append(components, pod.Namespace)
	components = append(components, pod.NodeName)

	// Owner reference (deployment, statefulset, daemonset, etc.)
	if pod.OwnerKind != "" {
		components = append(components, "owner:"+pod.OwnerKind+"/"+pod.OwnerName)
	}

	// Container images (most important for detecting app changes)
	var images []string
	for _, c := range pod.Containers {
		images = append(images, c.Name+":"+c.Image)
	}
	sort.Strings(images)
	if len(images) > 0 {
		fp.ImageName = images[0] // Use first container image as the "image name"
		components = append(components, "images:"+strings.Join(images, ","))
	}

	// Labels (sorted by key for consistency)
	if len(pod.Labels) > 0 {
		var labelKeys []string
		for k := range pod.Labels {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)
		var labelPairs []string
		for _, k := range labelKeys {
			labelPairs = append(labelPairs, k+"="+pod.Labels[k])
		}
		components = append(components, "labels:"+strings.Join(labelPairs, ","))
	}

	// Generate hash
	h := sha256.New()
	h.Write([]byte(strings.Join(components, "|")))
	fp.Hash = hex.EncodeToString(h.Sum(nil))[:16]

	return fp
}
