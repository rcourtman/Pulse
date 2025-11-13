package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	// nodeNameRegex validates node names (alphanumeric, dots, underscores, hyphens, 1-64 chars)
	// Must not start with hyphen to prevent SSH option injection
	nodeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

	// ipv4Regex validates IPv4 addresses
	ipv4Regex = regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)

	// ipv6Regex validates IPv6 addresses (simplified)
	ipv6Regex = regexp.MustCompile(`^[0-9a-fA-F:]+$`)
)

var (
	allowedCommands = map[string]struct{}{
		"sensors":  {},
		"ipmitool": {},
	}
)

// sanitizeCorrelationID validates and sanitizes a correlation ID
// Returns a valid UUID, generating a new one if input is missing or invalid
func sanitizeCorrelationID(id string) string {
	if id == "" {
		return uuid.NewString()
	}
	if _, err := uuid.Parse(id); err != nil {
		return uuid.NewString()
	}
	return id
}

// validateNodeName checks if a node name is in valid format
func validateNodeName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid node name")
	}

	if ipv4Regex.MatchString(name) {
		return nil
	}

	candidate := name
	if strings.HasPrefix(candidate, "[") && strings.HasSuffix(candidate, "]") {
		candidate = candidate[1 : len(candidate)-1]
	}

	if ip := net.ParseIP(candidate); ip != nil {
		return nil
	}

	if nodeNameRegex.MatchString(name) {
		return nil
	}

	return fmt.Errorf("invalid node name")
}

func validateCommand(name string, args []string) error {
	if err := validateCommandName(name); err != nil {
		return err
	}

	for _, arg := range args {
		if err := validateCommandArg(arg); err != nil {
			return err
		}
	}

	if name == "ipmitool" {
		if err := validateIPMIToolArgs(args); err != nil {
			return err
		}
	}

	return nil
}

func validateCommandName(name string) error {
	if name == "" {
		return errors.New("command required")
	}

	if strings.Contains(name, "/") {
		return errors.New("absolute command paths not allowed")
	}

	if _, ok := allowedCommands[name]; !ok {
		return fmt.Errorf("command %q not permitted", name)
	}

	if !isASCII(name) {
		return errors.New("command must be ASCII")
	}

	return nil
}

func validateCommandArg(arg string) error {
	if len(arg) == 0 {
		return nil
	}

	if len(arg) > 1024 {
		return errors.New("argument too long")
	}

	if !utf8.ValidString(arg) {
		return errors.New("argument contains invalid UTF-8")
	}

	if hasNullByte(arg) {
		return errors.New("argument contains null byte")
	}

	if !isASCII(arg) {
		return errors.New("argument must be ASCII")
	}

	if hasShellMeta(arg) {
		return errors.New("argument contains forbidden shell characters")
	}

	if strings.Contains(arg, "=") && !strings.HasPrefix(arg, "-") {
		return errors.New("environment-style arguments not permitted")
	}

	return nil
}

func validateIPMIToolArgs(args []string) error {
	lowered := make([]string, len(args))
	for i, arg := range args {
		lowered[i] = strings.ToLower(arg)
	}

	for i := 0; i < len(lowered); i++ {
		token := lowered[i]
		switch token {
		case "shell", "raw", "exec", "lanplus", "lanplusciphers":
			return errors.New("dangerous ipmitool arguments not permitted")
		case "chassis":
			if i+1 < len(lowered) {
				switch lowered[i+1] {
				case "power", "bootparam", "status", "policy":
					return errors.New("chassis operations not permitted")
				}
			}
		case "power", "reset", "off", "cycle", "bmc", "mc":
			return errors.New("power control commands not permitted")
		}
	}

	return nil
}

func hasShellMeta(s string) bool {
	forbidden := []string{";", "|", "&", "$", "`", "\\", ">", "<", "(", ")", "[", "]", "{", "}", "!", "~"}
	for _, ch := range forbidden {
		if strings.Contains(s, ch) {
			return true
		}
	}

	if strings.Contains(s, "..") {
		return true
	}

	if strings.ContainsAny(s, "\n\r\t") {
		return true
	}

	if strings.HasPrefix(s, "-") && strings.Contains(s, "=") {
		if strings.Contains(s, "/") {
			return true
		}
	}

	return false
}

