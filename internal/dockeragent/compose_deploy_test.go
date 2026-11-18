package dockeragent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

// Run the public typed command against a real, disposable Git remote. The
// agent must leave main, the checkout, the normal index and Docker untouched.
func TestComposeGitProposalPreservesBaseAndWorkingTree(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "checkout")
	remote := filepath.Join(dir, "origin.git")
	git := func(cwd string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = cwd
		res, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %s: %s (%v)", args[0], res, err)
		}
		return strings.TrimSpace(string(res))
	}
	git(dir, "init", "--bare", "--initial-branch=main", remote)
	git(dir, "init", "--initial-branch=main", repo)
	git(repo, "config", "user.name", "Deployment Operator")
	git(repo, "config", "user.email", "operator@example.test")
	git(repo, "config", "commit.gpgsign", "false")
	file := "compose.yaml"
	data := []byte("services:\n  api:\n    image: example/web:1 # tracked\n  worker:\n    image: example/web:1\n")
	if err := os.WriteFile(filepath.Join(repo, file), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", file)
	git(repo, "commit", "-m", "Initial services")
	git(repo, "remote", "add", "origin", remote)
	git(repo, "push", "-u", "origin", "main")
	base := git(repo, "rev-parse", "HEAD")
	source := composeSource{Project: "photos", Mode: "git", Repository: repo, Remote: "origin", Branch: "main", File: file}
	registry := filepath.Join(dir, "sources.json")
	wire, _ := json.Marshal(map[string]any{"projects": []composeSource{source}})
	if err := os.WriteFile(registry, wire, 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, err := source.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	target := strings.Repeat("b", 64)
	oldDigest := "sha256:" + strings.Repeat("f", 64)
	change, updated, err := source.review(snap, "api", "example/web:1", digest)
	if err != nil {
		t.Fatal(err)
	}
	a := &Agent{runtime: RuntimeDocker, cfg: Config{ComposeSourcesFile: registry}, logger: zerolog.Nop(), docker: &fakeDockerClient{containerInspectFn: func(_ context.Context, id string) (containertypes.InspectResponse, error) {
		ct := baseInspect()
		ct.ID = target
		ct.Image = oldDigest
		ct.State.Running = true
		ct.Config.Image = "example/web:1"
		ct.Config.Labels = map[string]string{"com.docker.compose.project": "photos", "com.docker.compose.service": "api"}
		return ct, nil
	}}}
	req := agentexec.DockerContainerUpdatePayload{RequestID: "attempt-test", ActionID: "action-test", Runtime: "docker", ContainerID: target, ExpectedImageDigest: oldDigest, Compose: change}
	if err := agentexec.BindDockerContainerUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	if err := a.TypedComposeUpdatePreflight(ctx, req); err != nil {
		t.Fatal(err)
	}
	out, err := a.TypedComposeUpdate(ctx, req)
	if err != nil || !out.Success || out.ProposalCommit == "" || out.NewContainerID != "" {
		t.Fatalf("proposal %#v, %v", out, err)
	}
	if got := git(remote, "show", out.ProposalBranch+":"+file); got != strings.TrimSpace(string(updated)) {
		t.Fatal("proposal changed additional source content")
	}
	if git(remote, "rev-parse", "main") != base || git(repo, "status", "--porcelain") != "" {
		t.Fatal("proposal modified main, checkout or index")
	}
	if direct := a.updateContainerWithProgress(ctx, target, nil); direct.Success || direct.ReasonCode != agentexec.ActionRefusalTargetPreconditionFailed {
		t.Fatalf("registered source allowed direct update: %#v", direct)
	}
	if again, err := a.TypedComposeUpdate(ctx, req); err == nil && again.Success {
		// Re-pushing the exact same commit is idempotent. A later retry may
		// never move the first review branch.
		if git(remote, "rev-parse", out.ProposalBranch) != out.ProposalCommit {
			t.Fatal("retry moved review branch")
		}
	}
	if err := os.WriteFile(filepath.Join(repo, file), append(data, []byte("# changed by deployment owner\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	git(repo, "commit", "-am", "Update source")
	git(repo, "push", "origin", "main")
	if err := a.TypedComposeUpdatePreflight(ctx, req); err == nil {
		t.Fatal("stale source review accepted")
	}
}

// The registry and both tiny HTTP images are owned by a disposable lab. The
// test creates one unique project, a loopback port and one volume. Its cleanup
// addresses just that project on the selected daemon.
func TestComposeLiveReviewedServiceUpdate(t *testing.T) {
	ref := os.Getenv("PULSE_COMPOSE_TEST_BEFORE")
	digest := os.Getenv("PULSE_COMPOSE_TEST_DIGEST")
	if ref == "" || digest == "" {
		t.Skip("requires a private Docker registry with two HTTP fixture manifests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	project := "pulsesource" + updateIntegrationSuffix(t)
	source := composeSource{Project: project, Mode: "local", File: filepath.Join(dir, "compose.yaml")}
	src := fmt.Sprintf("services:\n  web:\n    image: %s\n    ports: [\"127.0.0.1::8080\"]\n    volumes: [\"data:/data\"]\n  neighbor:\n    image: %s\nvolumes:\n  data: {}\n", ref, ref)
	if err := os.WriteFile(source.File, []byte(src), 0640); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "sources.json")
	data, _ := json.Marshal(map[string]any{"projects": []composeSource{source}})
	if err := os.WriteFile(cfg, data, 0600); err != nil {
		t.Fatal(err)
	}
	cli, err := newMobyDockerClient(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	a := &Agent{runtime: RuntimeDocker, docker: cli, cfg: Config{ComposeSourcesFile: cfg}, logger: zerolog.Nop()}
	p := reviewedComposeService{source: source, endpoint: cli.DaemonHost()}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := p.compose(c, source.File, "down", "--volumes"); err != nil {
			t.Errorf("cleanup %s: %v", project, err)
		}
	})
	if _, err = p.compose(ctx, source.File, "up", "-d"); err != nil {
		t.Fatalf("start fixture %s: %v", project, err)
	}
	readHTTP := func() string {
		t.Helper()
		port, err := p.compose(ctx, source.File, "port", "web", "8080")
		if err != nil {
			t.Fatal(err)
		}
		uri := "http://" + strings.TrimSpace(string(port)) + "/health"
		h := http.Client{Timeout: 2 * time.Second}
		for i := 0; i < 20; i++ {
			r, err := h.Get(uri)
			if err == nil {
				b, e := io.ReadAll(io.LimitReader(r.Body, 4096))
				_ = r.Body.Close()
				if e == nil && r.StatusCode == 200 {
					return strings.TrimSpace(string(b))
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
		t.Fatal("loopback HTTP check failed")
		return ""
	}
	beforeBody := readHTTP()
	oldID, err := p.serviceID(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	neighbor, err := p.serviceID(ctx, "neighbor")
	if err != nil {
		t.Fatal(err)
	}
	before, err := cli.ContainerInspect(ctx, oldID)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := source.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	change, updated, err := source.review(snap, "web", before.Config.Image, digest)
	if err != nil {
		t.Fatal(err)
	}
	req := agentexec.DockerContainerUpdatePayload{RequestID: "fixture-attempt", ActionID: "fixture-action", Runtime: "docker", ContainerID: oldID, ExpectedImageDigest: before.Image, Compose: change}
	if err := agentexec.BindDockerContainerUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	if err := a.TypedComposeUpdatePreflight(ctx, req); err != nil {
		t.Fatal(err)
	}
	out, err := a.TypedComposeUpdate(ctx, req)
	if err != nil || !out.Success || out.NewImageDigest != digest {
		t.Fatalf("update %#v %v", out, err)
	}
	afterBody := readHTTP()
	if beforeBody == afterBody {
		t.Fatalf("expected different HTTP response bodies across deploy: %q", afterBody)
	}
	current, err := os.ReadFile(source.File)
	if err != nil || string(current) != string(updated) {
		t.Fatal("incorrect source after deployment")
	}
	oldSource, err := os.ReadFile(source.File + ".pulse-before-fixture-action")
	if err != nil || string(oldSource) != src {
		t.Fatal("previous source missing")
	}
	if id, e := p.serviceID(ctx, "neighbor"); e != nil || id != neighbor {
		t.Fatal("neighbor was recreated")
	}
	after, err := cli.ContainerInspect(ctx, out.NewContainerID)
	if err != nil || len(before.Mounts) == 0 || len(after.Mounts) == 0 || before.Mounts[0].Name != after.Mounts[0].Name {
		t.Fatal("data volume identity changed")
	}
	t.Logf("project %s HTTP before %q after %q, replacement %s, peer and data volume retained", project, beforeBody, afterBody, out.NewContainerID)
}

func TestInstallReviewedComposeSourceKeepsRecoveryBesideRegisteredFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "compose.yaml")
	staged := filepath.Join(dir, "staged.yaml")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(path, want string) {
		t.Helper()
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("read %s: got %q, want %q (error %v)", filepath.Base(path), got, want, err)
		}
	}
	write(source, "first")
	write(staged, "second")
	if err := installReviewedComposeSource(source, staged, "invalid/action"); err == nil {
		t.Fatal("accepted non-component action ID")
	}
	if err := installReviewedComposeSource(source, staged, "review-1"); err != nil {
		t.Fatal(err)
	}
	write(staged, "third")
	if err := installReviewedComposeSource(source, staged, "review-1"); err == nil {
		t.Fatal("reused an existing recovery copy")
	}
	check(source, "second")
	check(source+".pulse-before-review-1", "first")
	check(staged, "third")
}
