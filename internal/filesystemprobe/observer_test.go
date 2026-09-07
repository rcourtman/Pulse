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
