package models

import (
	"reflect"
	"testing"
	"time"
)

// This file adds deep-copy INDEPENDENCE coverage for the populated (non-nil)
// loop bodies of the clone* helpers in deepcopy.go. Each populated case:
//  1. builds a source whose nested reference fields (sub-slices, maps, ptrs)
//     are non-nil and populated,
//  2. clones it,
//  3. asserts value equality via reflect.DeepEqual,
//  4. MUTATES a nested field of the ORIGINAL (element index / map entry /
//     pointed-to value) and asserts the CLONE is UNCHANGED, which proves the
//     clone owns distinct backing arrays (a shallow copy would alias and leak).
//
// Element-index mutation is used for slices because it deterministically
// detects shared backing arrays (unlike append, which may reallocate).

func w0716dcFloat64(v float64) *float64     { return &v }
func w0716dcInt64(v int64) *int64           { return &v }
func w0716dcTimePtr(t time.Time) *time.Time { return &t }

// w0716dcT is a stable timestamp reused across these tests.
var w0716dcT = time.Date(2024, 5, 17, 12, 30, 0, 0, time.UTC)

func Test_w0716_dc_DockerImages(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerImages(nil); got != nil {
			t.Fatalf("cloneDockerImages(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerImages([]DockerImage{}); got != nil {
			t.Fatalf("cloneDockerImages(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		src := []DockerImage{{
			ID:          "img-1",
			RepoTags:    []string{"repo:latest", "repo:v1"},
			RepoDigests: []string{"sha256:abc"},
			Labels:      map[string]string{"app": "web"},
			SizeBytes:   1024,
		}}
		clone := cloneDockerImages(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].RepoTags[0] = "MUTATED"
		src[0].RepoDigests[0] = "MUTATED"
		src[0].Labels["app"] = "MUTATED"
		src[0].Labels["leak"] = "x"

		if clone[0].RepoTags[0] != "repo:latest" {
			t.Errorf("RepoTags not isolated: got %q", clone[0].RepoTags[0])
		}
		if clone[0].RepoDigests[0] != "sha256:abc" {
			t.Errorf("RepoDigests not isolated: got %q", clone[0].RepoDigests[0])
		}
		if clone[0].Labels["app"] != "web" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["app"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
	})
}

