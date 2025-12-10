package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// Version information (set at build time with -ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const (
	defaultSocketPath   = "/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
	defaultSSHKeyPath   = "/var/lib/pulse-sensor-proxy/ssh"
	defaultConfigPath   = "/etc/pulse-sensor-proxy/config.yaml"
	defaultAuditLogPath = "/var/log/pulse/sensor-proxy/audit.log"
	maxRequestBytes     = 16 * 1024 // 16 KiB max request size
	defaultRunAsUser    = "pulse-sensor-proxy"
)

func defaultWorkDir() string {
	return "/var/lib/pulse-sensor-proxy"
}

var (
	configPath string
)

var rootCmd = &cobra.Command{
	Use:     "pulse-sensor-proxy",
	Short:   "Pulse Sensor Proxy - Secure sensor data bridge for containerized Pulse",
	Long:    `Sensor monitoring proxy that keeps SSH keys on the host and exposes sensor data via unix socket`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		runProxy()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("pulse-sensor-proxy %s\n", Version)
		if BuildTime != "unknown" {
			fmt.Printf("Built: %s\n", BuildTime)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Commit: %s\n", GitCommit)
		}
	},
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Print SSH public keys",
	Run: func(cmd *cobra.Command, args []string) {
		sshDir := os.Getenv("PULSE_SENSOR_PROXY_SSH_DIR")
		if sshDir == "" {
			sshDir = defaultSSHKeyPath
		}

		pubKeyPath := filepath.Join(sshDir, "id_ed25519.pub")
		pubKeyBytes, err := os.ReadFile(pubKeyPath)
		if err != nil {
			// Try to find the key in the working directory as fallback
			if wd, wdErr := os.Getwd(); wdErr == nil {
				localPath := filepath.Join(wd, "id_ed25519.pub")
				if localBytes, localErr := os.ReadFile(localPath); localErr == nil {
					pubKeyBytes = localBytes
				} else {
					fmt.Fprintf(os.Stderr, "Failed to read public key from %s: %v\n", pubKeyPath, err)
					os.Exit(1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Failed to read public key from %s: %v\n", pubKeyPath, err)
				os.Exit(1)
			}
		}
		pubKey := strings.TrimSpace(string(pubKeyBytes))

		fmt.Printf("Proxy Public Key: %s\n", pubKey)
		fmt.Printf("Sensors Public Key: %s\n", pubKey)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(keysCmd)
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to configuration file (default: /etc/pulse-sensor-proxy/config.yaml)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseLogLevel converts a string log level to zerolog.Level
func parseLogLevel(levelStr string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(levelStr)) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled", "none":
		return zerolog.Disabled
	default:
		log.Warn().Str("level", levelStr).Msg("Unknown log level, defaulting to info")
		return zerolog.InfoLevel
	}
}

type userSpec struct {
	name   string
	uid    int
	gid    int
	groups []int
	home   string
}

func dropPrivileges(username string) (*userSpec, error) {
	if username == "" {
		return nil, nil
	}

	if os.Geteuid() != 0 {
		return nil, nil
	}

	spec, err := resolveUserSpec(username)
	if err != nil {
		return nil, err
	}

	if len(spec.groups) == 0 {
		spec.groups = []int{spec.gid}
	}

	if err := unix.Setgroups(spec.groups); err != nil {
		return nil, fmt.Errorf("setgroups: %w", err)
	}
	if err := unix.Setgid(spec.gid); err != nil {
		return nil, fmt.Errorf("setgid: %w", err)
	}
	if err := unix.Setuid(spec.uid); err != nil {
		return nil, fmt.Errorf("setuid: %w", err)
	}

	if spec.home != "" {
		_ = os.Setenv("HOME", spec.home)
	}
	if spec.name != "" {
		_ = os.Setenv("USER", spec.name)
		_ = os.Setenv("LOGNAME", spec.name)
	}

	return spec, nil
}

func resolveUserSpec(username string) (*userSpec, error) {
	u, err := user.Lookup(username)
	if err == nil {
		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return nil, fmt.Errorf("parse uid %q: %w", u.Uid, err)
		}
		gid, err := strconv.Atoi(u.Gid)
		if err != nil {
			return nil, fmt.Errorf("parse gid %q: %w", u.Gid, err)
		}

		var groups []int
		if gids, err := u.GroupIds(); err == nil {
			for _, g := range gids {
				if gidVal, convErr := strconv.Atoi(g); convErr == nil {
					groups = append(groups, gidVal)
				}
			}
		}

		if len(groups) == 0 {
			groups = []int{gid}
		}

		return &userSpec{
			name:   u.Username,
			uid:    uid,
			gid:    gid,
			groups: groups,
			home:   u.HomeDir,
		}, nil
	}

	fallbackSpec, fallbackErr := lookupUserFromPasswd(username)
	if fallbackErr == nil {
		return fallbackSpec, nil
	}

	return nil, fmt.Errorf("lookup user %q failed: %v (fallback: %w)", username, err, fallbackErr)
}

