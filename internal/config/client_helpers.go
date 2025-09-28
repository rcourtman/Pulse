package config

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// CreateProxmoxConfig creates a proxmox.ClientConfig from a PVEInstance
func CreateProxmoxConfig(node *PVEInstance) proxmox.ClientConfig {
	user := node.User
	if node.TokenName == "" && node.TokenValue == "" && user != "" && !strings.Contains(user, "@") {
		user = user + "@pam"
	}

	return proxmox.ClientConfig{
		Host:        node.Host,
		User:        user,
		Password:    node.Password,
		TokenName:   node.TokenName,
		TokenValue:  node.TokenValue,
		VerifySSL:   node.VerifySSL,
		Fingerprint: node.Fingerprint,
	}
}

// CreatePBSConfig creates a pbs.ClientConfig from a PBSInstance
func CreatePBSConfig(node *PBSInstance) pbs.ClientConfig {
	return pbs.ClientConfig{
		Host:        node.Host,
		User:        node.User,
		Password:    node.Password,
		TokenName:   node.TokenName,
		TokenValue:  node.TokenValue,
		VerifySSL:   node.VerifySSL,
		Fingerprint: node.Fingerprint,
	}
}

// CreateProxmoxConfigFromFields creates a proxmox.ClientConfig from individual fields
func CreateProxmoxConfigFromFields(host, user, password, tokenName, tokenValue, fingerprint string, verifySSL bool) proxmox.ClientConfig {
	if tokenName == "" && tokenValue == "" && user != "" && !strings.Contains(user, "@") {
		user = user + "@pam"
	}

	return proxmox.ClientConfig{
		Host:        host,
		User:        user,
		Password:    password,
		TokenName:   tokenName,
		TokenValue:  tokenValue,
		VerifySSL:   verifySSL,
		Fingerprint: fingerprint,
	}
}

// CreatePBSConfigFromFields creates a pbs.ClientConfig from individual fields
func CreatePBSConfigFromFields(host, user, password, tokenName, tokenValue, fingerprint string, verifySSL bool) pbs.ClientConfig {
	return pbs.ClientConfig{
		Host:        host,
		User:        user,
		Password:    password,
		TokenName:   tokenName,
		TokenValue:  tokenValue,
		VerifySSL:   verifySSL,
		Fingerprint: fingerprint,
	}
}
