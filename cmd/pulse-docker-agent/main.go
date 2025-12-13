package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
)

type stringFlagList []string

func (l *stringFlagList) String() string {
	return strings.Join(*l, ",")
}

func (l *stringFlagList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func (l stringFlagList) Values() []string {
	if len(l) == 0 {
		return nil
	}
	return append([]string(nil), l...)
}

func main() {
	// Handle --version flag early before other config parsing
	versionFlag := false
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "version" {
			versionFlag = true
			break
		}
	}

	if versionFlag {
		fmt.Printf("pulse-docker-agent version %s\n", dockeragent.Version)
		os.Exit(0)
	}

	cfg := loadConfig()

	zerolog.SetGlobalLevel(cfg.LogLevel)

	logger := zerolog.New(os.Stdout).Level(cfg.LogLevel).With().Timestamp().Logger()
	cfg.Logger = &logger

	// Deprecation warning
	logger.Warn().Msg("pulse-docker-agent is DEPRECATED and will be removed in a future release")
	logger.Warn().Msg("Please migrate to the unified 'pulse-agent' with --enable-docker flag")
	logger.Warn().Msg("Example: pulse-agent --url <URL> --token <TOKEN> --enable-docker")
	logger.Warn().Msg("")

	agent, err := dockeragent.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create docker agent")
	}
	defer agent.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info().Str("pulse_url", cfg.PulseURL).Dur("interval", cfg.Interval).Msg("Starting Pulse Docker agent")

	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatal().Err(err).Msg("Agent terminated with error")
	}

	logger.Info().Msg("Agent stopped")
}

