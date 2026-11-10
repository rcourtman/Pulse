package qualification

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	fixtureBaseImage  = "busybox:1.36"
	fixtureBuildLabel = "io.pulse.build"
)

// Fixture is a reviewed stand-in for a real homelab service. Fault set v1
// showed that a model stops diagnosing and starts second-guessing as soon as
// the lab looks like a test: a busybox image name, a script in the container
// command or a log line that disagrees with the network each cost a run. So a
// fixture carries a real image name, keeps its behaviour inside the image and
// logs what the real service would log. Manifests pick a fixture by key and
// cannot change what it runs.
type Fixture struct {
	Key string
	// Image is the tag the fixture is built as. It shadows a real image name
	// on purpose, which is why fixtures are limited to a private Docker daemon
	// and why a build never replaces an image it did not build.
	Image string
	// Entrypoint is the whole behaviour of the service.
	Entrypoint string
	// Healthcheck is the image HEALTHCHECK command, empty for none.
	Healthcheck string
	// Tmpfs lists container paths mounted as tmpfs.
	Tmpfs []string
	// VolumeTarget is where a manifest-declared shared volume is mounted.
	VolumeTarget string
	// ServiceProcessExit is the fixed command the service_process_exit
	// injector runs inside the container. It ends the serving process while
	// the container stays up.
	ServiceProcessExit []string
	// CrashOnce is the fixed command the crash_once injector runs inside the
	// container. The service then dies by itself with its crash exit code.
	CrashOnce []string
}

// fixtureCatalogue is the six-service stack of fault set v1. The entrypoints
// are the ones that produced the Phase 1 baseline.
var fixtureCatalogue = []Fixture{
	{
		Key:         "paperless-web",
		Image:       "ghcr.io/paperless-ngx/paperless-ngx:2.13.5",
		Healthcheck: "wget -q -O /dev/null http://127.0.0.1:8000/ || exit 1",
		Entrypoint: `#!/bin/sh
mkdir -p /www && echo ok > /www/index.html
echo "[init] Paperless-ngx docker container starting..."
echo "[webserver] gunicorn: Listening at: http://0.0.0.0:8000"
httpd -f -p 8000 -h /www &
wait $!
echo "[supervisor] ERROR: webserver (gunicorn) exited unexpectedly with status $?; not respawned" >&2
while true; do sleep 3600; done
`,
		ServiceProcessExit: []string{"pkill", "httpd"},
	},
	{
		Key:   "immich-machine-learning",
		Image: "ghcr.io/immich-app/immich-machine-learning:v1.119.0",
		Tmpfs: []string{"/run/flags"},
		Entrypoint: `#!/bin/sh
echo "[immich-ml] Loading clip model 'ViT-B-32__openai' to memory"
echo "[immich-ml] Application startup complete."
while true; do sleep 3
  if [ -f /run/flags/crash ]; then
    echo "Fatal Python error: Segmentation fault" >&2
    echo "  File \"onnxruntime/capi/onnxruntime_inference_collection.py\", line 220 in run" >&2
    exit 139
  fi
done
`,
		CrashOnce: []string{"touch", "/run/flags/crash"},
	},
	{
		Key:   "flyway-migrate",
		Image: "flyway/flyway:10",
		Entrypoint: `#!/bin/sh
echo "Flyway Community Edition 10.20.1"
echo "Migrating schema \"public\" to version \"42 - add index\""
sleep 1
echo "Successfully applied 1 migration to schema \"public\", now at version v42"
exit 0
`,
	},
	{
		Key:   "vaultwarden",
		Image: "vaultwarden/server:1.32.5",
		Entrypoint: `#!/bin/sh
case "$DATABASE_URL" in
postgresql://*) echo "[vaultwarden::api][INFO] Rocket has launched from http://0.0.0.0:80"; while true; do sleep 3600; done ;;
*) echo "[vaultwarden::db][ERROR] Error: invalid DATABASE_URL \"$DATABASE_URL\": relative URL without a base (expected postgresql://user@host:port/db)" >&2; sleep 2; exit 1 ;;
esac
`,
	},
	{
		Key:          "postgres",
		Image:        "postgres:16",
		VolumeTarget: "/hb",
		// The heartbeat is written to a temporary file and renamed. A plain
		// redirect leaves the file empty for a moment, and under host load the
		// dependent health check read it then and flapped.
		Entrypoint: `#!/bin/sh
echo "LOG:  database system is ready to accept connections"
while true; do date +%s > /hb/beat.tmp && mv /hb/beat.tmp /hb/beat; sleep 2; done
`,
	},
	{
		Key:          "nextcloud",
		Image:        "nextcloud:30-apache",
		VolumeTarget: "/hb",
		Healthcheck:  `b=$(cat /hb/beat 2>/dev/null); test $(( $(date +%s) - ${b:-0} )) -lt 8`,
		// The error names the database by its container name and no address.
		// An address that was not on the real Docker network once sent the
		// model after stale DNS instead of the stopped database.
		Entrypoint: `#!/bin/sh
echo "AH00094: Command line: 'apache2 -D FOREGROUND'"
while true; do
  b=$(cat /hb/beat 2>/dev/null); if [ $(( $(date +%s) - ${b:-0} )) -ge 8 ]; then
    echo "[nextcloud][error] Doctrine\\DBAL\\Exception: SQLSTATE[08006] [7] connection to server at \"nextcloud-db\", port 5432 failed: Connection refused" >&2
  fi
  sleep 10
done
`,
	},
}