func lookupUserFromPasswd(username string) (*userSpec, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, fmt.Errorf("open /etc/passwd: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}
		if fields[0] != username {
			continue
		}

		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("parse uid %q: %w", fields[2], err)
		}
		gid, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("parse gid %q: %w", fields[3], err)
		}

		return &userSpec{
			name:   fields[0],
			uid:    uid,
			gid:    gid,
			groups: []int{gid},
			home:   fields[5],
		}, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan /etc/passwd: %w", err)
	}

	return nil, fmt.Errorf("user %q not found in /etc/passwd", username)
}

// Proxy manages the temperature monitoring proxy
type Proxy struct {
	socketPath         string
	sshKeyPath         string
	workDir            string
	knownHosts         knownhosts.Manager
	listener           net.Listener
	rateLimiter        *rateLimiter
	nodeGate           *nodeGate
	router             map[string]handlerFunc
	config             *Config
	metrics            *ProxyMetrics
	audit              *auditLogger
	nodeValidator      *nodeValidator
	readTimeout        time.Duration
	writeTimeout       time.Duration
	maxSSHOutputBytes  int64
	controlPlaneCfg    *ControlPlaneConfig
	controlPlaneToken  string
	controlPlaneCancel context.CancelFunc

	allowedPeerUIDs   map[uint32]struct{}
	allowedPeerGIDs   map[uint32]struct{}
	peerCapabilities  map[uint32]Capability
	idMappedUIDRanges []idRange
	idMappedGIDRanges []idRange
}

type contextKey int

const (
	contextKeyPeerCapabilities contextKey = iota + 1
)

func withPeerCapabilities(ctx context.Context, caps Capability) context.Context {
	return context.WithValue(ctx, contextKeyPeerCapabilities, caps)
}

func peerCapabilitiesFromContext(ctx context.Context) Capability {
	if ctx == nil {
		return 0
	}
	if value := ctx.Value(contextKeyPeerCapabilities); value != nil {
		if caps, ok := value.(Capability); ok {
			return caps
		}
	}
	return 0
}

// RPC request types
const (
	RPCEnsureClusterKeys = "ensure_cluster_keys"
	RPCRegisterNodes     = "register_nodes"
	RPCGetTemperature    = "get_temperature"
	RPCGetStatus         = "get_status"
	RPCRequestCleanup    = "request_cleanup"
)

// Privileged RPC methods that require host-level access (not accessible from containers)
var privilegedMethods = map[string]bool{
	RPCEnsureClusterKeys: true, // SSH key distribution
	RPCRegisterNodes:     true, // Node registration
	RPCRequestCleanup:    true, // Cleanup operations
}

// RPCRequest represents a request from Pulse
type RPCRequest struct {
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Method        string                 `json:"method"`
	Params        map[string]interface{} `json:"params"`
}

// RPCResponse represents a response to Pulse
type RPCResponse struct {
	CorrelationID string      `json:"correlation_id,omitempty"`
	Success       bool        `json:"success"`
	Data          interface{} `json:"data,omitempty"`
	Error         string      `json:"error,omitempty"`
}

// handlerFunc is the signature for RPC method handlers
type handlerFunc func(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error)