func hasNullByte(s string) bool {
	return strings.IndexByte(s, 0) >= 0
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

const (
	nodeValidatorCacheTTL = 5 * time.Minute

	validationReasonNotAllowlisted   = "node_not_allowlisted"
	validationReasonNotClusterMember = "node_not_cluster_member"
	validationReasonNoSources        = "no_validation_sources"
	validationReasonResolutionFailed = "allowlist_resolution_failed"
	validationReasonClusterFailed    = "cluster_query_failed"
)

// nodeValidator enforces node allow-list and cluster membership checks
type nodeValidator struct {
	allowHosts     map[string]struct{}
	allowCIDRs     []*net.IPNet
	hasAllowlist   bool
	strict         bool
	clusterEnabled bool
	metrics        *ProxyMetrics
	resolver       hostResolver
	clusterFetcher func() ([]string, error)
	cacheTTL       time.Duration
	clock          func() time.Time
	clusterCache   clusterMembershipCache
}

type clusterMembershipCache struct {
	mu      sync.Mutex
	expires time.Time
	nodes   map[string]struct{}
}

type hostResolver interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
}

type defaultHostResolver struct{}

func (defaultHostResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	results, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no IPs resolved for %s", host)
	}

	ips := make([]net.IP, 0, len(results))
	for _, addr := range results {
		if addr.IP != nil {
			ips = append(ips, addr.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP addresses resolved for %s", host)
	}

	return ips, nil
}

func newNodeValidator(cfg *Config, metrics *ProxyMetrics) (*nodeValidator, error) {
	if cfg == nil {
		return nil, errors.New("config is required for node validator")
	}

	v := &nodeValidator{
		allowHosts:     make(map[string]struct{}),
		strict:         cfg.StrictNodeValidation,
		metrics:        metrics,
		resolver:       defaultHostResolver{},
		clusterFetcher: discoverClusterNodes,
		cacheTTL:       nodeValidatorCacheTTL,
		clock:          time.Now,
	}

	for _, raw := range cfg.AllowedNodes {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		if _, network, err := net.ParseCIDR(entry); err == nil {
			v.allowCIDRs = append(v.allowCIDRs, network)
			continue
		}

		if normalized := normalizeAllowlistEntry(entry); normalized != "" {
			v.allowHosts[normalized] = struct{}{}
		}
	}

	v.hasAllowlist = len(v.allowHosts) > 0 || len(v.allowCIDRs) > 0

	if v.hasAllowlist {
		log.Info().
			Int("allowed_node_count", len(v.allowHosts)).
			Int("allowed_cidr_count", len(v.allowCIDRs)).
			Msg("Node allow-list configured")
	}

	if !v.hasAllowlist && isProxmoxHost() {
		v.clusterEnabled = true
		log.Info().Msg("Node validator using Proxmox cluster membership (auto-detect)")
	}

	if !v.clusterEnabled {
		v.clusterFetcher = nil
	}

	if !v.hasAllowlist && !v.clusterEnabled {
		if v.strict {
			log.Warn().Msg("strict_node_validation enabled but no allowlist or cluster context is available")
		} else {
			log.Warn().Msg("SECURITY: Node validator running in permissive mode (no allowlist or cluster context) - all nodes allowed. Configure allowed_nodes to restrict access.")
		}
	}

	return v, nil
}

// Validate ensures the provided node is authorized before any SSH is attempted.
func (v *nodeValidator) Validate(ctx context.Context, node string) error {
	if v == nil {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if v.hasAllowlist {
		allowed, err := v.matchesAllowlist(ctx, node)
		if err != nil {
			v.recordFailure(validationReasonResolutionFailed)
			log.Warn().Err(err).Str("node", node).Msg("Node allow-list resolution failed")
			return err
		}
		if !allowed {
			return v.deny(node, validationReasonNotAllowlisted)
		}
		return nil
	}

	if v.clusterEnabled {
		allowed, err := v.matchesCluster(ctx, node)
		if err != nil {
			// Cluster query failed (e.g., IPC permission denied, running in LXC)
			// Fall back to localhost-only validation instead of permissive mode
			v.recordFailure(validationReasonClusterFailed)
			log.Warn().
				Err(err).
				Str("node", node).
				Msg("SECURITY: Cluster validation unavailable - falling back to localhost-only validation. Configure allowed_nodes for cluster-wide access.")
			// Attempt to validate against localhost addresses
			return v.validateAsLocalhost(ctx, node)
		} else if !allowed {
			return v.deny(node, validationReasonNotClusterMember)
		} else {
			return nil
		}
	}

	if v.strict {
		return v.deny(node, validationReasonNoSources)
	}

	return nil
}

func (v *nodeValidator) validateAsLocalhost(ctx context.Context, node string) error {
	// When cluster validation is unavailable, only allow access to localhost
	// This maintains security while allowing self-monitoring
	if ctx == nil {
		ctx = context.Background()
	}

	// Try to discover local host addresses
	localAddrs, err := discoverLocalHostAddresses()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to discover local host addresses for fallback validation")
		// If we can't even discover localhost, deny access
		return v.deny(node, validationReasonClusterFailed)
	}

	// Check if the requested node matches any local address
	normalized := normalizeAllowlistEntry(node)
	if normalized == "" {
		normalized = strings.ToLower(strings.TrimSpace(node))
	}

	for _, localAddr := range localAddrs {
		if strings.EqualFold(localAddr, normalized) {
			log.Debug().
				Str("node", node).
				Str("matched_local", localAddr).
				Msg("Node validated as localhost (cluster validation unavailable)")
			return nil
		}
	}

	// Node doesn't match any local address - deny
	return v.deny(node, "node_not_localhost")
}

func (v *nodeValidator) matchesAllowlist(ctx context.Context, node string) (bool, error) {
	normalized := normalizeAllowlistEntry(node)
	if normalized != "" {
		if _, ok := v.allowHosts[normalized]; ok {
			return true, nil
		}
	}

	if ip := parseNodeIP(node); ip != nil {
		if v.ipAllowed(ip) {
			return true, nil
		}
		// If the node itself is an IP and it didn't match, no need to resolve again.
		return false, nil
	}

	if len(v.allowCIDRs) == 0 {
		return false, nil
	}

	host := stripNodeDelimiters(node)
	ips, err := v.resolver.LookupIP(ctx, host)
	if err != nil {
		return false, fmt.Errorf("resolve node %q: %w", host, err)
	}

	for _, ip := range ips {
		if v.ipAllowed(ip) {
			return true, nil
		}
	}

	return false, nil
}

func (v *nodeValidator) matchesCluster(ctx context.Context, node string) (bool, error) {
	if v.clusterFetcher == nil {
		return false, errors.New("cluster membership disabled")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	members, err := v.getClusterMembers(ctx)
	if err != nil {
		return false, err
	}

	normalized := normalizeAllowlistEntry(node)
	if normalized == "" {
		normalized = strings.ToLower(strings.TrimSpace(node))
	}

	_, ok := members[normalized]
	return ok, nil
}

func (v *nodeValidator) getClusterMembers(ctx context.Context) (map[string]struct{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()
	if v.clock != nil {
		now = v.clock()
	}

	v.clusterCache.mu.Lock()
	defer v.clusterCache.mu.Unlock()

	if v.clusterCache.nodes != nil && now.Before(v.clusterCache.expires) {
		return v.clusterCache.nodes, nil
	}

	nodes, err := v.clusterFetcher()
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(nodes))
	resolvedHosts := make(map[string]struct{})
	for _, node := range nodes {
		if normalized := normalizeAllowlistEntry(node); normalized != "" {
			result[normalized] = struct{}{}
		}

		host := stripNodeDelimiters(strings.TrimSpace(node))
		if host == "" {
			continue
		}

		if net.ParseIP(host) != nil {
			continue
		}

		if _, seen := resolvedHosts[host]; seen {
			continue
		}
		resolvedHosts[host] = struct{}{}

		if v.resolver == nil {
			continue
		}

		ips, err := v.resolver.LookupIP(ctx, host)
		if err != nil {
			log.Debug().
				Str("host", host).
				Err(err).
				Msg("Failed to resolve cluster node hostname to IP")
			continue
		}

		for _, ip := range ips {
			if ip == nil {
				continue
			}
			result[ip.String()] = struct{}{}
		}
	}

	ttl := v.cacheTTL
	if ttl <= 0 {
		ttl = nodeValidatorCacheTTL
	}
	v.clusterCache.nodes = result
	v.clusterCache.expires = now.Add(ttl)
	log.Debug().
		Int("cluster_node_count", len(result)).
		Msg("Refreshed cluster membership cache")

	return result, nil
}

func (v *nodeValidator) ipAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if _, ok := v.allowHosts[ip.String()]; ok {
		return true
	}
	for _, network := range v.allowCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (v *nodeValidator) recordFailure(reason string) {
	if v.metrics != nil {
		v.metrics.recordNodeValidationFailure(reason)
	}
}

func (v *nodeValidator) deny(node, reason string) error {
	v.recordFailure(reason)
	log.Warn().
		Str("node", node).
		Str("reason", reason).
		Msg("potential SSRF attempt blocked")
	return fmt.Errorf("node %q rejected by validator (%s)", node, reason)
}

func normalizeAllowlistEntry(entry string) string {
	candidate := strings.TrimSpace(entry)
	if candidate == "" {
		return ""
	}
	unwrapped := stripNodeDelimiters(candidate)
	if ip := net.ParseIP(unwrapped); ip != nil {
		return ip.String()
	}
	return strings.ToLower(candidate)
}

func parseNodeIP(node string) net.IP {
	clean := stripNodeDelimiters(strings.TrimSpace(node))
	return net.ParseIP(clean)
}

func stripNodeDelimiters(node string) string {
	if strings.HasPrefix(node, "[") && strings.HasSuffix(node, "]") && len(node) > 2 {
		return node[1 : len(node)-1]
	}
	return node
}