func Test_w0716_dc_DockerVolumes(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerVolumes(nil); got != nil {
			t.Fatalf("cloneDockerVolumes(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerVolumes([]DockerVolume{}); got != nil {
			t.Fatalf("cloneDockerVolumes(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		src := []DockerVolume{{
			Name:    "vol-1",
			Driver:  "local",
			Labels:  map[string]string{"env": "prod"},
			Options: map[string]string{"device": "/dev/sda1"},
		}}
		clone := cloneDockerVolumes(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Labels["env"] = "MUTATED"
		src[0].Labels["leak"] = "x"
		src[0].Options["device"] = "MUTATED"
		src[0].Options["leak"] = "x"

		if clone[0].Labels["env"] != "prod" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["env"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
		if clone[0].Options["device"] != "/dev/sda1" {
			t.Errorf("Options not isolated: got %q", clone[0].Options["device"])
		}
		if _, leak := clone[0].Options["leak"]; leak {
			t.Error("Options map aliases source backing map")
		}
	})
}

func Test_w0716_dc_DockerNetworks(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerNetworks(nil); got != nil {
			t.Fatalf("cloneDockerNetworks(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerNetworks([]DockerNetwork{}); got != nil {
			t.Fatalf("cloneDockerNetworks(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		src := []DockerNetwork{{
			ID:      "net-1",
			Name:    "bridge-net",
			Subnets: []DockerNetworkSubnet{{Subnet: "10.0.0.0/24", Gateway: "10.0.0.1"}},
			Labels:  map[string]string{"scope": "local"},
			Options: map[string]string{"com.docker.network.bridge.enable_icc": "true"},
		}}
		clone := cloneDockerNetworks(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Subnets[0].Subnet = "MUTATED"
		src[0].Labels["scope"] = "MUTATED"
		src[0].Labels["leak"] = "x"
		src[0].Options["leak"] = "x"

		if clone[0].Subnets[0].Subnet != "10.0.0.0/24" {
			t.Errorf("Subnets not isolated: got %q", clone[0].Subnets[0].Subnet)
		}
		if clone[0].Labels["scope"] != "local" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["scope"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
		if _, leak := clone[0].Options["leak"]; leak {
			t.Error("Options map aliases source backing map")
		}
	})
}

func Test_w0716_dc_DockerTasks(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerTasks(nil); got != nil {
			t.Fatalf("cloneDockerTasks(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerTasks([]DockerTask{}); got != nil {
			t.Fatalf("cloneDockerTasks(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		// startVal/completedVal are the pointed-to variables; they get
		// clobbered by the mutation step below, so assertions compare against
		// the immutable package var w0716dcT (and a fresh .Add), never against
		// startVal/completedVal.
		startVal := w0716dcT
		completedVal := w0716dcT.Add(time.Hour)
		src := []DockerTask{{
			ID:          "task-1",
			ServiceName: "svc",
			UpdatedAt:   &startVal,
			StartedAt:   &startVal,
			CompletedAt: &completedVal,
		}}
		clone := cloneDockerTasks(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		// Mutate the pointed-to time values of the ORIGINAL; the clone must
		// own distinct *time.Time allocations.
		mutated := time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
		*src[0].UpdatedAt = mutated
		*src[0].StartedAt = mutated
		*src[0].CompletedAt = mutated

		if !clone[0].UpdatedAt.Equal(w0716dcT) {
			t.Errorf("UpdatedAt not isolated: got %v", clone[0].UpdatedAt)
		}
		if !clone[0].StartedAt.Equal(w0716dcT) {
			t.Errorf("StartedAt not isolated: got %v", clone[0].StartedAt)
		}
		if !clone[0].CompletedAt.Equal(w0716dcT.Add(time.Hour)) {
			t.Errorf("CompletedAt not isolated: got %v", clone[0].CompletedAt)
		}
	})
}

func Test_w0716_dc_DockerNodes(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerNodes(nil); got != nil {
			t.Fatalf("cloneDockerNodes(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerNodes([]DockerNode{}); got != nil {
			t.Fatalf("cloneDockerNodes(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		updated := w0716dcT
		src := []DockerNode{{
			ID:           "node-1",
			Hostname:     "worker-1",
			Labels:       map[string]string{"region": "us"},
			EngineLabels: map[string]string{"foo": "bar"},
			UpdatedAt:    &updated,
		}}
		clone := cloneDockerNodes(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Labels["region"] = "MUTATED"
		src[0].Labels["leak"] = "x"
		src[0].EngineLabels["foo"] = "MUTATED"
		src[0].EngineLabels["leak"] = "x"
		*src[0].UpdatedAt = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)

		if clone[0].Labels["region"] != "us" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["region"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
		if clone[0].EngineLabels["foo"] != "bar" {
			t.Errorf("EngineLabels not isolated: got %q", clone[0].EngineLabels["foo"])
		}
		if _, leak := clone[0].EngineLabels["leak"]; leak {
			t.Error("EngineLabels map aliases source backing map")
		}
		if !clone[0].UpdatedAt.Equal(w0716dcT) {
			t.Errorf("UpdatedAt not isolated: got %v", clone[0].UpdatedAt)
		}
	})
}

func Test_w0716_dc_DockerSecrets(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerSecrets(nil); got != nil {
			t.Fatalf("cloneDockerSecrets(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerSecrets([]DockerSecret{}); got != nil {
			t.Fatalf("cloneDockerSecrets(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		updated := w0716dcT
		src := []DockerSecret{{
			ID:        "sec-1",
			Name:      "db-password",
			Labels:    map[string]string{"tier": "db"},
			UpdatedAt: &updated,
		}}
		clone := cloneDockerSecrets(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Labels["tier"] = "MUTATED"
		src[0].Labels["leak"] = "x"
		*src[0].UpdatedAt = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)

		if clone[0].Labels["tier"] != "db" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["tier"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
		if !clone[0].UpdatedAt.Equal(w0716dcT) {
			t.Errorf("UpdatedAt not isolated: got %v", clone[0].UpdatedAt)
		}
	})
}

func Test_w0716_dc_DockerConfigs(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerConfigs(nil); got != nil {
			t.Fatalf("cloneDockerConfigs(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneDockerConfigs([]DockerConfig{}); got != nil {
			t.Fatalf("cloneDockerConfigs(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		updated := w0716dcT
		src := []DockerConfig{{
			ID:        "cfg-1",
			Name:      "nginx-conf",
			Labels:    map[string]string{"app": "nginx"},
			UpdatedAt: &updated,
		}}
		clone := cloneDockerConfigs(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Labels["app"] = "MUTATED"
		src[0].Labels["leak"] = "x"
		*src[0].UpdatedAt = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)

		if clone[0].Labels["app"] != "nginx" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["app"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
		if !clone[0].UpdatedAt.Equal(w0716dcT) {
			t.Errorf("UpdatedAt not isolated: got %v", clone[0].UpdatedAt)
		}
	})
}

func Test_w0716_dc_HostDiskSMART(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneHostDiskSMART(nil); got != nil {
			t.Fatalf("cloneHostDiskSMART(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneHostDiskSMART([]HostDiskSMART{}); got != nil {
			t.Fatalf("cloneHostDiskSMART(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		powerOn := int64(12345)
		media := int64(2)
		src := []HostDiskSMART{{
			Device:      "/dev/sda",
			Model:       "SSD-1",
			Temperature: 35,
			Health:      "PASSED",
			Attributes: &SMARTAttributes{
				PowerOnHours: &powerOn,
				MediaErrors:  &media,
			},
		}}
		clone := cloneHostDiskSMART(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		// Mutate the pointed-to SMART attribute values of the ORIGINAL; the
		// clone must own a distinct *SMARTAttributes (and inner pointers).
		*src[0].Attributes.PowerOnHours = 999999
		*src[0].Attributes.MediaErrors = 999999

		if *clone[0].Attributes.PowerOnHours != 12345 {
			t.Errorf("Attributes.PowerOnHours not isolated: got %d", *clone[0].Attributes.PowerOnHours)
		}
		if *clone[0].Attributes.MediaErrors != 2 {
			t.Errorf("Attributes.MediaErrors not isolated: got %d", *clone[0].Attributes.MediaErrors)
		}
	})
}

func Test_w0716_dc_HostGPUSensors(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneHostGPUSensors(nil); got != nil {
			t.Fatalf("cloneHostGPUSensors(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneHostGPUSensors([]HostGPUSensor{}); got != nil {
			t.Fatalf("cloneHostGPUSensors(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		temp := 65.0
		util := 42.0
		memUsed := int64(1024)
		memTotal := int64(8192)
		src := []HostGPUSensor{{
			ID:                 "gpu-0",
			Name:               "NVIDIA RTX",
			TemperatureCelsius: &temp,
			UtilizationPercent: &util,
			MemoryUsedBytes:    &memUsed,
			MemoryTotalBytes:   &memTotal,
		}}
		clone := cloneHostGPUSensors(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		*src[0].TemperatureCelsius = 999.0
		*src[0].UtilizationPercent = 999.0
		*src[0].MemoryUsedBytes = 999
		*src[0].MemoryTotalBytes = 999

		if *clone[0].TemperatureCelsius != 65.0 {
			t.Errorf("TemperatureCelsius not isolated: got %v", *clone[0].TemperatureCelsius)
		}
		if *clone[0].UtilizationPercent != 42.0 {
			t.Errorf("UtilizationPercent not isolated: got %v", *clone[0].UtilizationPercent)
		}
		if *clone[0].MemoryUsedBytes != 1024 {
			t.Errorf("MemoryUsedBytes not isolated: got %d", *clone[0].MemoryUsedBytes)
		}
		if *clone[0].MemoryTotalBytes != 8192 {
			t.Errorf("MemoryTotalBytes not isolated: got %d", *clone[0].MemoryTotalBytes)
		}
	})
}

func Test_w0716_dc_DockerContainerBlockIO(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneDockerContainerBlockIO(nil); got != nil {
			t.Fatalf("cloneDockerContainerBlockIO(nil) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		readRate := 1024.0
		writeRate := 2048.0
		src := &DockerContainerBlockIO{
			ReadBytes:               100,
			WriteBytes:              200,
			ReadRateBytesPerSecond:  &readRate,
			WriteRateBytesPerSecond: &writeRate,
		}
		clone := cloneDockerContainerBlockIO(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		*src.ReadRateBytesPerSecond = 999.0
		*src.WriteRateBytesPerSecond = 999.0

		if *clone.ReadRateBytesPerSecond != 1024.0 {
			t.Errorf("ReadRateBytesPerSecond not isolated: got %v", *clone.ReadRateBytesPerSecond)
		}
		if *clone.WriteRateBytesPerSecond != 2048.0 {
			t.Errorf("WriteRateBytesPerSecond not isolated: got %v", *clone.WriteRateBytesPerSecond)
		}
	})
}

// --- Additional low-coverage pure deep-copy clone funcs (<=50%) ---

func Test_w0716_dc_KubernetesPods(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneKubernetesPods(nil); got != nil {
			t.Fatalf("cloneKubernetesPods(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneKubernetesPods([]KubernetesPod{}); got != nil {
			t.Fatalf("cloneKubernetesPods(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		start := w0716dcT
		src := []KubernetesPod{{
			UID:       "pod-1",
			Name:      "web-0",
			Namespace: "default",
			StartTime: &start,
			Labels:    map[string]string{"app": "web"},
			Containers: []KubernetesPodContainer{
				{Name: "main", Image: "nginx:1.2", Ready: true},
			},
		}}
		clone := cloneKubernetesPods(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Labels["app"] = "MUTATED"
		src[0].Labels["leak"] = "x"
		src[0].Containers[0].Name = "MUTATED"
		src[0].Containers[0].Image = "MUTATED"
		*src[0].StartTime = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)

		if clone[0].Labels["app"] != "web" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["app"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
		if clone[0].Containers[0].Name != "main" {
			t.Errorf("Containers not isolated: got %q", clone[0].Containers[0].Name)
		}
		if clone[0].Containers[0].Image != "nginx:1.2" {
			t.Errorf("Containers image not isolated: got %q", clone[0].Containers[0].Image)
		}
		if !clone[0].StartTime.Equal(w0716dcT) {
			t.Errorf("StartTime not isolated: got %v", clone[0].StartTime)
		}
	})
}

func Test_w0716_dc_KubernetesDeployments(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneKubernetesDeployments(nil); got != nil {
			t.Fatalf("cloneKubernetesDeployments(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := cloneKubernetesDeployments([]KubernetesDeployment{}); got != nil {
			t.Fatalf("cloneKubernetesDeployments(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		src := []KubernetesDeployment{{
			UID:             "dep-1",
			Name:            "web",
			Namespace:       "default",
			DesiredReplicas: 3,
			Labels:          map[string]string{"app": "web"},
		}}
		clone := cloneKubernetesDeployments(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Labels["app"] = "MUTATED"
		src[0].Labels["leak"] = "x"

		if clone[0].Labels["app"] != "web" {
			t.Errorf("Labels not isolated: got %q", clone[0].Labels["app"])
		}
		if _, leak := clone[0].Labels["leak"]; leak {
			t.Error("Labels map aliases source backing map")
		}
	})
}

func Test_w0716_dc_ZFSPool(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneZFSPool(nil); got != nil {
			t.Fatalf("cloneZFSPool(nil) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		src := &ZFSPool{
			Name:       "tank",
			State:      "ONLINE",
			Status:     "Healthy",
			Devices:    []ZFSDevice{{Name: "da0", Type: "disk", State: "ONLINE"}},
			ReadErrors: 1,
		}
		clone := cloneZFSPool(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src.Devices[0].Name = "MUTATED"
		src.Devices[0].State = "MUTATED"

		if clone.Devices[0].Name != "da0" {
			t.Errorf("Devices not isolated: got %q", clone.Devices[0].Name)
		}
		if clone.Devices[0].State != "ONLINE" {
			t.Errorf("Devices state not isolated: got %q", clone.Devices[0].State)
		}
	})
}

func Test_w0716_dc_PBSDatastores(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := clonePBSDatastores(nil); got != nil {
			t.Fatalf("clonePBSDatastores(nil) = %#v, want nil", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := clonePBSDatastores([]PBSDatastore{}); got != nil {
			t.Fatalf("clonePBSDatastores(empty) = %#v, want nil", got)
		}
	})
	t.Run("populated independent deep copy", func(t *testing.T) {
		src := []PBSDatastore{{
			Name:       "store1",
			Total:      1000,
			Used:       500,
			Status:     "OK",
			Namespaces: []PBSNamespace{{Path: "ns1", Depth: 0}, {Path: "ns1/sub", Depth: 1}},
		}}
		clone := clonePBSDatastores(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		src[0].Namespaces[0].Path = "MUTATED"
		src[0].Namespaces[1].Path = "MUTATED"

		if clone[0].Namespaces[0].Path != "ns1" {
			t.Errorf("Namespaces[0] not isolated: got %q", clone[0].Namespaces[0].Path)
		}
		if clone[0].Namespaces[1].Path != "ns1/sub" {
			t.Errorf("Namespaces[1] not isolated: got %q", clone[0].Namespaces[1].Path)
		}
	})
}
