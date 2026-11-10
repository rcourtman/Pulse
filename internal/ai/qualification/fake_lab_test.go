package qualification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// fakeDocker is a small in-memory Docker daemon and lab host. It answers the
// exact commands the lab driver sends, keeps the state they change, and
// behaves like the fixtures do, so a test can drive the real driver and the
// real response runner end to end without Docker, Pulse or a model.
type fakeDocker struct {
	mu         sync.Mutex
	calls      []string
	containers map[string]*fakeContainer
	volumes    map[string]map[string]string
	networks   map[string]map[string]string
	images     map[string]string
	endpoint   string
	seq        int
	host       *fakeHostFilesystem
}

type fakeContainer struct {
	ID           string
	Name         string
	Image        string
	Running      bool
	Health       string
	RestartCount int
	ExitCode     int
	Labels       map[string]string
	Env          map[string]string
	Mounts       []string
	Tmpfs        []string
	Restart      string
	Network      string
}

// fakeHostFilesystem is a dedicated 4 KiB-block filesystem with ext4's 5%
// reserve, mounted or not.
type fakeHostFilesystem struct {
	MountPath string
	TotalMiB  int
	Mounted   bool
	Files     map[string]int
}

func newFakeDocker() *fakeDocker {
	fake := &fakeDocker{
		containers: make(map[string]*fakeContainer), volumes: make(map[string]map[string]string),
		networks: make(map[string]map[string]string), images: map[string]string{fixtureBaseImage: ""},
		endpoint: "unix:///run/user/1000/docker.sock",
	}
	for _, fixture := range fixtureCatalogue {
		fake.images[fixture.Image] = fixture.buildDigest()
	}
	return fake
}

func (f *fakeDocker) container(name string) *fakeContainer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.containers[name]
}

func (f *fakeDocker) called(fragment string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if strings.Contains(call, fragment) {
			count++
		}
	}
	return count
}

func (f *fakeDocker) Run(_ context.Context, name string, args ...string) (CommandResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	if name != "docker" {
		return f.hostCommand(name, args)
	}
	if len(args) >= 2 && args[0] == "--context" {
		args = args[2:]
	}
	if len(args) == 0 {
		return CommandResult{}, errors.New("empty docker command")
	}
	switch args[0] {
	case "context":
		return CommandResult{Stdout: f.endpoint + "\n"}, nil
	case "ps":
		return f.list(args, func(yield func(id string, labels map[string]string)) {
			for _, container := range f.containers {
				yield(container.ID, container.Labels)
			}
		}), nil
	case "volume":
		return f.objects(args[1:], f.volumes)
	case "network":
		if args[1] == "connect" || args[1] == "disconnect" {
			return CommandResult{}, nil
		}
		return f.objects(args[1:], f.networks)
	case "image":
		switch args[1] {
		case "ls":
			var ids []string
			for tag := range f.images {
				ids = append(ids, "sha256:"+tag)
			}
			sort.Strings(ids)
			return CommandResult{Stdout: strings.Join(ids, "\n")}, nil
		case "inspect":
			label, ok := f.images[args[len(args)-1]]
			if !ok {
				return CommandResult{ExitCode: 1}, errors.New("no such image")
			}
			return CommandResult{Stdout: label + "\n"}, nil
		}
	case "build":
		tag, label := "", ""
		for i := 1; i < len(args)-1; i++ {
			switch args[i] {
			case "-t":
				tag = args[i+1]
			case "--label":
				_, label, _ = strings.Cut(args[i+1], "=")
			}
		}
		f.images[tag] = label
		return CommandResult{Stdout: "sha256:built\n"}, nil
	case "run":
		return f.run(args[1:])
	case "inspect":
		container := f.find(args[1])
		if container == nil {
			return CommandResult{ExitCode: 1}, errors.New("no such container")
		}
		return CommandResult{Stdout: f.inspectJSON(container)}, nil
	case "exec":
		container := f.find(args[1])
		if container == nil || !container.Running {
			return CommandResult{ExitCode: 1}, errors.New("container is not running")
		}
		switch strings.Join(args[2:], " ") {
		case "pkill httpd":
			container.Health = "unhealthy"
		case "touch /run/flags/crash":
			container.Running, container.ExitCode = false, 139
		default:
			return CommandResult{ExitCode: 1}, fmt.Errorf("unexpected exec %v", args[2:])
		}
		return CommandResult{}, nil
	case "restart", "start":
		container := f.find(args[len(args)-1])
		if container == nil {
			return CommandResult{ExitCode: 1}, errors.New("no such container")
		}
		container.Running, container.ExitCode = true, 0
		if container.Health != "" {
			container.Health = "healthy"
		}
		return CommandResult{}, nil
	case "stop":
		container := f.find(args[len(args)-1])
		if container == nil {
			return CommandResult{ExitCode: 1}, errors.New("no such container")
		}
		container.Running, container.ExitCode = false, 137
		return CommandResult{}, nil
	case "rm":
		container := f.find(args[len(args)-1])
		if container == nil {
			return CommandResult{ExitCode: 1}, errors.New("no such container")
		}
		delete(f.containers, container.Name)
		return CommandResult{}, nil
	}
	return CommandResult{ExitCode: 1}, fmt.Errorf("fake docker does not implement %v", args)
}