func runProxy() {
	// Initialize logger with default level (will be configured after loading config)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	socketPath := os.Getenv("PULSE_SENSOR_PROXY_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	sshKeyPath := os.Getenv("PULSE_SENSOR_PROXY_SSH_DIR")
	if sshKeyPath == "" {
		sshKeyPath = defaultSSHKeyPath
	}

	// Load configuration
	// Priority: --config flag > PULSE_SENSOR_PROXY_CONFIG env > default path
	cfgPath := configPath // from flag
	if cfgPath == "" {
		cfgPath = os.Getenv("PULSE_SENSOR_PROXY_CONFIG")
	}
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Apply configured log level
	level := parseLogLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)

	runAsUser := os.Getenv("PULSE_SENSOR_PROXY_USER")
	if runAsUser == "" {
		runAsUser = defaultRunAsUser
	}

	if spec, err := dropPrivileges(runAsUser); err != nil {
		log.Fatal().Err(err).Str("user", runAsUser).Msg("Failed to drop privileges")
	} else if spec != nil {
		log.Info().
			Str("user", spec.name).
			Int("uid", spec.uid).
			Int("gid", spec.gid).
			Msg("Running as unprivileged user")
	}

	auditPath := os.Getenv("PULSE_SENSOR_PROXY_AUDIT_LOG")
	if auditPath == "" {
		auditPath = defaultAuditLogPath
	}

	// Initialize audit logger with automatic fallback to stderr if file is unavailable
	auditLogger := newAuditLogger(auditPath)
	defer auditLogger.Close()

	// Initialize metrics
	metrics := NewProxyMetrics(Version)

	nodeValidator, err := newNodeValidator(cfg, metrics)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize node validator")
	}

	log.Info().
		Str("socket", socketPath).
		Str("ssh_key_dir", sshKeyPath).
		Str("config_path", cfgPath).
		Str("audit_log", auditPath).
		Str("log_level", cfg.LogLevel).
		Str("version", Version).
		Msg("Starting pulse-sensor-proxy")

	// Warn if running inside a container - this is an unsupported configuration
	// Container environments cannot complete SSH workflows, causing [preauth] log floods
	if system.InContainer() && os.Getenv("PULSE_SENSOR_PROXY_SUPPRESS_CONTAINER_WARNING") != "true" {
		log.Warn().Msg("pulse-sensor-proxy is running inside a container - this is unsupported and will cause SSH [preauth] log floods. Install on the Proxmox host instead. See docs/TEMPERATURE_MONITORING.md for setup instructions. Set PULSE_SENSOR_PROXY_SUPPRESS_CONTAINER_WARNING=true to suppress this warning.")
	}

	knownHostsManager, err := knownhosts.NewManager(filepath.Join(sshKeyPath, "known_hosts"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize known hosts manager")
	}

	proxy := &Proxy{
		socketPath:        socketPath,
		sshKeyPath:        sshKeyPath,
		knownHosts:        knownHostsManager,
		nodeGate:          newNodeGate(),
		config:            cfg,
		metrics:           metrics,
		audit:             auditLogger,
		nodeValidator:     nodeValidator,
		readTimeout:       cfg.ReadTimeout,
		writeTimeout:      cfg.WriteTimeout,
		maxSSHOutputBytes: cfg.MaxSSHOutputBytes,
		controlPlaneCfg:   cfg.PulseControlPlane,
	}

	if wd, err := os.Getwd(); err == nil {
		proxy.workDir = wd
	} else {
		log.Warn().Err(err).Msg("Failed to determine working directory; using default")
		proxy.workDir = defaultWorkDir()
	}

	if err := proxy.initAuthRules(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize authentication rules")
	}

	proxy.rateLimiter = newRateLimiter(metrics, cfg.RateLimit, proxy.idMappedUIDRanges, proxy.idMappedGIDRanges)

	// Register RPC method handlers
	proxy.router = map[string]handlerFunc{
		RPCGetStatus:         proxy.handleGetStatusV2,
		RPCEnsureClusterKeys: proxy.handleEnsureClusterKeysV2,
		RPCRegisterNodes:     proxy.handleRegisterNodesV2,
		RPCGetTemperature:    proxy.handleGetTemperatureV2,
		RPCRequestCleanup:    proxy.handleRequestCleanup,
	}

	if err := proxy.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start proxy")
	}
	proxy.startControlPlaneSync()

	// Start HTTP server if enabled
	var httpServer *HTTPServer
	if cfg.HTTPEnabled {
		httpServer = NewHTTPServer(proxy, cfg)
		if err := httpServer.Start(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}

	// Start metrics server
	if err := metrics.Start(cfg.MetricsAddress); err != nil {
		log.Fatal().Err(err).Msg("Failed to start metrics server")
	}

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Info().Msg("Shutting down proxy...")

	// Shutdown HTTP server if running
	if httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error shutting down HTTP server")
		}
	}

	proxy.Stop()
	if proxy.rateLimiter != nil {
		proxy.rateLimiter.shutdown()
	}
	metrics.Shutdown(context.Background())
	log.Info().Msg("Proxy stopped")
}