func loadConfig() dockeragent.Config {
	envURL := utils.GetenvTrim("PULSE_URL")
	envToken := utils.GetenvTrim("PULSE_TOKEN")
	envInterval := utils.GetenvTrim("PULSE_INTERVAL")
	envHostname := utils.GetenvTrim("PULSE_HOSTNAME")
	envAgentID := utils.GetenvTrim("PULSE_AGENT_ID")
	envInsecure := utils.GetenvTrim("PULSE_INSECURE_SKIP_VERIFY")
	envNoAutoUpdate := utils.GetenvTrim("PULSE_NO_AUTO_UPDATE")
	envTargets := utils.GetenvTrim("PULSE_TARGETS")
	envRuntime := utils.GetenvTrim("PULSE_RUNTIME")
	envContainerStates := utils.GetenvTrim("PULSE_CONTAINER_STATES")
	envSwarmScope := utils.GetenvTrim("PULSE_SWARM_SCOPE")
	envSwarmServices := utils.GetenvTrim("PULSE_SWARM_SERVICES")
	envSwarmTasks := utils.GetenvTrim("PULSE_SWARM_TASKS")
	envIncludeContainers := utils.GetenvTrim("PULSE_INCLUDE_CONTAINERS")
	envCollectDisk := utils.GetenvTrim("PULSE_COLLECT_DISK")
	envLogLevel := utils.GetenvTrim("LOG_LEVEL")

	defaultInterval := 30 * time.Second
	if envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			defaultInterval = parsed
		}
	}

	swarmScopeDefault := envSwarmScope
	if swarmScopeDefault == "" {
		swarmScopeDefault = "node"
	}

	includeServicesDefault := true
	if envSwarmServices != "" {
		includeServicesDefault = utils.ParseBool(envSwarmServices)
	}

	includeTasksDefault := true
	if envSwarmTasks != "" {
		includeTasksDefault = utils.ParseBool(envSwarmTasks)
	}

	includeContainersDefault := true
	if envIncludeContainers != "" {
		includeContainersDefault = utils.ParseBool(envIncludeContainers)
	}

	collectDiskDefault := true
	if envCollectDisk != "" {
		collectDiskDefault = utils.ParseBool(envCollectDisk)
	}

	logLevelDefault := "info"
	if envLogLevel != "" {
		logLevelDefault = envLogLevel
	}

	urlFlag := flag.String("url", envURL, "Pulse server URL (e.g. http://pulse:7655)")
	tokenFlag := flag.String("token", envToken, "Pulse API token (required)")
	intervalFlag := flag.Duration("interval", defaultInterval, "Reporting interval (e.g. 30s)")
	hostnameFlag := flag.String("hostname", envHostname, "Override hostname reported to Pulse")
	agentIDFlag := flag.String("agent-id", envAgentID, "Override agent identifier")
	insecureFlag := flag.Bool("insecure", utils.ParseBool(envInsecure), "Skip TLS certificate verification")
	noAutoUpdateFlag := flag.Bool("no-auto-update", utils.ParseBool(envNoAutoUpdate), "Disable automatic agent updates")
	runtimeFlag := flag.String("runtime", envRuntime, "Container runtime to expect (auto, docker, podman)")
	logLevelFlag := flag.String("log-level", logLevelDefault, "Log level: debug, info, warn, error")
	var targetFlags stringFlagList
	flag.Var(&targetFlags, "target", "Pulse target in url|token[|insecure] format. Repeat to send to multiple Pulse instances")
	var containerStateFlags stringFlagList
	flag.Var(&containerStateFlags, "container-state", "Only include containers whose status matches this value (repeat to allow multiple). Allowed values: created,running,restarting,removing,paused,exited,dead.")
	swarmScopeFlag := flag.String("swarm-scope", strings.ToLower(strings.TrimSpace(swarmScopeDefault)), "Swarm data scope to collect: node, cluster, or auto")
	includeServicesFlag := flag.Bool("swarm-services", includeServicesDefault, "Include Swarm service summaries in reports")
	includeTasksFlag := flag.Bool("swarm-tasks", includeTasksDefault, "Include Swarm tasks in reports")
	includeContainersFlag := flag.Bool("include-containers", includeContainersDefault, "Include per-container metrics in reports")
	collectDiskFlag := flag.Bool("collect-disk", collectDiskDefault, "Collect per-container disk usage, block IO, and mount details in reports")

	flag.Parse()

	// Check for common mistakes with unparsed arguments
	unparsedArgs := flag.Args()
	if len(unparsedArgs) > 0 {
		// Check if any look like flags with single dash
		for _, arg := range unparsedArgs {
			if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
				fmt.Fprintf(os.Stderr, "error: unrecognized argument %q\n", arg)
				fmt.Fprintln(os.Stderr, "note: flags must use double dashes (e.g., --token, not -token)")
				fmt.Fprintln(os.Stderr, "\nUsage:")
				flag.Usage()
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "error: unexpected arguments: %v\n", unparsedArgs)
		flag.Usage()
		os.Exit(1)
	}

	pulseURL := *urlFlag
	urlFromEnvOrFlag := envURL != "" || *urlFlag != envURL
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}

	targets := make([]dockeragent.TargetConfig, 0)

	if len(targetFlags) > 0 {
		parsedTargets, err := parseTargetSpecs(targetFlags.Values())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		targets = append(targets, parsedTargets...)
	}

	if envTargets != "" {
		envTargetSpecs := splitTargetSpecs(envTargets)
		if len(envTargetSpecs) > 0 {
			parsedTargets, err := parseTargetSpecs(envTargetSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			targets = append(targets, parsedTargets...)
		}
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" && len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "error: PULSE_TOKEN, --token, or at least one --target/PULSE_TARGETS entry must be provided")
		fmt.Fprintln(os.Stderr, "\nExample usage:")
		fmt.Fprintln(os.Stderr, "  pulse-docker-agent --url http://pulse.example.com:7655 --token <your-token>")
		fmt.Fprintln(os.Stderr, "\nOr set environment variables:")
		fmt.Fprintln(os.Stderr, "  export PULSE_URL=http://pulse.example.com:7655")
		fmt.Fprintln(os.Stderr, "  export PULSE_TOKEN=<your-token>")
		fmt.Fprintln(os.Stderr, "  pulse-docker-agent")
		os.Exit(1)
	}

	// Warn if using default localhost URL without explicit configuration
	if !urlFromEnvOrFlag && len(targets) == 0 && token != "" {
		fmt.Fprintln(os.Stderr, "warning: no --url or PULSE_URL provided, defaulting to http://localhost:7655")
		fmt.Fprintln(os.Stderr, "note: if your Pulse server is not on localhost, specify --url http://your-pulse-server:7655")
		fmt.Fprintln(os.Stderr, "")
	}

	interval := *intervalFlag
	if interval <= 0 {
		interval = 30 * time.Second
	}

	logLevel, err := parseLogLevel(*logLevelFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	containerStates := make([]string, 0)
	if len(containerStateFlags) > 0 {
		containerStates = append(containerStates, containerStateFlags.Values()...)
	}
	if envContainerStates != "" {
		containerStates = append(containerStates, splitStringList(envContainerStates)...)
	}

	return dockeragent.Config{
		PulseURL:           pulseURL,
		APIToken:           token,
		Interval:           interval,
		HostnameOverride:   strings.TrimSpace(*hostnameFlag),
		AgentID:            strings.TrimSpace(*agentIDFlag),
		InsecureSkipVerify: *insecureFlag,
		DisableAutoUpdate:  *noAutoUpdateFlag,
		Targets:            targets,
		ContainerStates:    containerStates,
		SwarmScope:         strings.ToLower(strings.TrimSpace(*swarmScopeFlag)),
		Runtime:            strings.ToLower(strings.TrimSpace(*runtimeFlag)),
		IncludeServices:    *includeServicesFlag,
		IncludeTasks:       *includeTasksFlag,
		IncludeContainers:  *includeContainersFlag,
		CollectDiskMetrics: *collectDiskFlag,
		LogLevel:           logLevel,
	}
}

func parseLogLevel(value string) (zerolog.Level, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return zerolog.InfoLevel, nil
	}

	level, err := zerolog.ParseLevel(normalized)
	if err != nil {
		return zerolog.InfoLevel, fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", value)
	}

	return level, nil
}

