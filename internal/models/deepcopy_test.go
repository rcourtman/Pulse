package models

import (
	"testing"
)

// --- Pointer clone helpers ---

func TestCloneBoolPtr_Nil(t *testing.T) {
	if cloneBoolPtr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneBoolPtr_Value(t *testing.T) {
	v := true
	got := cloneBoolPtr(&v)
	if got == nil || *got != true {
		t.Error("value should be preserved")
	}
	v = false
	if *got != true {
		t.Error("clone should be independent of source")
	}
}

func TestCloneFloat64Ptr_Nil(t *testing.T) {
	if cloneFloat64Ptr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneFloat64Ptr_Value(t *testing.T) {
	v := 3.14
	got := cloneFloat64Ptr(&v)
	if got == nil || *got != 3.14 {
		t.Error("value should be preserved")
	}
	v = 0
	if *got != 3.14 {
		t.Error("clone should be independent of source")
	}
}

func TestCloneIntPtr_Nil(t *testing.T) {
	if cloneIntPtr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneIntPtr_Value(t *testing.T) {
	v := 42
	got := cloneIntPtr(&v)
	if got == nil || *got != 42 {
		t.Error("value should be preserved")
	}
	v = 0
	if *got != 42 {
		t.Error("clone should be independent")
	}
}

func TestCloneInt64Ptr_Nil(t *testing.T) {
	if cloneInt64Ptr(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneInt64Ptr_Value(t *testing.T) {
	v := int64(999)
	got := cloneInt64Ptr(&v)
	if got == nil || *got != 999 {
		t.Error("value should be preserved")
	}
	v = 0
	if *got != 999 {
		t.Error("clone should be independent")
	}
}

// --- Map clone helpers ---

func TestCloneStringMap_Nil(t *testing.T) {
	if cloneStringMap(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneStringMap_Isolation(t *testing.T) {
	src := map[string]string{"a": "1", "b": "2"}
	dst := cloneStringMap(src)
	dst["c"] = "3"
	if _, ok := src["c"]; ok {
		t.Error("mutating clone should not affect source")
	}
	if len(dst) != 3 {
		t.Error("clone should have the added entry")
	}
}

func TestCloneStringFloat64Map_Nil(t *testing.T) {
	if cloneStringFloat64Map(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneStringFloat64Map_Isolation(t *testing.T) {
	src := map[string]float64{"temp": 42.5}
	dst := cloneStringFloat64Map(src)
	dst["fan"] = 1200.0
	if _, ok := src["fan"]; ok {
		t.Error("mutating clone should not affect source")
	}
}

// --- Slice clone helpers ---

func TestCloneCoreTemps_Nil(t *testing.T) {
	if cloneCoreTemps(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneCoreTemps_Isolation(t *testing.T) {
	src := []CoreTemp{{Core: 0, Temp: 45.0}}
	dst := cloneCoreTemps(src)
	dst[0].Temp = 99.0
	if src[0].Temp != 45.0 {
		t.Error("mutating clone should not affect source")
	}
}

func TestCloneGPUTemps_Nil(t *testing.T) {
	if cloneGPUTemps(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneNVMeTemps_Nil(t *testing.T) {
	if cloneNVMeTemps(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneDiskTemps_Nil(t *testing.T) {
	if cloneDiskTemps(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

// --- Temperature clone ---

func TestCloneTemperature_Nil(t *testing.T) {
	if cloneTemperature(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneTemperature_Isolation(t *testing.T) {
	src := &Temperature{
		Cores: []CoreTemp{{Core: 0, Temp: 45.0}},
		NVMe:  []NVMeTemp{{Device: "nvme0", Temp: 35.0}},
	}
	dst := cloneTemperature(src)
	dst.Cores[0].Temp = 99.0
	if src.Cores[0].Temp != 45.0 {
		t.Error("mutating cloned temperature should not affect source")
	}
}

// --- Guest network interface clone ---

func TestCloneGuestNetworkInterfaces_Nil(t *testing.T) {
	if cloneGuestNetworkInterfaces(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneGuestNetworkInterfaces_Isolation(t *testing.T) {
	src := []GuestNetworkInterface{
		{Name: "eth0", Addresses: []string{"192.168.1.10"}},
	}
	dst := cloneGuestNetworkInterfaces(src)
	dst[0].Addresses = append(dst[0].Addresses, "10.0.0.1")
	if len(src[0].Addresses) != 1 {
		t.Error("mutating clone addresses should not affect source")
	}
}

// --- Host network interface clone ---

func TestCloneHostNetworkInterfaces_Nil(t *testing.T) {
	if cloneHostNetworkInterfaces(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneHostNetworkInterfaces_Isolation(t *testing.T) {
	src := []HostNetworkInterface{
		{Name: "eth0", Addresses: []string{"192.168.1.10"}, MAC: "00:11:22:33:44:55"},
	}
	dst := cloneHostNetworkInterfaces(src)
	dst[0].Addresses = append(dst[0].Addresses, "10.0.0.1")
	if len(src[0].Addresses) != 1 {
		t.Error("mutating clone addresses should not affect source")
	}
}

// --- SMART attributes clone ---

func TestCloneSMARTAttributes_Nil(t *testing.T) {
	if cloneSMARTAttributes(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneSMARTAttributes_Isolation(t *testing.T) {
	pending := int64(3)
	media := int64(1)
	src := &SMARTAttributes{
		PendingSectors: &pending,
		MediaErrors:    &media,
	}
	dst := cloneSMARTAttributes(src)
	newVal := int64(99)
	dst.PendingSectors = &newVal
	if *src.PendingSectors != 3 {
		t.Error("mutating cloned SMART attributes should not affect source")
	}
}

// --- Node clone ---

func TestCloneNode_Isolation(t *testing.T) {
	src := Node{
		ID:     "node-1",
		Name:   "pve1",
		Status: "online",
		Temperature: &Temperature{
			Cores: []CoreTemp{{Core: 0, Temp: 45.0}},
		},
	}
	dst := cloneNode(src)
	dst.Name = "changed"
	dst.Temperature.Cores[0].Temp = 99.0
	if src.Name != "pve1" {
		t.Error("clone should not affect source name")
	}
	if src.Temperature.Cores[0].Temp != 45.0 {
		t.Error("clone temperature should be independent")
	}
}

func TestCloneNodes_Nil(t *testing.T) {
	if cloneNodes(nil) != nil {
		t.Error("nil should clone to nil")
	}
}

func TestCloneNodes_Empty(t *testing.T) {
	got := cloneNodes([]Node{})
	if got != nil {
		t.Error("empty slice should clone to nil (consistent with nil input)")
	}
}

// --- VM clone ---

func TestCloneVM_NetworkIsolation(t *testing.T) {
	src := VM{
		ID:   "vm-1",
		Name: "web",
		NetworkInterfaces: []GuestNetworkInterface{
			{Name: "eth0", Addresses: []string{"192.168.1.10"}},
		},
	}
	dst := cloneVM(src)
	dst.NetworkInterfaces[0].Addresses = append(dst.NetworkInterfaces[0].Addresses, "10.0.0.1")
	if len(src.NetworkInterfaces[0].Addresses) != 1 {
		t.Error("clone should not affect source network interfaces")
	}
}

// --- Host clone ---

func TestCloneHost_MapIsolation(t *testing.T) {
	src := Host{
		ID:       "host-1",
		Hostname: "server1",
		Tags:     []string{"env:prod"},
		Sensors: HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu": 42.0},
		},
	}
	dst := cloneHost(src)
	dst.Tags = append(dst.Tags, "new:val")
	dst.Sensors.TemperatureCelsius["gpu"] = 65.0
	if len(src.Tags) != 1 {
		t.Error("clone tags should be independent")
	}
	if _, ok := src.Sensors.TemperatureCelsius["gpu"]; ok {
		t.Error("clone sensor map should be independent")
	}
}

func TestCloneHosts_Nil(t *testing.T) {
	if cloneHosts(nil) != nil {
		t.Error("nil should clone to nil")
	}
}