// Start initializes and starts the proxy
func (p *Proxy) Start() error {
	// Create SSH key directory if it doesn't exist
	if err := os.MkdirAll(p.sshKeyPath, 0700); err != nil {
		return fmt.Errorf("failed to create SSH key directory: %w", err)
	}

	// Ensure SSH keypair exists
	if err := p.ensureSSHKeypair(); err != nil {
		return fmt.Errorf("failed to ensure SSH keypair: %w", err)
	}

	// Remove existing socket if it exists
	if err := os.RemoveAll(p.socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create socket directory if needed
	socketDir := filepath.Dir(p.socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create unix socket listener
	listener, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create unix socket: %w", err)
	}
	p.listener = listener

	// Set liberal socket permissions; SO_PEERCRED enforces auth
	if err := os.Chmod(p.socketPath, 0666); err != nil {
		log.Warn().Err(err).Msg("Failed to set socket permissions")
	}

	log.Info().Str("socket", p.socketPath).Msg("Unix socket ready")

	// Start accepting connections
	go p.acceptConnections()

	return nil
}

// Stop shuts down the proxy
func (p *Proxy) Stop() {
	if p.controlPlaneCancel != nil {
		p.controlPlaneCancel()
	}
	if p.listener != nil {
		p.listener.Close()
		os.Remove(p.socketPath)
	}
}

func (p *Proxy) startControlPlaneSync() {
	if p.controlPlaneCfg == nil || strings.TrimSpace(p.controlPlaneCfg.URL) == "" {
		return
	}

	tokenBytes, err := os.ReadFile(p.controlPlaneCfg.TokenFile)
	if err != nil {
		log.Warn().Err(err).Str("token_file", p.controlPlaneCfg.TokenFile).Msg("Control plane token unavailable; skipping sync")
		return
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		log.Warn().Str("token_file", p.controlPlaneCfg.TokenFile).Msg("Control plane token is empty; skipping sync")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.controlPlaneToken = token
	p.controlPlaneCancel = cancel
	go p.controlPlaneLoop(ctx)
	log.Info().
		Str("url", p.controlPlaneCfg.URL).
		Int("refresh_interval", p.controlPlaneCfg.RefreshIntervalSec).
		Msg("Control plane synchronization enabled")
}

func (p *Proxy) controlPlaneLoop(ctx context.Context) {
	refresh := time.Duration(p.controlPlaneCfg.RefreshIntervalSec) * time.Second
	if refresh <= 0 {
		refresh = time.Duration(defaultControlPlaneRefreshSecs) * time.Second
	}

	client := &http.Client{Timeout: 10 * time.Second}
	if p.controlPlaneCfg.InsecureSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	for {
		if err := p.fetchAuthorizedNodes(client); err != nil {
			log.Warn().Err(err).Msg("Control plane sync failed")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(refresh):
		}
	}
}

func (p *Proxy) fetchAuthorizedNodes(client *http.Client) error {
	cfg := p.controlPlaneCfg
	if cfg == nil {
		return nil
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.URL, "/")+"/api/temperature-proxy/authorized-nodes", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Proxy-Token", p.controlPlaneToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("control plane responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Nodes []struct {
			Name string `json:"name"`
			IP   string `json:"ip"`
		} `json:"nodes"`
		Hash            string    `json:"hash"`
		RefreshInterval int       `json:"refresh_interval"`
		GeneratedAt     time.Time `json:"generated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode authorized-nodes response: %w", err)
	}

	if payload.RefreshInterval > 0 && cfg.RefreshIntervalSec != payload.RefreshInterval {
		cfg.RefreshIntervalSec = payload.RefreshInterval
	}

	var entries []string
	for _, node := range payload.Nodes {
		if node.Name != "" {
			entries = append(entries, node.Name)
		}
		if node.IP != "" {
			entries = append(entries, node.IP)
		}
	}

	if len(entries) == 0 {
		return errors.New("authorized-nodes response empty")
	}

	p.nodeValidator.UpdateAllowlist(entries)
	return nil
}

// acceptConnections handles incoming socket connections
func (p *Proxy) acceptConnections() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			// Check if listener was closed
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return
			}
			log.Error().Err(err).Msg("Failed to accept connection")
			continue
		}

		go p.handleConnection(conn)
	}
}

// handleConnection processes a single RPC request with full validation and throttling
func (p *Proxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()

	// Track concurrent requests
	p.metrics.queueDepth.Inc()
	defer p.metrics.queueDepth.Dec()

	// Start timing for latency metrics
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Enforce read deadline to prevent hung connections
	readDeadline := p.readTimeout
	if readDeadline <= 0 {
		readDeadline = 5 * time.Second
	}
	if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		log.Warn().Err(err).Msg("Failed to set read deadline")
	}

	// Extract and verify peer credentials
	cred, err := extractPeerCredentials(conn)
	if err != nil {
		log.Warn().Err(err).Msg("Peer credentials unavailable")
		if p.audit != nil {
			p.audit.LogConnectionDenied("", nil, remoteAddr, "peer_credentials_unavailable")
		}
		p.sendErrorV2(conn, "unauthorized", "")
		return
	}

	peerCaps, err := p.authorizePeer(cred)
	if err != nil {
		log.Warn().
			Err(err).
			Uint32("uid", cred.uid).
			Uint32("gid", cred.gid).
			Msg("Peer authorization failed")
		if p.audit != nil {
			p.audit.LogConnectionDenied("", cred, remoteAddr, err.Error())
		}
		p.sendErrorV2(conn, "unauthorized", "")
		return
	}

	if p.audit != nil {
		p.audit.LogConnectionAccepted("", cred, remoteAddr)
	}

	if p.rateLimiter == nil {
		log.Error().Msg("Rate limiter not initialized; rejecting connection")
		p.sendErrorV2(conn, "service unavailable", "")
		return
	}

	// Check rate limit and concurrency
	peer := p.rateLimiter.identifyPeer(cred)
	peerLabel := peer.String()
	releaseLimiter, limitReason, allowed := p.rateLimiter.allow(peer)
	if !allowed {
		log.Warn().
			Uint32("uid", cred.uid).
			Uint32("pid", cred.pid).
			Str("reason", limitReason).
			Msg("Rate limit exceeded")
		if p.audit != nil {
			p.audit.LogRateLimitHit("", cred, remoteAddr, limitReason)
		}
		p.sendErrorV2(conn, "rate limit exceeded", "")
		return
	}
	releaseFn := releaseLimiter
	defer func() {
		if releaseFn != nil {
			releaseFn()
		}
	}()
	applyPenalty := func(reason string) {
		if releaseFn != nil {
			releaseFn()
			releaseFn = nil
		}
		p.rateLimiter.penalize(peerLabel, reason)
	}

	// Read request using newline-delimited framing
	limited := &io.LimitedReader{R: conn, N: maxRequestBytes}
	reader := bufio.NewReader(limited)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Warn().
				Err(err).
				Str("remote", remoteAddr).
				Msg("Read timeout waiting for client request - slow client or attack")
			if p.metrics != nil {
				p.metrics.recordReadTimeout()
			}
			if p.audit != nil {
				p.audit.LogValidationFailure("", cred, remoteAddr, "", nil, "read_timeout")
			}
			p.sendErrorV2(conn, "read timeout", "")
			applyPenalty("read_timeout")
			return
		}
		if errors.Is(err, bufio.ErrBufferFull) || limited.N <= 0 {
			if p.audit != nil {
				p.audit.LogValidationFailure("", cred, remoteAddr, "", nil, "payload_too_large")
			}
			p.sendErrorV2(conn, "payload too large", "")
			applyPenalty("payload_too_large")
			return
		}
		if errors.Is(err, io.EOF) {
			if p.audit != nil {
				p.audit.LogValidationFailure("", cred, remoteAddr, "", nil, "empty_request")
			}
			p.sendErrorV2(conn, "empty request", "")
			applyPenalty("empty_request")
			return
		}
		if p.audit != nil {
			p.audit.LogValidationFailure("", cred, remoteAddr, "", nil, "read_error")
		}
		p.sendErrorV2(conn, "failed to read request", "")
		applyPenalty("read_error")
		return
	}

	// Clear read deadline now that request payload has been received
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to clear read deadline after request read")
	}

	// Trim whitespace and validate
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		if p.audit != nil {
			p.audit.LogValidationFailure("", cred, remoteAddr, "", nil, "empty_request")
		}
		p.sendErrorV2(conn, "empty request", "")
		applyPenalty("empty_request")
		return
	}

	// Parse JSON
	var req RPCRequest
	if err := json.Unmarshal(line, &req); err != nil {
		if p.audit != nil {
			p.audit.LogValidationFailure("", cred, remoteAddr, "", nil, "invalid_json")
		}
		p.sendErrorV2(conn, "invalid request format", "")
		applyPenalty("invalid_json")
		return
	}

	// Sanitize correlation ID
	req.CorrelationID = sanitizeCorrelationID(req.CorrelationID)

	// Create contextual logger
	logger := log.With().
		Str("corr_id", req.CorrelationID).
		Uint32("uid", cred.uid).
		Uint32("pid", cred.pid).
		Str("method", req.Method).
		Logger()

	// Prepare response
	resp := RPCResponse{
		CorrelationID: req.CorrelationID,
		Success:       false,
	}

	// Find handler
	handler := p.router[req.Method]
	if handler == nil {
		if p.audit != nil {
			p.audit.LogValidationFailure(req.CorrelationID, cred, remoteAddr, req.Method, nil, "unknown_method")
		}
		resp.Error = "unknown method"
		logger.Warn().Msg("Unknown method")
		p.sendResponse(conn, resp, p.writeTimeout)
		applyPenalty("unknown_method")
		return
	}

	// Check if method requires host-level privileges
	if privilegedMethods[req.Method] {
		if !peerCaps.Has(CapabilityAdmin) {
			resp.Error = "method requires admin capability"
			log.Warn().
				Str("method", req.Method).
				Uint32("uid", cred.uid).
				Uint32("gid", cred.gid).
				Uint32("pid", cred.pid).
				Str("corr_id", req.CorrelationID).
				Msg("SECURITY: peer lacking admin capability attempted privileged method - access denied")
			if p.audit != nil {
				p.audit.LogValidationFailure(req.CorrelationID, cred, remoteAddr, req.Method, nil, "capability_denied")
			}
			p.sendResponse(conn, resp, p.writeTimeout)
			p.metrics.rpcRequests.WithLabelValues(req.Method, "unauthorized").Inc()
			applyPenalty("capability_denied")
			return
		}
	}

	if p.audit != nil {
		p.audit.LogCommandStart(req.CorrelationID, cred, remoteAddr, "", req.Method, nil)
	}

	// Annotate context with peer capabilities before executing handler
	ctx = withPeerCapabilities(ctx, peerCaps)

	// Execute handler
	result, err := handler(ctx, &req, logger)
	duration := time.Since(startTime)
	if err != nil {
		if p.audit != nil {
			p.audit.LogCommandResult(req.CorrelationID, cred, remoteAddr, "", req.Method, nil, 1, duration, "", "", err)
		}
		resp.Error = err.Error()
		logger.Warn().Err(err).Msg("Handler failed")
		p.sendResponse(conn, resp, p.writeTimeout)
		// Record failed request
		p.metrics.rpcRequests.WithLabelValues(req.Method, "error").Inc()
		p.metrics.rpcLatency.WithLabelValues(req.Method).Observe(time.Since(startTime).Seconds())
		return
	}

	// Success
	resp.Success = true
	resp.Data = result
	if p.audit != nil {
		p.audit.LogCommandResult(req.CorrelationID, cred, remoteAddr, "", req.Method, nil, 0, duration, "", "", nil)
	}
	logger.Info().Msg("Request completed")

	p.sendResponse(conn, resp, p.writeTimeout)

	// Record successful request
	p.metrics.rpcRequests.WithLabelValues(req.Method, "success").Inc()
	p.metrics.rpcLatency.WithLabelValues(req.Method).Observe(time.Since(startTime).Seconds())
}

// sendErrorV2 sends an error response with correlation ID
func (p *Proxy) sendErrorV2(conn net.Conn, message, correlationID string) {
	resp := RPCResponse{
		CorrelationID: correlationID,
		Success:       false,
		Error:         message,
	}
	p.sendResponse(conn, resp, p.writeTimeout)
}

// sendResponse sends an RPC response
func (p *Proxy) sendResponse(conn net.Conn, resp RPCResponse, writeTimeout time.Duration) {
	// Clear read deadline before writing
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to clear read deadline")
	}

	if writeTimeout <= 0 {
		writeTimeout = 10 * time.Second
	}

	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		log.Warn().Err(err).Dur("write_timeout", writeTimeout).Msg("Failed to set write deadline")
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Warn().Err(err).Msg("Write timeout while sending RPC response")
			if p.metrics != nil {
				p.metrics.recordWriteTimeout()
			}
		} else {
			log.Error().Err(err).Msg("Failed to encode RPC response")
		}
	}
}

// ensureSSHKeypair generates SSH keypair if it doesn't exist
func (p *Proxy) ensureSSHKeypair() error {
	privKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519")
	pubKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519.pub")

	// Check if keypair already exists
	if _, err := os.Stat(privKeyPath); err == nil {
		if _, err := os.Stat(pubKeyPath); err == nil {
			log.Info().Msg("SSH keypair already exists")
			return nil
		}
	}

	log.Info().Msg("Generating new SSH keypair")

	// Generate ed25519 keypair using ssh-keygen
	cmd := fmt.Sprintf("ssh-keygen -t ed25519 -f %s -N '' -C 'pulse-sensor-proxy'", privKeyPath)
	if output, err := execCommand(cmd); err != nil {
		return fmt.Errorf("failed to generate SSH keypair: %w (output: %s)", err, output)
	}

	log.Info().Str("path", privKeyPath).Msg("SSH keypair generated")
	return nil
}

// V2 handlers with context and structured logging

// handleGetStatusV2 returns proxy status with context support
func (p *Proxy) handleGetStatusV2(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
	pubKeyPath := filepath.Join(p.sshKeyPath, "id_ed25519.pub")
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	logger.Info().Msg("Status request served")
	response := map[string]interface{}{
		"version":    Version,
		"public_key": string(pubKey),
		"ssh_dir":    p.sshKeyPath,
	}

	if caps := peerCapabilitiesFromContext(ctx); caps != 0 {
		if names := capabilityNames(caps); len(names) > 0 {
			response["capabilities"] = names
		}
	}

	return response, nil
}

// handleEnsureClusterKeysV2 discovers cluster nodes and pushes SSH keys with validation
func (p *Proxy) handleEnsureClusterKeysV2(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
	// Check if we're on a Proxmox host
	if !isProxmoxHost() {
		return nil, fmt.Errorf("not running on Proxmox host - cannot discover cluster")
	}

	// Check for optional key_dir parameter (for key rotation)
	keyDir := p.sshKeyPath // default
	if keyDirParam, ok := req.Params["key_dir"]; ok {
		if keyDirStr, ok := keyDirParam.(string); ok && keyDirStr != "" {
			keyDir = keyDirStr
			logger.Info().Str("key_dir", keyDir).Msg("Using custom key directory for rotation")
		}
	}

	// Discover cluster nodes
	nodes, err := discoverClusterNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to discover cluster: %w", err)
	}

	logger.Info().Strs("nodes", nodes).Msg("Discovered cluster nodes")

	// Push SSH key to each node
	results := make(map[string]interface{})
	successCount := 0
	for _, node := range nodes {
		// Validate node name
		if err := validateNodeName(node); err != nil {
			logger.Warn().Str("node", node).Msg("Invalid node name format")
			results[node] = map[string]interface{}{
				"success": false,
				"error":   "invalid node name",
			}
			continue
		}

		logger.Info().Str("node", node).Str("key_dir", keyDir).Msg("Pushing SSH key to node")
		if err := p.pushSSHKeyFrom(node, keyDir); err != nil {
			logger.Error().Err(err).Str("node", node).Msg("Failed to push SSH key")
			results[node] = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
		} else {
			logger.Info().Str("node", node).Msg("SSH key pushed successfully")
			results[node] = map[string]interface{}{
				"success": true,
			}
			successCount++
		}
	}

	return map[string]interface{}{
		"nodes":         nodes,
		"results":       results,
		"success_count": successCount,
		"total_count":   len(nodes),
	}, nil
}

// handleRegisterNodesV2 returns discovered nodes with validation
func (p *Proxy) handleRegisterNodesV2(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
	// Check if we're on a Proxmox host
	if !isProxmoxHost() {
		return nil, fmt.Errorf("not running on Proxmox host")
	}

	// Discover cluster nodes
	nodes, err := discoverClusterNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to discover nodes: %w", err)
	}

	// Test SSH connectivity to each node
	nodeStatus := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		status := map[string]interface{}{
			"name": node,
		}

		// Validate node name
		if err := validateNodeName(node); err != nil {
			status["ssh_ready"] = false
			status["error"] = "invalid node name"
			nodeStatus = append(nodeStatus, status)
			continue
		}

		if err := p.testSSHConnection(node); err != nil {
			status["ssh_ready"] = false
			status["error"] = err.Error()
		} else {
			status["ssh_ready"] = true
		}

		nodeStatus = append(nodeStatus, status)
	}

	logger.Info().Int("node_count", len(nodeStatus)).Msg("Node discovery completed")
	return map[string]interface{}{
		"nodes": nodeStatus,
	}, nil
}

// handleGetTemperatureV2 fetches temperature data with concurrency control and validation
func (p *Proxy) handleGetTemperatureV2(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
	// Extract node parameter
	nodeParam, ok := req.Params["node"]
	if !ok {
		return nil, fmt.Errorf("missing 'node' parameter")
	}

	node, ok := nodeParam.(string)
	if !ok {
		return nil, fmt.Errorf("'node' parameter must be a string")
	}

	// Trim and validate node name
	node = strings.TrimSpace(node)
	if err := validateNodeName(node); err != nil {
		logger.Warn().Str("node", node).Msg("Invalid node name format")
		return nil, fmt.Errorf("invalid node name")
	}

	if p.nodeValidator != nil {
		if err := p.nodeValidator.Validate(ctx, node); err != nil {
			logger.Warn().Err(err).Str("node", node).Msg("Node validation failed")
			return nil, err
		}
	}

	// Acquire per-node concurrency lock (context-aware to prevent goroutine leaks)
	releaseNode, err := p.nodeGate.acquireContext(ctx, node)
	if err != nil {
		logger.Warn().Err(err).Str("node", node).Msg("Request cancelled while waiting for node lock")
		return nil, fmt.Errorf("request cancelled while waiting for node")
	}
	defer releaseNode()

	logger.Debug().Str("node", node).Msg("Fetching temperature via SSH")

	// Fetch temperature data with timeout
	sshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tempData, err := p.getTemperatureViaSSH(sshCtx, node)
	if err != nil {
		logger.Warn().Err(err).Str("node", node).Msg("Failed to get temperatures")
		return nil, fmt.Errorf("failed to get temperatures: %w", err)
	}

	logger.Info().Str("node", node).Msg("Temperature data fetched successfully")
	return map[string]interface{}{
		"node":        node,
		"temperature": tempData,
	}, nil
}
