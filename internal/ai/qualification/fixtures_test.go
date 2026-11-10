package qualification

import (
	"context"
	"strings"
	"testing"
	"time"
)

const fixtureTestRunID = "q-20261110-0badc0de"

func faultSetManifest(t *testing.T, id string) Manifest {
	t.Helper()
	manifest, ok := loadFaultSetCatalog(t).ByID[id]
	if !ok {
		t.Fatalf("catalogue has no scenario %s", id)
	}
	return manifest
}

func seededHostFilesystem() *fakeHostFilesystem {
	return &fakeHostFilesystem{MountPath: "/mnt/nvr", TotalMiB: 400, Mounted: true, Files: map[string]int{
		"/mnt/nvr/music/library.tar": 20, "/mnt/nvr/photos/2025-archive.tar": 30,
		"/mnt/nvr/frigate/recordings/2026-11-09/14/front_door/00.00.mp4": 15,
	}}
}

// What made fault set v1 fixtures honest is that nothing a model can inspect
// says "test": a real image name, a real container name and no script in the
// container command.
func TestFixtureContainersLookLikeTheRealService(t *testing.T) {
	manifest := faultSetManifest(t, "response.app-unhealthy-database-stopped")
	docker := newFakeDocker()
	driver := NewDockerLab(docker, DockerTarget{Context: "lab"})
	lab, err := driver.Prepare(context.Background(), manifest, fixtureTestRunID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"paperless": "ghcr.io/paperless-ngx/paperless-ngx:2.13.5", "immich_machine_learning": "ghcr.io/immich-app/immich-machine-learning:v1.119.0",
		"vaultwarden-db-migrate": "flyway/flyway:10", "vaultwarden": "vaultwarden/server:1.32.5",
		"nextcloud-db": "postgres:16", "nextcloud": "nextcloud:30-apache",
	}
	for name, image := range want {
		container := docker.container(name)
		if container == nil {
			t.Fatalf("no container named exactly %q: %v", name, lab.ResourceNames)
		}
		if container.Image != image {
			t.Errorf("%s runs %s, want %s", name, container.Image, image)
		}
		if container.Labels[labRunLabel] != labRunToken(fixtureTestRunID) {
			t.Errorf("%s lacks the run label cleanup depends on", name)
		}
	}
	for _, call := range docker.calls {
		if !strings.Contains(call, " run -d ") {
			continue
		}
		// The image is the last argument: a fixture passes no command.
		image := call[strings.LastIndex(call, " ")+1:]
		known := false
		for _, wanted := range want {
			known = known || image == wanted
		}
		if !known {
			t.Errorf("fixture container was started with a command after its image: %s", call)
		}
		for _, revealing := range []string{"pulse-qual", "/bin/sh", "busybox", "health-cmd"} {
			if strings.Contains(call, revealing) {
				t.Errorf("fixture container start reveals %q: %s", revealing, call)
			}
		}
	}
	if got := docker.container("vaultwarden").Env["DATABASE_URL"]; got != "postgresql://vault@10.0.0.5:5432/vault" {
		t.Errorf("vaultwarden DATABASE_URL = %q", got)
	}
	if got := docker.container("immich_machine_learning").Tmpfs; len(got) != 1 || got[0] != "/run/flags" {
		t.Errorf("crash flag tmpfs = %v", got)
	}
	for _, name := range []string{"nextcloud", "nextcloud-db"} {
		if got := docker.container(name).Mounts; len(got) != 1 || got[0] != "source=nextcloud_hb,target=/hb" {
			t.Errorf("%s mounts = %v, want the shared heartbeat volume", name, got)
		}
	}
	if docker.called("volume create") != 1 {
		t.Errorf("the shared volume was created %d times, want once", docker.called("volume create"))
	}
	// The app's error names its database by container name, so the two have
	// to agree or the log points at a host that does not exist.
	nextcloud, _ := fixtureByKey("nextcloud")
	if !strings.Contains(nextcloud.Entrypoint, `connection to server at \"nextcloud-db\"`) {
		t.Fatal("the nextcloud fixture no longer names nextcloud-db")
	}
	for _, fixture := range fixtureCatalogue {
		for _, address := range []string{"172.", "192.168."} {
			if strings.Contains(fixture.Entrypoint, address) {
				t.Errorf("fixture %s logs a network address (%s) the lab cannot guarantee", fixture.Key, address)
			}
		}
	}
	postgres, _ := fixtureByKey("postgres")
	if !strings.Contains(postgres.Entrypoint, "beat.tmp && mv /hb/beat.tmp /hb/beat") {
		t.Fatal("the heartbeat is no longer written atomically")
	}

	cleanup := driver.Cleanup(context.Background(), manifest, lab)
	if !cleanup.Passed || !cleanup.SecondCleanupNoop || !cleanup.InventoryUnchanged {
		t.Fatalf("cleanup of a fixture stack failed: %+v", cleanup)
	}
	if len(docker.containers) != 0 || len(docker.volumes) != 0 || len(docker.networks) != 0 {
		t.Fatalf("cleanup left objects behind: %v %v %v", docker.containers, docker.volumes, docker.networks)
	}
}

