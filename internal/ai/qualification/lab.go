package qualification

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	labRunLabel      = "io.pulse.owner"
	labScenarioLabel = "io.pulse.profile"
	labAliasLabel    = "io.pulse.component"
)

type CommandResult struct {
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ns"`
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(start)}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("%s failed: %w: %s", commandSummary(name, args), err, sanitizeArtifactText(stderr.String()))
	}
	return result, nil
}

func commandSummary(name string, args []string) string {
	visible := args
	if len(visible) > 6 {
		visible = visible[:6]
	}
	return strings.Join(append([]string{name}, visible...), " ")
}

type DockerTarget struct {
	Context         string
	SSHHost         string
	AllowSharedHost bool
}

func (t DockerTarget) Validate(manifest Manifest) error {
	if t.Context != "" && t.SSHHost != "" {
		return errors.New("docker context and SSH host are mutually exclusive")
	}
	if t.SSHHost != "" && (!t.AllowSharedHost || !manifest.Lab.SharedHostOK) {
		return errors.New("shared Docker host requires both manifest shared_host_ok and --allow-shared-host")
	}
	if t.Context == "" && t.SSHHost == "" {
		return errors.New("an explicit Docker context or SSH host is required")
	}
	return nil
}

type DockerInventory struct {
	Containers []string `json:"containers"`
	Volumes    []string `json:"volumes"`
	Networks   []string `json:"networks"`
	Images     []string `json:"images"`
}

type DockerState struct {
	Alias        string            `json:"alias"`
	Name         string            `json:"name"`
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	Running      bool              `json:"running"`
	Health       string            `json:"health,omitempty"`
	RestartCount int               `json:"restart_count"`
	ExitCode     int               `json:"exit_code"`
	Labels       map[string]string `json:"labels,omitempty"`
	Networks     []string          `json:"networks,omitempty"`
}

type PreparedLab struct {
	RunID             string                 `json:"run_id"`
	ScenarioID        string                 `json:"scenario_id"`
	NetworkName       string                 `json:"network_name"`
	ResourceNames     map[string]string      `json:"resource_names"`
	ResourceIDs       map[string]string      `json:"resource_ids"`
	FaultVolumes      map[string]string      `json:"fault_volumes,omitempty"`
	PreInventory      DockerInventory        `json:"pre_inventory"`
	BaselineStates    map[string]DockerState `json:"baseline_states"`
	ExpectedInventory DockerInventory        `json:"expected_inventory"`
	AppliedFaults     []string               `json:"applied_faults,omitempty"`
	mu                sync.Mutex
}

type PredicateObservation struct {
	Predicate Predicate    `json:"predicate"`
	Observed  any          `json:"observed,omitempty"`
	Passed    bool         `json:"passed"`
	CheckedAt time.Time    `json:"checked_at"`
	Error     string       `json:"error,omitempty"`
	State     *DockerState `json:"state,omitempty"`
}

type CleanupResult struct {
	FirstRemoved       DockerInventory `json:"first_removed"`
	SecondRemoved      DockerInventory `json:"second_removed"`
	PostInventory      DockerInventory `json:"post_inventory"`
	SecondCleanupNoop  bool            `json:"second_cleanup_noop"`
	InventoryUnchanged bool            `json:"inventory_unchanged"`
	Passed             bool            `json:"passed"`
	Errors             []string        `json:"errors,omitempty"`
}

type DockerLab struct {
	runner CommandRunner
	target DockerTarget
}

func NewDockerLab(runner CommandRunner, target DockerTarget) *DockerLab {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return &DockerLab{runner: runner, target: target}
}