func parseTargetSpecs(specs []string) ([]dockeragent.TargetConfig, error) {
	targets := make([]dockeragent.TargetConfig, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		target, err := parseTargetSpec(spec)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func parseTargetSpec(spec string) (dockeragent.TargetConfig, error) {
	parts := strings.Split(spec, "|")
	if len(parts) < 2 {
		return dockeragent.TargetConfig{}, fmt.Errorf("invalid target %q: expected format url|token[|insecure]", spec)
	}

	url := strings.TrimSpace(parts[0])
	token := strings.TrimSpace(parts[1])
	if url == "" {
		return dockeragent.TargetConfig{}, fmt.Errorf("invalid target %q: URL is required", spec)
	}
	if token == "" {
		return dockeragent.TargetConfig{}, fmt.Errorf("invalid target %q: token is required", spec)
	}

	insecure := false
	if len(parts) >= 3 {
		switch strings.ToLower(strings.TrimSpace(parts[2])) {
		case "1", "true", "yes", "y", "on":
			insecure = true
		case "", "0", "false", "no", "n", "off":
			insecure = false
		default:
			return dockeragent.TargetConfig{}, fmt.Errorf("invalid target %q: insecure flag must be true/false", spec)
		}
	}

	return dockeragent.TargetConfig{
		URL:                url,
		Token:              token,
		InsecureSkipVerify: insecure,
	}, nil
}

func splitTargetSpecs(value string) []string {
	if value == "" {
		return nil
	}

	normalized := strings.ReplaceAll(value, "\n", ";")
	raw := strings.Split(normalized, ";")
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitStringList(value string) []string {
	if value == "" {
		return nil
	}

	items := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r':
			return true
		default:
			return false
		}
	})

	result := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