func TestFaultSetInjectorsAndOracles(t *testing.T) {
	cases := []struct {
		scenario string
		check    func(t *testing.T, docker *fakeDocker, applied bool)
	}{
		{"response.web-worker-died", func(t *testing.T, docker *fakeDocker, applied bool) {
			container := docker.container("paperless")
			if !container.Running || (container.Health == "unhealthy") != applied {
				t.Fatalf("paperless running=%t health=%s applied=%t", container.Running, container.Health, applied)
			}
		}},
		{"response.worker-crashed-stays-exited", func(t *testing.T, docker *fakeDocker, applied bool) {
			container := docker.container("immich_machine_learning")
			if container.Running == applied || (applied && container.ExitCode != 139) {
				t.Fatalf("immich running=%t exit=%d applied=%t", container.Running, container.ExitCode, applied)
			}
		}},
		{"response.restart-loop-bad-config", func(t *testing.T, docker *fakeDocker, applied bool) {
			container := docker.container("vaultwarden")
			want := "postgresql://vault@10.0.0.5:5432/vault"
			if applied {
				want = "10.0.0.5:5432/vault"
			}
			if container.Env["DATABASE_URL"] != want || container.Image != "vaultwarden/server:1.32.5" || container.Restart != "always" ||
				container.Labels[labRunLabel] != labRunToken(fixtureTestRunID) {
				t.Fatalf("vaultwarden after applied=%t: %+v", applied, container)
			}
		}},
		{"response.app-unhealthy-database-stopped", func(t *testing.T, docker *fakeDocker, applied bool) {
			if docker.container("nextcloud-db").Running == applied || !docker.container("nextcloud").Running {
				t.Fatalf("database running=%t applied=%t", docker.container("nextcloud-db").Running, applied)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			manifest := faultSetManifest(t, tc.scenario)
			docker := newFakeDocker()
			driver := NewDockerLab(docker, DockerTarget{Context: "lab"})
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			lab, err := driver.Prepare(ctx, manifest, fixtureTestRunID)
			if err != nil {
				t.Fatal(err)
			}
			fault := manifest.Faults[0]
			before := lab.ResourceIDs[fault.Target]
			if err := driver.ApplyFault(ctx, manifest, lab, fault); err != nil {
				t.Fatal(err)
			}
			tc.check(t, docker, true)
			if _, err := driver.Observe(ctx, manifest, lab, fault.Oracle); err != nil {
				t.Fatalf("fault oracle: %v", err)
			}
			if fault.Injector.Kind == "recreate_with_env" && lab.ResourceIDs[fault.Target] == before {
				t.Fatal("a recreated container kept its old Docker ID in the lab record")
			}
			if err := driver.RevertFault(ctx, manifest, lab, fault); err != nil {
				t.Fatal(err)
			}
			tc.check(t, docker, false)
			if _, err := driver.Observe(ctx, manifest, lab, fault.RevertOracle); err != nil {
				t.Fatalf("revert oracle: %v", err)
			}
			if _, err := driver.Observe(ctx, manifest, lab, manifest.Baseline); err != nil {
				t.Fatalf("baseline after revert: %v", err)
			}
			if cleanup := driver.Cleanup(ctx, manifest, lab); !cleanup.Passed {
				t.Fatalf("cleanup: %+v", cleanup)
			}
		})
	}
}

func TestFixtureImagesAreBuiltOnceAndNeverReplaceARealImage(t *testing.T) {
	manifests := FaultSetManifests(loadFaultSetCatalog(t), "v1")
	ctx := context.Background()

	docker := newFakeDocker()
	docker.images = map[string]string{fixtureBaseImage: ""}
	driver := NewDockerLab(docker, DockerTarget{Context: "lab"})
	statuses, err := driver.BuildFixtureImages(ctx, manifests)
	if err != nil || len(statuses) != len(fixtureCatalogue) {
		t.Fatalf("first build: %v %+v", err, statuses)
	}
	for _, status := range statuses {
		if status.State != "built" || docker.images[status.Image] != status.Digest {
			t.Errorf("fixture %s state=%s label=%q digest=%q", status.Key, status.State, docker.images[status.Image], status.Digest)
		}
	}
	builds := docker.called(" build ")
	if statuses, err = driver.BuildFixtureImages(ctx, manifests); err != nil || docker.called(" build ") != builds {
		t.Fatalf("second build rebuilt current images: %v", err)
	}
	for _, status := range statuses {
		if status.State != "current" {
			t.Errorf("fixture %s state=%s on the second build", status.Key, status.State)
		}
	}

	// A real postgres:16 on the daemon carries no build label.
	docker.images["postgres:16"] = ""
	if _, err := driver.BuildFixtureImages(ctx, manifests); err == nil || !strings.Contains(err.Error(), "never replace a real image") {
		t.Fatalf("a real image was accepted or replaced: %v", err)
	}
	if docker.images["postgres:16"] != "" {
		t.Fatal("the real image was overwritten")
	}
	if _, err := driver.Prepare(ctx, manifests[0], fixtureTestRunID); err == nil || !strings.Contains(err.Error(), "build-fixtures") {
		t.Fatalf("a run started on an image the harness did not build: %v", err)
	}

	// A fixture whose source changed since the build is stale, not current.
	docker.images["postgres:16"] = "000000000000"
	if statuses, err = driver.BuildFixtureImages(ctx, manifests); err != nil {
		t.Fatal(err)
	}
	rebuilt := 0
	for _, status := range statuses {
		if status.State == "built" {
			rebuilt++
		}
	}
	if rebuilt != 1 {
		t.Fatalf("%d fixtures rebuilt after one went stale, want 1", rebuilt)
	}

	if _, err := NewDockerLab(docker, DockerTarget{SSHHost: "lab"}).BuildFixtureImages(ctx, manifests); err == nil {
		t.Fatal("fixture images must not be built over SSH")
	}
}

func TestHostFilesystemGuardRefusesAnythingButADedicatedMount(t *testing.T) {
	manifest := faultSetManifest(t, "response.host-disk-one-folder")
	ctx := context.Background()
	prepare := func(target DockerTarget, mutate func(*fakeDocker)) (*fakeDocker, error) {
		docker := newFakeDocker()
		docker.host = seededHostFilesystem()
		if mutate != nil {
			mutate(docker)
		}
		_, err := NewDockerLab(docker, target).Prepare(ctx, manifest, fixtureTestRunID)
		return docker, err
	}
	handed := DockerTarget{Context: "lab", HostFilesystems: []string{"/mnt/nvr"}}
	for name, tc := range map[string]struct {
		target DockerTarget
		mutate func(*fakeDocker)
		want   string
	}{
		"not handed to the lab":      {DockerTarget{Context: "lab"}, nil, "--host-filesystem"},
		"a different path was given": {DockerTarget{Context: "lab", HostFilesystems: []string{"/mnt/other"}}, nil, "--host-filesystem"},
		"not mounted":                {handed, func(d *fakeDocker) { d.host.Mounted = false }, "not a separate mounted filesystem"},
		"far too large":              {handed, func(d *fakeDocker) { d.host.TotalMiB = 200 * 1024 }, "a lab filesystem is between"},
		"no standing content":        {handed, func(d *fakeDocker) { d.host.Files = map[string]int{} }, "no standing content"},
		"remote Docker daemon":       {handed, func(d *fakeDocker) { d.endpoint = "tcp://lab:2376" }, "not a local socket"},
	} {
		t.Run(name, func(t *testing.T) {
			docker, err := prepare(tc.target, tc.mutate)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if docker.called("dd ") != 0 {
				t.Fatal("the driver wrote to a filesystem it had refused")
			}
		})
	}
	if _, err := prepare(handed, nil); err != nil {
		t.Fatalf("a seeded, dedicated, handed-over mount was refused: %v", err)
	}
}

func TestHostFilesystemFillGrowsOneFolderAndRevertsExactly(t *testing.T) {
	manifest := faultSetManifest(t, "response.host-disk-one-folder")
	docker := newFakeDocker()
	docker.host = seededHostFilesystem()
	driver := NewDockerLab(docker, DockerTarget{Context: "lab", HostFilesystems: []string{"/mnt/nvr"}})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	lab, err := driver.Prepare(ctx, manifest, fixtureTestRunID)
	if err != nil {
		t.Fatal(err)
	}
	seed := len(docker.host.Files)
	fault := manifest.Faults[0]
	if err := driver.ApplyFault(ctx, manifest, lab, fault); err != nil {
		t.Fatal(err)
	}
	state := lab.HostFilesystems["nvr"]
	if !strings.HasPrefix(state.FaultDir, "/mnt/nvr/frigate/recordings/") || !strings.HasSuffix(state.FaultDir, "/driveway") {
		t.Fatalf("fault folder = %s", state.FaultDir)
	}
	if len(state.FaultFiles) == 0 || len(docker.host.Files) != seed+len(state.FaultFiles) {
		t.Fatalf("recorded %d fault files, filesystem gained %d", len(state.FaultFiles), len(docker.host.Files)-seed)
	}
	for _, file := range state.FaultFiles {
		if !strings.HasPrefix(file, state.FaultDir+"/") || docker.host.Files[file] != hostFillFileMiB {
			t.Fatalf("fault file %s is outside the fault folder or not %d MiB", file, hostFillFileMiB)
		}
	}
	observations, err := driver.Observe(ctx, manifest, lab, fault.Oracle)
	if err != nil {
		t.Fatalf("fill oracle: %v (%+v)", err, observations)
	}
	if used, ok := observations[0].Observed.(float64); !ok || used < hostFillTargetPercent || used >= 100 {
		t.Fatalf("filesystem at %v%% after the fill, want at least %.0f%% and not full", observations[0].Observed, hostFillTargetPercent)
	}
	if err := driver.ApplyFault(ctx, manifest, lab, fault); err == nil {
		t.Fatal("a second fill was accepted")
	}

	// Cleanup alone has to remove the fault, for a run that never reached
	// its revert.
	if cleanup := driver.Cleanup(ctx, manifest, lab); !cleanup.Passed {
		t.Fatalf("cleanup: %+v", cleanup)
	}
	if len(docker.host.Files) != seed || len(state.FaultFiles) != 0 {
		t.Fatalf("cleanup left %d files, the standing content is %d", len(docker.host.Files), seed)
	}
	for _, marker := range hostSeedMarkers {
		if _, ok := docker.host.Files["/mnt/nvr/"+marker]; !ok {
			t.Fatalf("cleanup removed standing content %s", marker)
		}
	}
	if _, err := driver.Observe(ctx, manifest, lab, fault.RevertOracle); err != nil {
		t.Fatalf("revert oracle: %v", err)
	}
}

func TestSeedHostFilesystemIsExplicitAndIdempotent(t *testing.T) {
	docker := newFakeDocker()
	docker.host = &fakeHostFilesystem{MountPath: "/mnt/nvr", TotalMiB: 400, Mounted: true, Files: map[string]int{}}
	driver := NewDockerLab(docker, DockerTarget{Context: "lab", HostFilesystems: []string{"/mnt/nvr"}})
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	seeded, err := driver.SeedHostFilesystem(context.Background(), "/mnt/nvr", now)
	if err != nil || !seeded {
		t.Fatalf("seed: %v seeded=%t", err, seeded)
	}
	if docker.host.Files["/mnt/nvr/frigate/recordings/2026-11-09/14/front_door/00.00.mp4"] != 15 || len(docker.host.Files) != 3 {
		t.Fatalf("standing content = %v", docker.host.Files)
	}
	if seeded, err = driver.SeedHostFilesystem(context.Background(), "/mnt/nvr", now); err != nil || seeded {
		t.Fatalf("second seed: %v seeded=%t", err, seeded)
	}
	if _, err := driver.SeedHostFilesystem(context.Background(), "/mnt/other", now); err == nil {
		t.Fatal("a mount that was not handed to the lab was seeded")
	}
}

func TestResponseManifestValidation(t *testing.T) {
	base := func() Manifest { return faultSetManifest(t, "response.app-unhealthy-database-stopped") }
	for name, tc := range map[string]struct {
		mutate func(*Manifest)
		want   string
	}{
		"no outcome":                 {func(m *Manifest) { m.Outcome = nil }, "require outcome"},
		"no engagement":              {func(m *Manifest) { m.Engagement = nil }, "require engagement"},
		"no decisive terms":          {func(m *Manifest) { m.Outcome.DecisiveTerms = nil }, "decisive_terms is required"},
		"fixed without a fix":        {func(m *Manifest) { m.Remediation = nil }, "expect=fixed requires remediation"},
		"handed over with a fix":     {func(m *Manifest) { m.Outcome.Expect = OutcomeExpectHandedOver }, "no action exists"},
		"quiet with a fault":         {func(m *Manifest) { m.Outcome.Expect = OutcomeExpectQuiet }, "quiet response scenario"},
		"unknown expectation":        {func(m *Manifest) { m.Outcome.Expect = "improved" }, "unsupported outcome.expect"},
		"investigation block":        {func(m *Manifest) { m.Investigation = &InvestigationSpec{MinEvidenceCalls: 1} }, "not in investigation"},
		"unscoped":                   {func(m *Manifest) { m.Patrol.Scoped = false }, "patrol.scoped=true"},
		"fixture on a shared host":   {func(m *Manifest) { m.Lab.SharedHostOK = true }, "private Docker daemon"},
		"unknown fixture":            {func(m *Manifest) { m.Resources[0].Fixture = "nginx" }, "unknown fixture"},
		"fixture with its own image": {func(m *Manifest) { m.Resources[0].Image = "alpine:3.20" }, "a fixture owns"},
		"crash a fixture that cannot": {func(m *Manifest) {
			m.Faults[0].Injector = InjectorSpec{Kind: "crash_once", Resource: "nextcloud-db"}
		}, "not supported by the fixture"},
		"env change on an undeclared key": {func(m *Manifest) {
			m.Faults[0].Injector = InjectorSpec{Kind: "recreate_with_env", Resource: "vaultwarden", Value: "ADMIN_TOKEN=x"}
		}, "env key the resource already declares"},
		"host mount outside the accepted roots": {func(m *Manifest) {
			m.Resources = append(m.Resources, ResourceSpec{Alias: "disk", Kind: resourceKindHostFilesystem, MountPath: "/var/lib/docker"})
		}, "dedicated mount directly under"},
		"docker probe on a host filesystem": {func(m *Manifest) {
			m.Resources = append(m.Resources, ResourceSpec{Alias: "disk", Kind: resourceKindHostFilesystem, MountPath: "/mnt/nvr"})
			m.Baseline = append(m.Baseline, Predicate{Probe: "docker.running", Target: "disk", Operator: "eq", Value: []byte("true")})
		}, "requires a container resource"},
	} {
		t.Run(name, func(t *testing.T) {
			manifest := base()
			tc.mutate(&manifest)
			if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	watch := validTestManifest()
	watch.Outcome = &OutcomeSpec{Expect: OutcomeExpectQuiet}
	if err := watch.Validate(); err == nil || !strings.Contains(err.Error(), "only response scenarios") {
		t.Fatalf("a Watch scenario accepted an outcome rubric: %v", err)
	}
	watch = validTestManifest()
	watch.Resources = append(watch.Resources, ResourceSpec{Alias: "disk", Kind: resourceKindHostFilesystem, MountPath: "/mnt/nvr"})
	if err := watch.Validate(); err == nil || !strings.Contains(err.Error(), "limited to response scenarios") {
		t.Fatalf("a Watch scenario accepted a host filesystem: %v", err)
	}
}
