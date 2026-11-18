package dockeragent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

type composeSnapshot struct {
	data         []byte
	commit, hash string
}
type composeOutput struct{ data bytes.Buffer }

func (w *composeOutput) Write(b []byte) (int, error) {
	if w.data.Len()+len(b) > maxComposeSourceBytes {
		return 0, errors.New("deployment command output exceeds its limit")
	}
	return w.data.Write(b)
}

// Command output can contain rendered environment values and Git credential
// diagnostics. Only closed stage descriptions and bounded typed values leave
// the agent. No arguments ever come from container labels except as verified
// exact project/service selectors in a locally registered source.
func deploymentCommand(ctx context.Context, dir string, env []string, stdin []byte, program string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	for _, pair := range os.Environ() {
		key, _, _ := strings.Cut(pair, "=")
		if !strings.HasPrefix(key, "GIT_") && !strings.HasPrefix(key, "COMPOSE_") && key != "DOCKER_CONTEXT" {
			cmd.Env = append(cmd.Env, pair)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = bytes.NewReader(stdin)
	var output composeOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, errors.New("deployment command did not complete")
	}
	return output.data.Bytes(), nil
}
func (s composeSource) git(ctx context.Context, env []string, input []byte, args ...string) ([]byte, error) {
	return deploymentCommand(ctx, s.Repository, env, input, "git", append([]string{"--literal-pathspecs", "-c", "core.hooksPath=" + os.DevNull, "-c", "protocol.ext.allow=never"}, args...)...)
}
func (s composeSource) snapshot(ctx context.Context) (composeSnapshot, error) {
	var snap composeSnapshot
	var err error
	if s.Mode == "git" {
		base, err := s.git(ctx, nil, nil, "rev-parse", "--verify", "refs/remotes/"+s.Remote+"/"+s.Branch+"^{commit}")
		if err != nil {
			return snap, errors.New("registered Git base ref unavailable")
		}
		snap.commit = strings.TrimSpace(string(base))
		snap.data, err = s.git(ctx, nil, nil, "show", snap.commit+":"+s.File)
		if err != nil {
			return snap, errors.New("registered Compose file unavailable in Git base")
		}
	} else {
		stat, err := os.Lstat(s.File)
		if err != nil || !stat.Mode().IsRegular() {
			return snap, errors.New("registered local source must be a regular file")
		}
		snap.data, err = readComposeSourceFile(s.File)
		if err != nil {
			return snap, errors.New("registered local source could not be read")
		}
	}
	// Bind both source owner and contents. Re-registering the project against
	// an identical file in another directory requires a new approval.
	pushURL := ""
	if s.Mode == "git" {
		remote, err := s.git(ctx, nil, nil, "remote", "get-url", "--push", s.Remote)
		if err != nil {
			return snap, errors.New("registered Git remote unavailable")
		}
		pushURL = strings.TrimSpace(string(remote))
	}
	spec, _ := json.Marshal([]any{s, pushURL, snap.commit, composeSourceHash(snap.data)})
	snap.hash = fmt.Sprintf("sha256:%x", sha256.Sum256(spec))
	return snap, err
}

func (s composeSource) review(snapshot composeSnapshot, service, runningImage, digest string) (*agentsdocker.ComposeImageChange, []byte, error) {
	before, after, updated, err := composeImageEdit(snapshot.data, service, digest)
	if err != nil {
		return nil, nil, err
	}
	live, err := reference.ParseNormalizedNamed(runningImage)
	if err != nil {
		return nil, nil, errors.New("running image reference unavailable")
	}
	saved, err := reference.ParseNormalizedNamed(before)
	if err != nil || reference.TagNameOnly(live).String() != reference.TagNameOnly(saved).String() {
		return nil, nil, errors.New("source image differs from the deployed service")
	}
	change := &agentsdocker.ComposeImageChange{Mode: s.Mode, Project: s.Project, Service: service, SourceHash: snapshot.hash, BaseCommit: snapshot.commit, BaseBranch: s.Branch, BeforeImage: before, AfterImage: after}
	if !change.Valid() {
		return nil, nil, errors.New("source review unavailable")
	}
	return change, updated, nil
}

// Read each registered project once per inventory. Ordinary installs do no
// filesystem or Git reads. A broken registration grants no update capability.
func (a *Agent) projectComposeUpdates(ctx context.Context, containers []agentsdocker.Container, sources map[string]composeSource, err error) {
	if a.cfg.ComposeSourcesFile == "" {
		return
	}
	if err != nil {
		for i := range containers {
			containers[i].ComposeSourceRegistered = containers[i].Labels["com.docker.compose.project"] != ""
		}
		return
	}
	inspect, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	snapshots := make(map[string]composeSnapshot)
	seen := make(map[string]bool)
	for i := range containers {
		c := &containers[i]
		project, service := c.Labels["com.docker.compose.project"], c.Labels["com.docker.compose.service"]
		source, ok := sources[project]
		c.ComposeSourceRegistered = ok
		if !ok || service == "" || c.UpdateStatus == nil || !c.UpdateStatus.UpdateAvailable || inspect.Err() != nil {
			continue
		}
		if !seen[project] {
			seen[project] = true
			snapshot, e := source.snapshot(inspect)
			if e != nil {
				continue
			}
			snapshots[project] = snapshot
		}
		snapshot, ok := snapshots[project]
		if !ok {
			continue
		}
		c.ComposeUpdate, _, _ = source.review(snapshot, service, c.Image, c.UpdateStatus.LatestDigest)
	}
}

// Git receives an ordinary review branch. The deployment branch, working tree
// and repository index remain owned by the configured deployment system.
func (s composeSource) propose(ctx context.Context, snap composeSnapshot, updated []byte, actionID string) (string, string, error) {
	if !composeServiceName.MatchString(actionID) {
		return "", "", errors.New("invalid proposal identity")
	}
	ref := "refs/heads/pulse/service-update/" + actionID
	entry, err := s.git(ctx, nil, nil, "ls-tree", "-z", snap.commit, "--", s.File)
	mode, _, _ := strings.Cut(string(entry), " ")
	if err != nil || (mode != "100644" && mode != "100755") {
		return "", "", errors.New("Git source is not a regular file")
	}
	remoteTip, err := s.git(ctx, nil, nil, "ls-remote", "--refs", s.Remote, "refs/heads/"+s.Branch)
	if fields := strings.Fields(string(remoteTip)); err != nil || len(fields) != 2 || fields[0] != snap.commit {
		return "", "", errors.New("Git base changed or could not be verified")
	}
	dir, err := os.MkdirTemp("", "pulse-compose-index-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
	blob, err := s.git(ctx, env, updated, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", "", errors.New("could not prepare Git image change")
	}
	if _, err = s.git(ctx, env, nil, "read-tree", snap.commit); err != nil {
		return "", "", errors.New("could not read Git base tree")
	}
	if _, err = s.git(ctx, env, nil, "update-index", "--cacheinfo", mode, strings.TrimSpace(string(blob)), s.File); err != nil {
		return "", "", errors.New("could not update Git source entry")
	}
	tree, err := s.git(ctx, env, nil, "write-tree")
	if err != nil {
		return "", "", errors.New("could not write proposal tree")
	}
	commit, err := s.git(ctx, env, nil, "commit-tree", strings.TrimSpace(string(tree)), "-p", snap.commit, "-m", "Update service image in "+s.Project)
	if err != nil {
		return "", "", errors.New("could not create proposal commit with the registered Git identity")
	}
	id := strings.TrimSpace(string(commit))
	if _, err = s.git(ctx, env, nil, "push", "--force-with-lease="+ref+":", s.Remote, id+":"+ref); err != nil {
		return "", "", errComposePublishUnconfirmed
	}
	return strings.TrimPrefix(ref, "refs/heads/"), id, nil
}

func (a *Agent) registeredComposeOwner(labels map[string]string) bool {
	project := labels["com.docker.compose.project"]
	if a.cfg.ComposeSourcesFile == "" || project == "" {
		return false
	}
	sources, err := loadComposeSources(a.cfg.ComposeSourcesFile)
	_, registered := sources[project]
	return err != nil || registered
}

type reviewedComposeService struct {
	source   composeSource
	snapshot composeSnapshot
	updated  []byte
	endpoint string
}

func (p reviewedComposeService) compose(ctx context.Context, file string, args ...string) ([]byte, error) {
	dir := filepath.Dir(p.source.File)
	base := []string{"--host", p.endpoint, "compose", "--project-directory", dir, "--project-name", p.source.Project, "--file", file}
	return deploymentCommand(ctx, dir, nil, nil, "docker", append(base, args...)...)
}

func (p reviewedComposeService) serviceID(ctx context.Context, service string) (string, error) {
	output, err := p.compose(ctx, p.source.File, "ps", "--all", "--quiet", "--no-trunc", service)
	if err != nil {
		return "", errors.New("could not read the registered Compose service")
	}
	fields := strings.Fields(string(output))
	if len(fields) != 1 || !fullComposeContainerID.MatchString(fields[0]) {
		return "", errors.New("source update requires exactly one service container")
	}
	return fields[0], nil
}

var fullComposeContainerID = regexp.MustCompile("^[0-9a-f]{64}$")

func (a *Agent) reviewComposeService(ctx context.Context, req agentexec.DockerContainerUpdatePayload) (reviewedComposeService, error) {
	var plan reviewedComposeService
	if err := agentexec.ValidateDockerContainerUpdatePayload(&req); err != nil || req.Compose == nil || !composeServiceName.MatchString(req.ActionID) {
		return plan, errors.New("invalid reviewed service update")
	}
	if err := a.preflightContainerImage(ctx, req.Runtime, req.ContainerID, req.ExpectedImageDigest, true); err != nil {
		return plan, err
	}
	ct, err := a.docker.ContainerInspect(ctx, req.ContainerID)
	if err != nil || ct.ID != req.ContainerID || ct.Config == nil || ct.State == nil || !ct.State.Running {
		return plan, errors.New("reviewed service container is unavailable or stopped")
	}
	change := req.Compose
	labels := ct.Config.Labels
	if labels["com.docker.compose.project"] != change.Project || labels["com.docker.compose.service"] != change.Service {
		return plan, errors.New("container no longer belongs to the reviewed service")
	}
	sources, err := loadComposeSources(a.cfg.ComposeSourcesFile)
	if err != nil {
		return plan, errors.New("registered sources unavailable")
	}
	source, ok := sources[change.Project]
	if !ok || source.Mode != change.Mode {
		return plan, errors.New("deployment ownership changed")
	}
	snap, err := source.snapshot(ctx)
	if err != nil {
		return plan, err
	}
	_, digest, _ := strings.Cut(change.AfterImage, "@")
	current, updated, err := source.review(snap, change.Service, ct.Config.Image, digest)
	if err != nil || *current != *change {
		return plan, errors.New("source differs from the approved image change")
	}
	plan = reviewedComposeService{source: source, snapshot: snap, updated: updated, endpoint: a.docker.DaemonHost()}
	if source.Mode == "local" {
		info, infoErr := a.docker.Info(ctx)
		if infoErr != nil || buildHostSecurityInfo(info) != nil || a.cfg.HelperInventory != nil {
			return plan, errors.New("direct Docker mutation is unavailable for this runtime")
		}
		if runtime.GOOS != "linux" || !strings.HasPrefix(plan.endpoint, "unix://") {
			return plan, errors.New("local Compose deployment requires a Linux agent on the Docker socket host")
		}
		if labels["com.docker.compose.project.config_files"] != source.File || labels["com.docker.compose.project.working_dir"] != filepath.Dir(source.File) {
			return plan, errors.New("running service was deployed with different files or project directory")
		}
		id, err := plan.serviceID(ctx, change.Service)
		if err != nil || id != req.ContainerID {
			return plan, errors.New("service membership changed or contains multiple replicas")
		}
		hash, err := plan.compose(ctx, source.File, "config", "--hash", change.Service)
		fields := strings.Fields(string(hash))
		if err != nil || len(fields) != 2 || fields[0] != change.Service || fields[1] != labels["com.docker.compose.config-hash"] {
			return plan, errors.New("effective Compose settings differ from the deployed service")
		}
	}
	return plan, nil
}

func (a *Agent) TypedComposeUpdatePreflight(ctx context.Context, req agentexec.DockerContainerUpdatePayload) error {
	_, err := a.reviewComposeService(ctx, req)
	return err
}

func (a *Agent) TypedComposeUpdate(ctx context.Context, req agentexec.DockerContainerUpdatePayload) (agentexec.DockerContainerUpdateOutcome, error) {
	plan, err := a.reviewComposeService(ctx, req)
	if err != nil {
		return agentexec.DockerContainerUpdateOutcome{}, err
	}
	out := agentexec.DockerContainerUpdateOutcome{OldContainerID: req.ContainerID, OldImageDigest: req.ExpectedImageDigest, ContainerName: req.Compose.Service}
	if plan.source.Mode == "git" {
		branch, commit, err := plan.source.propose(ctx, plan.snapshot, plan.updated, req.ActionID)
		if err != nil {
			out.SourceStage = "git_prepare"
			if errors.Is(err, errComposePublishUnconfirmed) {
				out.SourceMutationStarted = true
				out.SourceStage = "git_publish"
			}
			out.Error = err.Error()
			return out, nil
		}
		out.SourceStage = "git_proposed"
		out.ProposalBranch = branch
		out.ProposalCommit = commit
		out.SourceMutationStarted = true
		out.Success = true
		return out, nil
	}
	return a.applyLocalCompose(ctx, req, plan, out), nil
}

var errComposePublishUnconfirmed = errors.New("proposal publication unconfirmed; check the review branch before retrying")

func (a *Agent) applyLocalCompose(ctx context.Context, req agentexec.DockerContainerUpdatePayload, plan reviewedComposeService, out agentexec.DockerContainerUpdateOutcome) agentexec.DockerContainerUpdateOutcome {
	source := plan.source.File
	dir := filepath.Dir(source)
	lock := source + ".pulse-update-lock"
	out.SourceStage = "source_review"
	// Cooperating agents share the same lock. An interrupted agent leaves a
	// visible lock for the operator to inspect before admitting another write.
	if err := os.Mkdir(lock, 0700); err != nil {
		out.Error = "another source update owns the Compose file lock"
		return out
	}
	defer os.Remove(lock)
	temp, err := os.CreateTemp(dir, ".pulse-compose-*.yaml")
	if err != nil {
		out.Error = "could not stage the reviewed Compose file"
		return out
	}
	staged := temp.Name()
	defer os.Remove(staged)
	if err = temp.Close(); err != nil {
		out.Error = "could not close staging file"
		return out
	}
	// Preserve file identity attributes, including owner, ACL and SELinux
	// context, before atomically publishing the new content in this directory.
	if _, err = deploymentCommand(ctx, dir, nil, nil, "cp", "--preserve=all", "--", source, staged); err != nil {
		out.Error = "could not preserve Compose source attributes"
		return out
	}
	if err = writeStagedCompose(staged, plan.updated); err != nil {
		out.Error = "could not stage the reviewed Compose change"
		return out
	}
	rendered, err := plan.compose(ctx, staged, "config", "--images", req.Compose.Service)
	if err != nil || strings.TrimSpace(string(rendered)) != req.Compose.AfterImage {
		out.Error = "staged service did not render the reviewed image"
		return out
	}
	out.SourceStage = "image_pull"
	if err = a.pullUpdateImage(ctx, req.Compose.AfterImage); err != nil {
		out.Error = "pulling the reviewed image digest failed"
		out.ReasonCode = agentexec.ActionRefusalImagePullFailed
		return out
	}
	if _, err = a.reviewComposeService(ctx, req); err != nil {
		out.Error = "source or service changed before deployment"
		return out
	}
	out.SourceStage = "source_write"
	backup := source + ".pulse-before-" + req.ActionID
	if err = os.Link(source, backup); err != nil {
		out.Error = "could not retain the previous Compose source for recovery"
		return out
	}
	if err = os.Rename(staged, source); err != nil {
		_ = os.Remove(backup)
		out.Error = "could not install the reviewed Compose source"
		return out
	}
	out.SourceMutationStarted = true
	if err = syncComposeDirectory(dir); err != nil {
		out.Error = "source installation durability unconfirmed; inspect the registered file and retained previous source"
		return out
	}
	out.SourceStage = "service_recreate"
	if _, err = plan.compose(ctx, source, "up", "--detach", "--no-deps", "--no-build", "--pull", "never", req.Compose.Service); err != nil {
		out.Error = "source changed; Compose could not confirm service deployment"
		return out
	}
	out.SourceStage = "service_readback"
	id, err := plan.serviceID(ctx, req.Compose.Service)
	if err != nil || id == req.ContainerID {
		out.Error = "replacement service container was not observed"
		return out
	}
	inspect, err := a.docker.ContainerInspect(ctx, id)
	if err != nil || inspect.ID != id || inspect.State == nil || !inspect.State.Running || inspect.Config == nil || inspect.Config.Labels["com.docker.compose.project"] != req.Compose.Project || inspect.Config.Labels["com.docker.compose.service"] != req.Compose.Service {
		out.Error = "replacement container running state could not be confirmed"
		return out
	}
	_, want, _ := strings.Cut(req.Compose.AfterImage, "@")
	digests, _, _, _ := a.getImageRepoDigests(ctx, inspect.Image, inspect.Config.Image)
	if !slices.Contains(digests, want) {
		out.Error = "running replacement image differs from the approved manifest"
		return out
	}
	out.NewContainerID = id
	out.NewImageDigest = want
	out.Success = true
	return out
}

func writeStagedCompose(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	return errors.Join(err, closeErr)
}
func syncComposeDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
