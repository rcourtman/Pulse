package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// VMAgentInfo summarizes the bounded version and file-read capability reported
// by the QEMU guest agent's /agent/info endpoint.
type VMAgentInfo struct {
	Version              string
	HasSupportedCommands bool
	FileReadDisabled     bool
}

// GetVMAgentInfoSummary returns the guest agent version and whether /agent/info
// explicitly reports guest-file-open or guest-file-read disabled for this VM.
func (c *Client) GetVMAgentInfoSummary(ctx context.Context, node string, vmid int) (VMAgentInfo, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/info", node, vmid))
	if err != nil {
		return VMAgentInfo{}, fmt.Errorf("guest agent info: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Result json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VMAgentInfo{}, err
	}

	return parseVMAgentInfoResult(result.Data.Result), nil
}

// GetVMAgentInfoSummary executes the /agent/info read through the cluster
// client's single-attempt guest-agent admission path.
func (cc *ClusterClient) GetVMAgentInfoSummary(ctx context.Context, node string, vmid int) (VMAgentInfo, error) {
	var info VMAgentInfo
	err := cc.executeGuestAgent(ctx, func(c *Client) error {
		var callErr error
		info, callErr = c.GetVMAgentInfoSummary(ctx, node, vmid)
		return callErr
	})
	return info, err
}

func parseVMAgentInfoResult(raw json.RawMessage) VMAgentInfo {
	fields, ok := guestAgentFields(raw, "version", "qemu-ga", "supported_commands", "supported-commands")
	if !ok {
		return VMAgentInfo{}
	}

	extractVersion := func(val json.RawMessage) string {
		if v, ok := guestAgentScalar(val).(string); ok {
			return strings.TrimSpace(v)
		}
		nested, _ := guestAgentScalarFields(val, "version")
		if s, ok := nested["version"].(string); ok {
			return strings.TrimSpace(s)
		}
		return ""
	}

	var summary VMAgentInfo
	if version := extractVersion(fields["version"]); version != "" {
		summary.Version = version
	} else if version := extractVersion(fields["qemu-ga"]); version != "" {
		summary.Version = version
	}

	rawCommands := fields["supported_commands"]
	if len(rawCommands) == 0 {
		rawCommands = fields["supported-commands"]
	}
	rawCommands = bytes.TrimSpace(rawCommands)
	if len(rawCommands) == 0 || bytes.Equal(rawCommands, []byte("null")) {
		return summary
	}

	var commands guestAgentList
	if err := json.Unmarshal(rawCommands, &commands); err != nil || commands == nil {
		return summary
	}

	summary.HasSupportedCommands = true
	for _, entry := range commands {
		scalars, ok := guestAgentScalarFields(entry, "name", "enabled")
		if !ok {
			continue
		}
		name, ok := scalars["name"].(string)
		if !ok || (name != "guest-file-open" && name != "guest-file-read") {
			continue
		}
		if enabled, ok := scalars["enabled"].(bool); ok && !enabled {
			summary.FileReadDisabled = true
			break
		}
	}
	return summary
}
