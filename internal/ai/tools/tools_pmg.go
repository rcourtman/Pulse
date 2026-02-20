package tools

import (
	"context"
	"fmt"
)

// registerPMGTools registers the pulse_pmg tool
func (e *PulseToolExecutor) registerPMGTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_pmg",
			Description: `Query Proxmox Mail Gateway status and statistics. Types: status, mail_stats, queues, spam.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "PMG query type",
						Enum:        []string{"status", "mail_stats", "queues", "spam"},
					},
					"instance": {
						Type:        "string",
						Description: "Optional: specific PMG instance name or ID",
					},
				},
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executePMG(ctx, args)
		},
	})
}

// executePMG routes to the appropriate PMG handler based on type
func (e *PulseToolExecutor) executePMG(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	pmgType, _ := args["type"].(string)
	switch pmgType {
	case "status":
		return e.executeGetPMGStatus(ctx, args)
	case "mail_stats":
		return e.executeGetMailStats(ctx, args)
	case "queues":
		return e.executeGetMailQueues(ctx, args)
	case "spam":
		return e.executeGetSpamStats(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: status, mail_stats, queues, spam", pmgType)), nil
	}
}

func (e *PulseToolExecutor) executeGetPMGStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)

	rs, _ := e.readStateForControl()

	// Prefer ReadState for instance selection when available.
	var wantID, wantName string
	if instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.InstanceID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.InstanceID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	if len(rs.PMGInstances()) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	var instances []PMGInstanceSummary
	for _, v := range rs.PMGInstances() {
		if instanceFilter != "" && v.ID() != instanceFilter && v.InstanceID() != instanceFilter && v.Name() != instanceFilter && v.InstanceID() != wantID && v.Name() != wantName {
			continue
		}

		var nodes []PMGNodeSummary
		for _, node := range v.Nodes() {
			nodes = append(nodes, PMGNodeSummary{
				Name:    node.Name,
				Status:  node.Status,
				Role:    node.Role,
				Uptime:  node.Uptime,
				LoadAvg: node.LoadAvg,
			})
		}

		instances = append(instances, PMGInstanceSummary{
			ID:      v.ID(),
			Name:    v.Name(),
			Host:    v.Hostname(),
			Status:  string(v.Status()),
			Version: v.Version(),
			Nodes:   nodes,
		})
	}

	if len(instances) == 0 && instanceFilter != "" {
		return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
	}

	// Ensure non-nil slices
	for i := range instances {
		if instances[i].Nodes == nil {
			instances[i].Nodes = []PMGNodeSummary{}
		}
	}

	response := PMGStatusResponse{
		Instances: instances,
		Total:     len(instances),
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetMailStats(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)

	// Prefer ReadState for instance selection when available.
	rs, _ := e.readStateForControl()
	var wantID, wantName string
	if instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.InstanceID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.InstanceID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	if len(rs.PMGInstances()) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	// If filtering, find that specific instance
	for _, pmg := range rs.PMGInstances() {
		if instanceFilter != "" {
			if (wantID != "" || wantName != "") && pmg.ID() != wantID && pmg.Name() != wantName {
				continue
			}
			if wantID == "" && wantName == "" && pmg.ID() != instanceFilter && pmg.Name() != instanceFilter {
				continue
			}
		}

		stats := pmg.MailStats()
		if stats == nil {
			if instanceFilter != "" {
				return NewTextResult(fmt.Sprintf("No mail statistics available for PMG instance '%s'.", instanceFilter)), nil
			}
			continue
		}

		response := MailStatsResponse{
			Instance: pmg.Name(),
			Stats: PMGMailStatsSummary{
				Timeframe:            stats.Timeframe,
				TotalIn:              stats.CountIn,
				TotalOut:             stats.CountOut,
				SpamIn:               stats.SpamIn,
				SpamOut:              stats.SpamOut,
				VirusIn:              stats.VirusIn,
				VirusOut:             stats.VirusOut,
				BouncesIn:            stats.BouncesIn,
				BouncesOut:           stats.BouncesOut,
				BytesIn:              stats.BytesIn,
				BytesOut:             stats.BytesOut,
				GreylistCount:        stats.GreylistCount,
				RBLRejects:           stats.RBLRejects,
				AverageProcessTimeMs: stats.AverageProcessTimeMs,
			},
		}

		return NewJSONResult(response), nil
	}

	if instanceFilter != "" {
		return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
	}

	return NewTextResult("No mail statistics available from any PMG instance."), nil
}

func (e *PulseToolExecutor) executeGetMailQueues(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)

	// Prefer ReadState for instance selection when available.
	rs, _ := e.readStateForControl()
	var wantID, wantName string
	if instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.InstanceID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.InstanceID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	if len(rs.PMGInstances()) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	// Collect queue data from all instances (or filtered instance)
	for _, pmg := range rs.PMGInstances() {
		if instanceFilter != "" {
			if (wantID != "" || wantName != "") && pmg.ID() != wantID && pmg.Name() != wantName {
				continue
			}
			if wantID == "" && wantName == "" && pmg.ID() != instanceFilter && pmg.Name() != instanceFilter {
				continue
			}
		}

		var queues []PMGQueueSummary
		for _, node := range pmg.Nodes() {
			if node.QueueStatus != nil {
				queues = append(queues, PMGQueueSummary{
					Node:             node.Name,
					Active:           node.QueueStatus.Active,
					Deferred:         node.QueueStatus.Deferred,
					Hold:             node.QueueStatus.Hold,
					Incoming:         node.QueueStatus.Incoming,
					Total:            node.QueueStatus.Total,
					OldestAgeSeconds: 0, // Age not provided in unified resource
				})
			}
		}

		if len(queues) == 0 {
			if instanceFilter != "" {
				return NewTextResult(fmt.Sprintf("No queue data available for PMG instance '%s'.", instanceFilter)), nil
			}
			continue
		}

		response := MailQueuesResponse{
			Instance: pmg.Name(),
			Queues:   queues,
		}

		return NewJSONResult(response), nil
	}

	if instanceFilter != "" {
		return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
	}

	return NewTextResult("No mail queue data available from any PMG instance."), nil
}

func (e *PulseToolExecutor) executeGetSpamStats(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)

	// Prefer ReadState for instance selection when available.
	rs, _ := e.readStateForControl()
	var wantID, wantName string
	if instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.InstanceID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.InstanceID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	if len(rs.PMGInstances()) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	for _, pmg := range rs.PMGInstances() {
		if instanceFilter != "" {
			if (wantID != "" || wantName != "") && pmg.ID() != wantID && pmg.Name() != wantName {
				continue
			}
			if wantID == "" && wantName == "" && pmg.ID() != instanceFilter && pmg.Name() != instanceFilter {
				continue
			}
		}

		quarantine := PMGQuarantineSummary{}
		qt := pmg.Quarantine()
		if qt != nil {
			quarantine = PMGQuarantineSummary{
				Spam:        qt.Spam,
				Virus:       qt.Virus,
				Attachment:  qt.Attachment,
				Blacklisted: qt.Blacklisted,
				Total:       qt.Spam + qt.Virus + qt.Attachment + qt.Blacklisted,
			}
		}

		var distribution []PMGSpamBucketSummary
		for _, bucket := range pmg.SpamDistribution() {
			distribution = append(distribution, PMGSpamBucketSummary{
				Score: bucket.Bucket,
				Count: bucket.Count,
			})
		}

		response := SpamStatsResponse{
			Instance:     pmg.Name(),
			Quarantine:   quarantine,
			Distribution: distribution,
		}

		if response.Distribution == nil {
			response.Distribution = []PMGSpamBucketSummary{}
		}

		return NewJSONResult(response), nil
	}

	if instanceFilter != "" {
		return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
	}

	return NewTextResult("No spam statistics available from any PMG instance."), nil
}