func (l *DockerLab) docker(ctx context.Context, args ...string) (CommandResult, error) {
	if l.target.SSHHost != "" {
		quoted := make([]string, 0, len(args)+1)
		quoted = append(quoted, "docker")
		for _, arg := range args {
			quoted = append(quoted, shellQuote(arg))
		}
		sshArgs := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=15", l.target.SSHHost, strings.Join(quoted, " ")}
		return l.runner.Run(ctx, "ssh", sshArgs...)
	}
	return l.runner.Run(ctx, "docker", append([]string{"--context", l.target.Context}, args...)...)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (l *DockerLab) Prepare(ctx context.Context, manifest Manifest, runID string) (*PreparedLab, error) {
	if err := l.target.Validate(manifest); err != nil {
		return nil, err
	}
	if !safeID.MatchString(runID) {
		return nil, errors.New("unsafe lab run id")
	}
	pre, err := l.inventory(ctx)
	if err != nil {
		return nil, fmt.Errorf("capture pre-lab inventory: %w", err)
	}
	lab := &PreparedLab{
		RunID:          runID,
		ScenarioID:     manifest.ID,
		NetworkName:    "backend-" + labRunToken(runID),
		ResourceNames:  make(map[string]string, len(manifest.Resources)),
		ResourceIDs:    make(map[string]string, len(manifest.Resources)),
		FaultVolumes:   make(map[string]string),
		PreInventory:   pre,
		BaselineStates: make(map[string]DockerState),
	}
	labels := []string{"--label", labRunLabel + "=" + labRunToken(runID), "--label", labScenarioLabel + "=" + labOpaqueToken("scenario", manifest.ID)}
	if _, err := l.docker(ctx, append([]string{"network", "create"}, append(labels, lab.NetworkName)...)...); err != nil {
		return lab, fmt.Errorf("create lab network: %w", err)
	}
	for _, resource := range manifest.Resources {
		name, err := renderResourceName(resource.Name, resource.Alias, runID)
		if err != nil {
			return lab, err
		}
		lab.ResourceNames[resource.Alias] = name
		if resource.FaultVolume {
			volume := name + "-fault"
			if _, err := l.docker(ctx, append([]string{"volume", "create"}, append(labels, volume)...)...); err != nil {
				return lab, fmt.Errorf("create fault volume for %s: %w", resource.Alias, err)
			}
			lab.FaultVolumes[resource.Alias] = volume
		}
		image := resource.Image
		if image == "" {
			image = manifest.Lab.Image
		}
		if !manifest.Lab.AllowPull {
			if _, err := l.docker(ctx, "image", "inspect", image); err != nil {
				return lab, fmt.Errorf("required pre-existing image %q is unavailable and allow_pull is false", image)
			}
		}
		args := []string{"run", "-d", "--name", name, "--network", lab.NetworkName}
		args = append(args, labels...)
		args = append(args, "--label", labAliasLabel+"="+resource.Alias)
		for key, value := range resource.Labels {
			args = append(args, "--label", key+"="+renderText(value, resource.Alias, runID))
		}
		if resource.Restart != "" {
			args = append(args, "--restart", resource.Restart)
		}
		if volume := lab.FaultVolumes[resource.Alias]; volume != "" {
			args = append(args, "--mount", "source="+volume+",target=/pulse-qual-fault")
		}
		if len(resource.Healthcheck) > 0 {
			interval := resource.HealthEvery
			if interval == "" {
				interval = "2s"
			}
			healthcheck := make([]string, len(resource.Healthcheck))
			for i, part := range resource.Healthcheck {
				healthcheck[i] = renderText(part, resource.Alias, runID)
			}
			args = append(args, "--health-cmd", strings.Join(healthcheck, " "), "--health-interval", interval, "--health-timeout", "2s", "--health-retries", "2", "--health-start-period", "1s")
		}
		args = append(args, image)
		for _, part := range resource.Command {
			args = append(args, renderText(part, resource.Alias, runID))
		}
		result, err := l.docker(ctx, args...)
		if err != nil {
			return lab, fmt.Errorf("start resource %s: %w", resource.Alias, err)
		}
		lab.ResourceIDs[resource.Alias] = strings.TrimSpace(result.Stdout)
	}
	if err := l.waitPredicates(ctx, manifest, lab, manifest.Baseline, nil); err != nil {
		return lab, fmt.Errorf("baseline did not converge: %w", err)
	}
	for alias := range lab.ResourceNames {
		state, err := l.inspect(ctx, lab, alias)
		if err != nil {
			return lab, err
		}
		lab.BaselineStates[alias] = state
	}
	lab.ExpectedInventory, err = l.inventory(ctx)
	if err != nil {
		return lab, err
	}
	return lab, nil
}

func renderResourceName(template, alias, runID string) (string, error) {
	name := renderText(template, alias, runID)
	if name == "" {
		name = alias + "-" + labRunToken(runID)
	}
	if (!strings.Contains(name, runID) && !strings.Contains(name, labRunToken(runID))) || !safeID.MatchString(name) {
		return "", fmt.Errorf("resource %q renders unsafe name %q; qualification resources must include the full run id or its opaque run token", alias, name)
	}
	return name, nil
}

func renderText(value, alias, runID string) string {
	value = strings.ReplaceAll(value, "${run_id}", runID)
	value = strings.ReplaceAll(value, "${run_token}", labRunToken(runID))
	return strings.ReplaceAll(value, "${alias}", alias)
}

func labRunToken(runID string) string {
	return labOpaqueToken("run", runID)
}

func labOpaqueToken(namespace, value string) string {
	sum := sha256.Sum256([]byte(namespace + "\x00" + value))
	return fmt.Sprintf("%x", sum[:6])
}

func (l *DockerLab) ApplyFault(ctx context.Context, manifest Manifest, lab *PreparedLab, fault FaultSpec) error {
	lab.mu.Lock()
	defer lab.mu.Unlock()
	if contains(lab.AppliedFaults, fault.ID) {
		return fmt.Errorf("fault %q is already applied", fault.ID)
	}
	resource := fault.Injector.Resource
	name := lab.ResourceNames[resource]
	switch fault.Injector.Kind {
	case "marker_enable":
		if err := l.setMarker(ctx, manifest, lab, resource, true); err != nil {
			return err
		}
	case "stop":
		if _, err := l.docker(ctx, "stop", "--time", "5", name); err != nil {
			return err
		}
	case "disconnect_network":
		if _, err := l.docker(ctx, "network", "disconnect", lab.NetworkName, name); err != nil {
			return err
		}
	case "kill":
		if _, err := l.docker(ctx, "kill", name); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported injector %q", fault.Injector.Kind)
	}
	lab.AppliedFaults = append(lab.AppliedFaults, fault.ID)
	return nil
}

func (l *DockerLab) RevertFault(ctx context.Context, manifest Manifest, lab *PreparedLab, fault FaultSpec) error {
	resource := fault.Injector.Resource
	name := lab.ResourceNames[resource]
	switch fault.Injector.Kind {
	case "marker_enable":
		if err := l.setMarker(ctx, manifest, lab, resource, false); err != nil {
			return err
		}
		_, _ = l.docker(ctx, "start", name)
	case "stop", "kill":
		if _, err := l.docker(ctx, "start", name); err != nil {
			return err
		}
	case "disconnect_network":
		if _, err := l.docker(ctx, "network", "connect", lab.NetworkName, name); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported injector %q", fault.Injector.Kind)
	}
	return nil
}

func (l *DockerLab) setMarker(ctx context.Context, manifest Manifest, lab *PreparedLab, alias string, enabled bool) error {
	volume := lab.FaultVolumes[alias]
	if volume == "" {
		return fmt.Errorf("resource %q has no fault volume", alias)
	}
	image := manifest.Lab.Image
	for _, resource := range manifest.Resources {
		if resource.Alias == alias && resource.Image != "" {
			image = resource.Image
		}
	}
	operation := "touch /pulse-qual-fault/enabled"
	if !enabled {
		operation = "rm -f /pulse-qual-fault/enabled"
	}
	_, err := l.docker(ctx, "run", "--rm", "--label", labRunLabel+"="+labRunToken(lab.RunID), "--mount", "source="+volume+",target=/pulse-qual-fault", image, "/bin/sh", "-c", operation)
	return err
}

func (l *DockerLab) Observe(ctx context.Context, manifest Manifest, lab *PreparedLab, predicates []Predicate) ([]PredicateObservation, error) {
	observations := make([]PredicateObservation, 0, len(predicates))
	for _, predicate := range predicates {
		observation, err := l.waitPredicate(ctx, manifest, lab, predicate)
		observations = append(observations, observation)
		if err != nil {
			return observations, err
		}
	}
	return observations, nil
}

func (l *DockerLab) waitPredicates(ctx context.Context, manifest Manifest, lab *PreparedLab, predicates []Predicate, output *[]PredicateObservation) error {
	observations, err := l.Observe(ctx, manifest, lab, predicates)
	if output != nil {
		*output = append(*output, observations...)
	}
	return err
}

func (l *DockerLab) waitPredicate(ctx context.Context, _ Manifest, lab *PreparedLab, predicate Predicate) (PredicateObservation, error) {
	timeout := 30 * time.Second
	if predicate.Timeout != "" {
		parsed, err := positiveDuration(predicate.Timeout)
		if err != nil {
			return PredicateObservation{Predicate: predicate, Error: err.Error()}, err
		}
		timeout = parsed
	}
	deadline := time.Now().Add(timeout)
	var last PredicateObservation
	for {
		last = PredicateObservation{Predicate: predicate, CheckedAt: time.Now().UTC()}
		observed, state, err := l.probe(ctx, lab, predicate)
		last.Observed, last.State = observed, state
		if err != nil {
			last.Error = err.Error()
		} else {
			passed, compareErr := comparePredicate(observed, predicate.Operator, predicate.Value)
			if compareErr != nil {
				last.Error = compareErr.Error()
			} else if passed {
				last.Passed = true
				return last, nil
			}
		}
		if time.Now().After(deadline) {
			return last, fmt.Errorf("oracle predicate %s %s %s did not converge: observed=%v error=%s", predicate.Probe, predicate.Operator, string(predicate.Value), observed, last.Error)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (l *DockerLab) probe(ctx context.Context, lab *PreparedLab, predicate Predicate) (any, *DockerState, error) {
	if predicate.Probe == "inventory.same_as_pre" {
		inventory, err := l.inventory(ctx)
		return inventoryEqual(inventory, lab.PreInventory), nil, err
	}
	state, err := l.inspect(ctx, lab, predicate.Target)
	if err != nil {
		if predicate.Probe == "docker.exists" {
			return false, nil, nil
		}
		return nil, nil, err
	}
	switch predicate.Probe {
	case "docker.exists":
		return true, &state, nil
	case "docker.status":
		return state.Status, &state, nil
	case "docker.running":
		return state.Running, &state, nil
	case "docker.health":
		return state.Health, &state, nil
	case "docker.restart_count":
		return state.RestartCount, &state, nil
	case "docker.exit_code":
		return state.ExitCode, &state, nil
	case "docker.network_attached":
		return contains(state.Networks, lab.NetworkName), &state, nil
	default:
		return nil, &state, fmt.Errorf("unsupported oracle probe %q", predicate.Probe)
	}
}

func (l *DockerLab) inspect(ctx context.Context, lab *PreparedLab, alias string) (DockerState, error) {
	name := lab.ResourceNames[alias]
	if name == "" {
		return DockerState{}, fmt.Errorf("unknown resource alias %q", alias)
	}
	result, err := l.docker(ctx, "inspect", name)
	if err != nil {
		return DockerState{}, err
	}
	var payload []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State struct {
			Status       string `json:"Status"`
			Running      bool   `json:"Running"`
			RestartCount int    `json:"RestartCount"`
			ExitCode     int    `json:"ExitCode"`
			Health       *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		RestartCount int `json:"RestartCount"`
		Network      struct {
			Networks map[string]json.RawMessage `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil || len(payload) != 1 {
		return DockerState{}, fmt.Errorf("decode Docker inspect for %s: %w", alias, err)
	}
	item := payload[0]
	health := ""
	if item.State.Health != nil {
		health = item.State.Health.Status
	}
	networks := make([]string, 0, len(item.Network.Networks))
	for network := range item.Network.Networks {
		networks = append(networks, network)
	}
	sort.Strings(networks)
	return DockerState{
		Alias: alias, Name: strings.TrimPrefix(item.Name, "/"), ID: item.ID,
		Status: item.State.Status, Running: item.State.Running, Health: health,
		RestartCount: item.RestartCount, ExitCode: item.State.ExitCode,
		Labels: item.Config.Labels, Networks: networks,
	}, nil
}

func comparePredicate(observed any, operator string, raw json.RawMessage) (bool, error) {
	var expected any
	if len(raw) == 0 {
		return false, errors.New("predicate value is required")
	}
	if err := json.Unmarshal(raw, &expected); err != nil {
		return false, err
	}
	switch operator {
	case "eq":
		return fmt.Sprint(observed) == fmt.Sprint(expected), nil
	case "not_eq":
		return fmt.Sprint(observed) != fmt.Sprint(expected), nil
	case "gte", "lte", "gt", "lt":
		left, err := strconv.ParseFloat(fmt.Sprint(observed), 64)
		if err != nil {
			return false, err
		}
		right, err := strconv.ParseFloat(fmt.Sprint(expected), 64)
		if err != nil {
			return false, err
		}
		switch operator {
		case "gte":
			return left >= right, nil
		case "lte":
			return left <= right, nil
		case "gt":
			return left > right, nil
		default:
			return left < right, nil
		}
	case "in":
		values, ok := expected.([]any)
		if !ok {
			return false, errors.New("in predicate requires array value")
		}
		for _, value := range values {
			if fmt.Sprint(observed) == fmt.Sprint(value) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported predicate operator %q", operator)
	}
}

func (l *DockerLab) inventory(ctx context.Context) (DockerInventory, error) {
	read := func(args ...string) ([]string, error) {
		result, err := l.docker(ctx, args...)
		if err != nil {
			return nil, err
		}
		values := strings.Fields(result.Stdout)
		sort.Strings(values)
		return values, nil
	}
	containers, err := read("ps", "-aq", "--no-trunc")
	if err != nil {
		return DockerInventory{}, err
	}
	volumes, err := read("volume", "ls", "-q")
	if err != nil {
		return DockerInventory{}, err
	}
	networks, err := read("network", "ls", "-q", "--no-trunc")
	if err != nil {
		return DockerInventory{}, err
	}
	images, err := read("image", "ls", "-q", "--no-trunc")
	if err != nil {
		return DockerInventory{}, err
	}
	images = uniqueStrings(images)
	return DockerInventory{Containers: containers, Volumes: volumes, Networks: networks, Images: images}, nil
}

func inventoryEqual(a, b DockerInventory) bool {
	return strings.Join(a.Containers, "\x00") == strings.Join(b.Containers, "\x00") &&
		strings.Join(a.Volumes, "\x00") == strings.Join(b.Volumes, "\x00") &&
		strings.Join(a.Networks, "\x00") == strings.Join(b.Networks, "\x00") &&
		strings.Join(a.Images, "\x00") == strings.Join(b.Images, "\x00")
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func (l *DockerLab) Cleanup(ctx context.Context, manifest Manifest, lab *PreparedLab) CleanupResult {
	cleanup := func() (DockerInventory, []string) {
		var removed DockerInventory
		var errs []string
		label := labRunLabel + "=" + labRunToken(lab.RunID)
		if result, err := l.docker(ctx, "ps", "-aq", "--filter", "label="+label); err == nil {
			for _, id := range strings.Fields(result.Stdout) {
				if _, err := l.docker(ctx, "rm", "-f", id); err != nil {
					errs = append(errs, err.Error())
				} else {
					removed.Containers = append(removed.Containers, id)
				}
			}
		} else {
			errs = append(errs, err.Error())
		}
		if result, err := l.docker(ctx, "volume", "ls", "-q", "--filter", "label="+label); err == nil {
			for _, name := range strings.Fields(result.Stdout) {
				if _, err := l.docker(ctx, "volume", "rm", name); err != nil {
					errs = append(errs, err.Error())
				} else {
					removed.Volumes = append(removed.Volumes, name)
				}
			}
		} else {
			errs = append(errs, err.Error())
		}
		if result, err := l.docker(ctx, "network", "ls", "-q", "--filter", "label="+label); err == nil {
			for _, id := range strings.Fields(result.Stdout) {
				if _, err := l.docker(ctx, "network", "rm", id); err != nil {
					errs = append(errs, err.Error())
				} else {
					removed.Networks = append(removed.Networks, id)
				}
			}
		} else {
			errs = append(errs, err.Error())
		}
		return removed, errs
	}
	first, errs := cleanup()
	second, secondErrs := cleanup()
	errs = append(errs, secondErrs...)
	post, err := l.inventory(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	}
	secondNoop := len(second.Containers)+len(second.Volumes)+len(second.Networks) == 0
	unchanged := inventoryEqual(post, lab.PreInventory)
	passed := len(errs) == 0
	if manifest.Teardown.RequireSecondNoop && !secondNoop {
		passed = false
		errs = append(errs, "second cleanup was not a no-op")
	}
	if manifest.Teardown.RequireInventorySame && !unchanged {
		passed = false
		errs = append(errs, "post-lab Docker inventory differs from pre-lab inventory")
	}
	return CleanupResult{FirstRemoved: first, SecondRemoved: second, PostInventory: post, SecondCleanupNoop: secondNoop, InventoryUnchanged: unchanged, Passed: passed, Errors: errs}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