func (f *fakeDocker) find(nameOrID string) *fakeContainer {
	if container, ok := f.containers[nameOrID]; ok {
		return container
	}
	for _, container := range f.containers {
		if container.ID == nameOrID {
			return container
		}
	}
	return nil
}

func (f *fakeDocker) list(args []string, each func(func(id string, labels map[string]string))) CommandResult {
	wantKey, wantValue := "", ""
	for i, arg := range args {
		if arg == "--filter" && i+1 < len(args) {
			wantKey, wantValue, _ = strings.Cut(strings.TrimPrefix(args[i+1], "label="), "=")
		}
	}
	var ids []string
	each(func(id string, labels map[string]string) {
		if wantKey == "" || labels[wantKey] == wantValue {
			ids = append(ids, id)
		}
	})
	sort.Strings(ids)
	return CommandResult{Stdout: strings.Join(ids, "\n")}
}

func (f *fakeDocker) objects(args []string, store map[string]map[string]string) (CommandResult, error) {
	switch args[0] {
	case "ls":
		return f.list(args, func(yield func(id string, labels map[string]string)) {
			for name, labels := range store {
				yield(name, labels)
			}
		}), nil
	case "create":
		labels := make(map[string]string)
		for i := 1; i < len(args)-1; i++ {
			if args[i] == "--label" {
				key, value, _ := strings.Cut(args[i+1], "=")
				labels[key] = value
			}
		}
		name := args[len(args)-1]
		if _, exists := store[name]; !exists {
			store[name] = labels
		}
		return CommandResult{Stdout: name + "\n"}, nil
	case "rm":
		delete(store, args[len(args)-1])
		return CommandResult{}, nil
	}
	return CommandResult{ExitCode: 1}, fmt.Errorf("fake docker does not implement %v", args)
}

func (f *fakeDocker) run(args []string) (CommandResult, error) {
	container := &fakeContainer{Labels: make(map[string]string), Env: make(map[string]string), Running: true}
	index := 0
	for ; index < len(args); index++ {
		flag := args[index]
		if !strings.HasPrefix(flag, "-") {
			break
		}
		if flag == "-d" || flag == "--rm" {
			continue
		}
		value := args[index+1]
		index++
		switch flag {
		case "--name":
			container.Name = value
		case "--network":
			container.Network = value
		case "--label":
			key, labelValue, _ := strings.Cut(value, "=")
			container.Labels[key] = labelValue
		case "--env":
			key, envValue, _ := strings.Cut(value, "=")
			container.Env[key] = envValue
		case "--mount":
			container.Mounts = append(container.Mounts, value)
		case "--tmpfs":
			container.Tmpfs = append(container.Tmpfs, value)
		case "--restart":
			container.Restart = value
		}
	}
	if index >= len(args) {
		return CommandResult{ExitCode: 1}, errors.New("docker run without an image")
	}
	container.Image = args[index]
	if _, exists := f.containers[container.Name]; exists {
		return CommandResult{ExitCode: 125}, fmt.Errorf("container name %q is already in use", container.Name)
	}
	if _, ok := f.images[container.Image]; !ok {
		return CommandResult{ExitCode: 125}, fmt.Errorf("no such image %s", container.Image)
	}
	f.seq++
	container.ID = fmt.Sprintf("%064x", f.seq)
	for _, fixture := range fixtureCatalogue {
		if fixture.Image != container.Image {
			continue
		}
		if fixture.Healthcheck != "" {
			container.Health = "healthy"
		}
		switch fixture.Key {
		case "flyway-migrate":
			container.Running = false
		case "vaultwarden":
			if !strings.HasPrefix(container.Env["DATABASE_URL"], "postgresql://") {
				container.RestartCount = 5
			}
		}
	}
	f.containers[container.Name] = container
	return CommandResult{Stdout: container.ID + "\n"}, nil
}

