package configapi

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// ClusterEndpointTrustOverrideRequest reconciles one existing dial authority,
// not a node-name alias or every member of the cluster. A required empty string
// explicitly selects the existing VerifySSL policy without a certificate pin.
// Missing/null fingerprints are not instructions to clear trust.
type ClusterEndpointTrustOverrideRequest struct {
	Endpoint    string  `json:"endpoint"`
	Fingerprint *string `json:"fingerprint"`
}

func applyPVEEndpointTrustUpdates(instance config.PVEInstance, req NodeConfigRequest, discovery config.DiscoveryConfig) (config.PVEInstance, error) {
	instance = instance.DeepCopy()
	endpoints, _, _ := monitoring.BuildPVEClusterClientEndpoints(instance, discovery)
	allowed := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		if key, ok := proxmox.ClusterEndpointIdentity(endpoint); ok {
			allowed[key] = true
		}
	}
	updates := make(map[string]string)
	for _, override := range req.ClusterEndpointTrustOverrides {
		key, ok := proxmox.ClusterEndpointIdentity(override.Endpoint)
		if !ok || !strings.HasPrefix(key, "https://") || !allowed[key] {
			return instance, fmt.Errorf("TLS trust can only be updated for an existing HTTPS endpoint; reload the connection before saving")
		}
		if override.Fingerprint == nil {
			return instance, fmt.Errorf("fingerprint is required for each TLS endpoint update")
		}
		if _, duplicate := updates[key]; duplicate {
			return instance, fmt.Errorf("TLS endpoint was provided more than once")
		}
		updates[key] = *override.Fingerprint
	}
	if req.hasFingerprintField() {
		if key, ok := proxmox.ClusterEndpointIdentity(instance.Host); ok {
			if other, exists := updates[key]; exists && other != req.Fingerprint {
				return instance, fmt.Errorf("primary and member TLS settings disagree for the same endpoint")
			}
			updates[key] = req.Fingerprint
		}
	}
	if len(updates) > 0 && instance.TLSFingerprintOverrides == nil {
		instance.TLSFingerprintOverrides = make(map[string]string)
	}
	for key, fingerprint := range updates {
		// Remove equivalent spellings only within this exact trust scope. A
		// different host, port, scheme or base path retains its own saved policy.
		for stored := range instance.TLSFingerprintOverrides {
			if storedKey, ok := proxmox.ClusterEndpointIdentity(stored); ok && storedKey == key {
				delete(instance.TLSFingerprintOverrides, stored)
			}
		}
		instance.TLSFingerprintOverrides[key] = fingerprint
	}
	return instance, nil
}

func pveConnectionTestTrust(instance config.PVEInstance, options nodeConnectionTestTLSOptions, discovery config.DiscoveryConfig) (string, string, error) {
	endpoint := instance.Host
	if options.Endpoint != "" {
		endpoint = options.Endpoint
	}
	key, valid := proxmox.ClusterEndpointIdentity(endpoint)
	if !valid {
		return "", "", fmt.Errorf("invalid PVE endpoint")
	}
	endpoints, pins, conflicts := monitoring.BuildPVEClusterClientEndpoints(instance, discovery)
	allowed := false
	for _, configured := range endpoints {
		configuredKey, ok := proxmox.ClusterEndpointIdentity(configured)
		if ok && key == configuredKey {
			endpoint = configured
			allowed = true
			break
		}
	}
	if !allowed {
		return "", "", fmt.Errorf("only existing PVE endpoints can be tested; reload the connection")
	}
	// A submitted pin is the exact prospective save for this endpoint. Omitted
	// TLS fields test saved polling policy, including ambiguity and explicit CA.
	if options.Fingerprint != nil {
		return endpoint, *options.Fingerprint, nil
	}
	fingerprint, known, err := proxmox.ResolveEndpointFingerprint(endpoint, pins, conflicts...)
	if err != nil {
		return "", "", err
	}
	if !known {
		fingerprint = instance.Fingerprint
	}
	return endpoint, fingerprint, nil
}

func pveClusterEndpointTLSResponses(instance config.PVEInstance, discovery config.DiscoveryConfig) []ClusterEndpointResponse {
	responses := toClusterEndpointResponses(instance.ClusterEndpoints, instance.ClusterNodeIdentities)
	_, pins, conflicts := monitoring.BuildPVEClusterClientEndpoints(instance, discovery)
	primary, primaryValid := proxmox.ClusterEndpointIdentity(instance.Host)
	for i, endpoint := range instance.ClusterEndpoints {
		target := monitoring.PVEClusterEndpointURL(endpoint, instance.VerifySSL, discovery)
		responses[i].TLSEndpoint = target
		key, valid := proxmox.ClusterEndpointIdentity(target)
		responses[i].UsesPrimaryTLS = valid && primaryValid && key == primary
		fingerprint, known, err := proxmox.ResolveEndpointFingerprint(target, pins, conflicts...)
		responses[i].TLSFirstUse = !known && instance.Fingerprint != "" && !responses[i].UsesPrimaryTLS
		if err == nil && known {
			responses[i].Fingerprint = fingerprint
		}
	}
	return responses
}

func pvePrimaryTLSFingerprint(instance config.PVEInstance) string {
	if fingerprint, known, err := proxmox.ResolveEndpointFingerprint(instance.Host, instance.TLSFingerprintOverrides); err == nil && known {
		return fingerprint
	}
	return instance.Fingerprint
}
