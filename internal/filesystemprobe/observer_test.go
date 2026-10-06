package filesystemprobe

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

func testRequest() ContainerRequest {
	return ContainerRequest{PID: 42, ContainerID: strings.Repeat("a", 64), Runtime: "docker", Mountpoints: []string{"/", "/cache"}}
}

func TestContainerCgroupRequiresExactIdentity(t *testing.T) {
	r := testRequest()
	for _, raw := range []string{"0::/docker/" + r.ContainerID, "1:cpu:/system.slice/docker-" + r.ContainerID + ".scope", "0::/docker/" + r.ContainerID + "/child"} {
		if !cgroupMatches(raw, r) {
			t.Fatalf("exact identity rejected: %q", raw)
		}
	}
	for _, raw := range []string{"0::/", "0::/docker/" + r.ContainerID[:12], "0::/docker/" + r.ContainerID + "b", "0::/system.slice/docker-" + r.ContainerID + ".scope-extra", "0::/libpod-" + r.ContainerID + ".scope", "malformed"} {
		if cgroupMatches(raw, r) {
			t.Fatalf("unattested identity accepted: %q", raw)
		}
	}
	r.Runtime = "podman"
	if !cgroupMatches("0::/user.slice/libpod-"+r.ContainerID+".scope", r) || cgroupMatches("0::/docker/"+r.ContainerID, r) {
		t.Fatal("runtime identity crossed")
	}
}

func TestObserverDoesNotAccumulateOrReuseTimedOutReads(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	o := &Observer{timeout: 20 * time.Millisecond, probe: func(context.Context, ContainerRequest) ([]filesystem.Observation, error) {
		calls.Add(1)
		<-release
		return []filesystem.Observation{{Mountpoint: "/", Usage: &filesystem.Usage{CapacityBytes: 1}}}, nil
	}}
	if got, err := o.Observe(context.Background(), testRequest()); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatalf("timeout: %+v %v", got, err)
	}
	for i := 0; i < 3; i++ {
		if got, err := o.Observe(context.Background(), testRequest()); err == nil || got != nil {
			t.Fatalf("stalled probe reused: %+v %v", got, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("started %d probes for one stuck container", calls.Load())
	}
}

func TestObserverRejectsInvalidCoordinatesBeforeNativeRead(t *testing.T) {
	cases := []func(*ContainerRequest){
		func(r *ContainerRequest) { r.PID = 0 }, func(r *ContainerRequest) { r.ContainerID = "name" },
		func(r *ContainerRequest) { r.Runtime = "remote" }, func(r *ContainerRequest) { r.Mountpoints = nil },
		func(r *ContainerRequest) { r.Mountpoints = []string{"relative"} }, func(r *ContainerRequest) { r.Mountpoints = []string{"/../host"} },
		func(r *ContainerRequest) { r.Mountpoints = []string{"/cache", "/cache"} }, func(r *ContainerRequest) { r.Mountpoints = []string{"/a\x00b"} },
	}
	o := &Observer{probe: func(context.Context, ContainerRequest) ([]filesystem.Observation, error) {
		t.Error("invalid coordinates reached native read")
		return nil, nil
	}}
	for _, change := range cases {
		r := testRequest()
		change(&r)
		if _, err := o.Observe(context.Background(), r); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := o.Observe(ctx, testRequest()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// A Proxmox guest is identified by its exact lxc/<vmid> cgroup and its init
// process. Neither another VMID with the same prefix, LXC's monitor cgroup,
// nor the PID 1 of a namespace nested inside the guest (Docker in LXC) may
// stand in for it.
func TestProxmoxLXCIdentityIsExact(t *testing.T) {
	for raw, want := range map[string]bool{
		"0::/lxc/126/ns":                        true,
		"0::/lxc/126/ns/init.scope":             true,
		"0::/lxc/126":                           true,
		"0::/lxc/1260/ns":                       false,
		"0::/lxc/12/ns":                         false,
		"0::/lxc.monitor/126":                   false,
		"0::/system.slice/lxc/126/ns":           false,
		"0::/docker/" + strings.Repeat("a", 64): false,
	} {
		if got := CgroupMatches(raw, "lxc", "126"); got != want {
			t.Errorf("CgroupMatches(%q) = %v, want %v", raw, got, want)
		}
	}
	for status, want := range map[string]bool{
		"Name:\tinit\nNSpid:\t3019253\t1\n": true,
		"NSpid:\t3019253\t413\n":            false,
		"NSpid:\t3019253\t420\t1\n":         false, // nested namespace init
		"NSpid:\t1\n":                       false, // the host's own init
		"NSpid:\t999\t1\n":                  false, // another process's line
		"Name:\tinit\n":                     false,
	} {
		if got := IsNamespaceInit(status, 3019253); got != want {
			t.Errorf("IsNamespaceInit(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestObserverRejectsInexactProxmoxVMIDs(t *testing.T) {
	o := &Observer{probe: func(context.Context, ContainerRequest) ([]filesystem.Observation, error) {
		t.Error("invalid coordinates reached native read")
		return nil, nil
	}}
	for _, id := range []string{"99", "0126", "126a", "-126", "1000000000", ""} {
		r := ContainerRequest{PID: 42, ContainerID: id, Runtime: "lxc", Mountpoints: []string{"/"}}
		if _, err := o.Observe(context.Background(), r); err == nil {
			t.Fatalf("accepted VMID %q", id)
		}
	}
	accepted := false
	o.probe = func(context.Context, ContainerRequest) ([]filesystem.Observation, error) {
		accepted = true
		return nil, nil
	}
	if _, err := o.Observe(context.Background(), ContainerRequest{PID: 42, ContainerID: "126", Runtime: "lxc", Mountpoints: []string{"/"}}); err != nil || !accepted {
		t.Fatalf("exact VMID rejected: %v", err)
	}
}