// health mirrors the one real dependency in the stack: the app is healthy
// only while its database writes a heartbeat.
func (f *fakeDocker) health(container *fakeContainer) string {
	if container.Image == "nextcloud:30-apache" && container.Health != "" {
		if database := f.containers["nextcloud-db"]; database == nil || !database.Running {
			return "unhealthy"
		}
	}
	return container.Health
}

func (f *fakeDocker) inspectJSON(container *fakeContainer) string {
	state := map[string]any{"Status": "running", "Running": container.Running, "ExitCode": container.ExitCode}
	if !container.Running {
		state["Status"] = "exited"
	}
	if health := f.health(container); health != "" {
		state["Health"] = map[string]any{"Status": health}
	}
	payload, _ := json.Marshal([]map[string]any{{
		"Id": container.ID, "Name": "/" + container.Name, "Config": map[string]any{"Labels": container.Labels},
		"State": state, "RestartCount": container.RestartCount,
		"NetworkSettings": map[string]any{"Networks": map[string]any{container.Network: map[string]any{}}},
	}})
	return string(payload)
}

func (h *fakeHostFilesystem) blocks() (blocks, free, available int64) {
	blocks = int64(h.TotalMiB) * 256
	used := int64(0)
	for _, mib := range h.Files {
		used += int64(mib) * 256
	}
	free = blocks - used
	available = free - blocks*5/100
	if available < 0 {
		available = 0
	}
	return blocks, free, available
}

func (f *fakeDocker) hostCommand(name string, args []string) (CommandResult, error) {
	host := f.host
	if host == nil {
		return CommandResult{ExitCode: 1}, fmt.Errorf("fake lab has no host filesystem for %s", name)
	}
	inside := func(target string) bool { return strings.HasPrefix(target, host.MountPath+"/") }
	switch name {
	case "stat":
		if args[0] == "-f" {
			blocks, free, available := host.blocks()
			if !host.Mounted {
				// An unmounted directory reports the disk it sits on.
				blocks, free, available = 64<<20, 32<<20, 30<<20
			}
			return CommandResult{Stdout: fmt.Sprintf("4096 %d %d %d\n", blocks, free, available)}, nil
		}
		if host.Mounted {
			return CommandResult{Stdout: "1794\n2049\n"}, nil
		}
		return CommandResult{Stdout: "2049\n2049\n"}, nil
	case "test":
		if _, ok := host.Files[args[1]]; ok {
			return CommandResult{}, nil
		}
		return CommandResult{ExitCode: 1}, errors.New("no such file")
	case "mkdir":
		if !inside(args[len(args)-1]) {
			return CommandResult{ExitCode: 1}, fmt.Errorf("mkdir outside the lab mount: %v", args)
		}
		return CommandResult{}, nil
	case "dd":
		target, count := "", 0
		for _, arg := range args {
			if value, ok := strings.CutPrefix(arg, "of="); ok {
				target = value
			}
			if value, ok := strings.CutPrefix(arg, "count="); ok {
				count, _ = strconv.Atoi(value)
			}
		}
		if !inside(target) {
			return CommandResult{ExitCode: 1}, fmt.Errorf("write outside the lab mount: %s", target)
		}
		if _, exists := host.Files[target]; exists {
			return CommandResult{ExitCode: 1}, errors.New("File exists")
		}
		if _, _, available := host.blocks(); available < int64(count)*256 {
			return CommandResult{ExitCode: 1}, errors.New("No space left on device")
		}
		host.Files[target] = count
		return CommandResult{}, nil
	case "rm":
		if !inside(args[len(args)-1]) {
			return CommandResult{ExitCode: 1}, fmt.Errorf("remove outside the lab mount: %v", args)
		}
		delete(host.Files, args[len(args)-1])
		return CommandResult{}, nil
	case "rmdir":
		for file := range host.Files {
			if strings.HasPrefix(file, args[0]+"/") {
				return CommandResult{ExitCode: 1}, errors.New("Directory not empty")
			}
		}
		if !inside(args[0]) || path.Dir(args[0]) == host.MountPath {
			return CommandResult{ExitCode: 1}, fmt.Errorf("rmdir reached the top of the lab mount: %s", args[0])
		}
		return CommandResult{}, nil
	}
	return CommandResult{ExitCode: 1}, fmt.Errorf("fake host does not implement %s %v", name, args)
}