func fixtureByKey(key string) (Fixture, bool) {
	for _, fixture := range fixtureCatalogue {
		if fixture.Key == key {
			return fixture, true
		}
	}
	return Fixture{}, false
}

func (f Fixture) dockerfile() string {
	var b strings.Builder
	fmt.Fprintf(&b, "FROM %s\n", fixtureBaseImage)
	b.WriteString("COPY entrypoint.sh /entrypoint.sh\n")
	if f.Healthcheck != "" {
		fmt.Fprintf(&b, "HEALTHCHECK --interval=5s --retries=2 CMD %s\n", f.Healthcheck)
	}
	b.WriteString("ENTRYPOINT [\"/entrypoint.sh\"]\n")
	return b.String()
}

// buildDigest identifies one exact fixture build. The image carries it as a
// label, so a run can refuse an image that is stale or that it did not build.
func (f Fixture) buildDigest() string {
	sum := sha256.Sum256([]byte(f.Image + "\x00" + f.dockerfile() + "\x00" + f.Entrypoint))
	return fmt.Sprintf("%x", sum[:6])
}

type FixtureImageStatus struct {
	Key    string `json:"key"`
	Image  string `json:"image"`
	Digest string `json:"digest"`
	// State is current (already built from this source) or built.
	State string `json:"state"`
}

// fixtureKeys returns the distinct fixtures a set of manifests uses.
func fixtureKeys(manifests []Manifest) []string {
	seen := make(map[string]struct{})
	for _, manifest := range manifests {
		for _, resource := range manifest.Resources {
			if resource.Fixture != "" {
				seen[resource.Fixture] = struct{}{}
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// fixtureImageLabel reads the build label of an image. exists is false when
// the daemon has no such image.
func (l *DockerLab) fixtureImageLabel(ctx context.Context, image string) (label string, exists bool) {
	result, err := l.docker(ctx, "image", "inspect", "--format", `{{ index .Config.Labels "`+fixtureBuildLabel+`" }}`, image)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(result.Stdout), true
}

func (l *DockerLab) requireFixtureImage(ctx context.Context, fixture Fixture) error {
	label, exists := l.fixtureImageLabel(ctx, fixture.Image)
	if !exists || label != fixture.buildDigest() {
		return fmt.Errorf("fixture image %s is missing or was not built from the current fixture source: run patrol-qualify -mode build-fixtures first", fixture.Image)
	}
	return nil
}

// BuildFixtureImages builds every fixture the manifests use and leaves current
// ones alone. It is a separate, explicit step so that a scenario run starts
// from an image inventory it does not change.
func (l *DockerLab) BuildFixtureImages(ctx context.Context, manifests []Manifest) ([]FixtureImageStatus, error) {
	if l.target.SSHHost != "" {
		return nil, errors.New("fixture images are built through a Docker context, not an SSH host")
	}
	if l.target.Context == "" {
		return nil, errors.New("an explicit Docker context is required")
	}
	for _, manifest := range manifests {
		if manifest.Lab.SharedHostOK {
			return nil, fmt.Errorf("scenario %s may run on a shared host and cannot use fixture images", manifest.ID)
		}
	}
	var statuses []FixtureImageStatus
	for _, key := range fixtureKeys(manifests) {
		fixture, ok := fixtureByKey(key)
		if !ok {
			return statuses, fmt.Errorf("unknown fixture %q", key)
		}
		status := FixtureImageStatus{Key: fixture.Key, Image: fixture.Image, Digest: fixture.buildDigest(), State: "current"}
		label, exists := l.fixtureImageLabel(ctx, fixture.Image)
		switch {
		case exists && label == status.Digest:
		case exists && label == "":
			return statuses, fmt.Errorf("image %s on this Docker daemon was not built by this harness: fixtures need a private daemon and never replace a real image", fixture.Image)
		default:
			if err := l.buildFixtureImage(ctx, fixture); err != nil {
				return statuses, err
			}
			status.State = "built"
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (l *DockerLab) buildFixtureImage(ctx context.Context, fixture Fixture) error {
	dir, err := os.MkdirTemp("", "fixture-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "entrypoint.sh"), []byte(fixture.Entrypoint), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(fixture.dockerfile()), 0o644); err != nil {
		return err
	}
	// The label is passed on the command line, not written into the
	// Dockerfile, so the digest can cover the Dockerfile itself.
	if _, err := l.docker(ctx, "build", "-q", "--label", fixtureBuildLabel+"="+fixture.buildDigest(), "-t", fixture.Image, dir); err != nil {
		return fmt.Errorf("build fixture image %s: %w", fixture.Image, err)
	}
	return nil
}
