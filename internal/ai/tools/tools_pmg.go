package tools

import (
	"context"
	"fmt"
)

// registerPMGTools registers Proxmox Mail Gateway query tools
func (e *PulseToolExecutor) registerPMGTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_pmg_status",
			Description: `Get Proxmox Mail Gateway instance status and health.

Returns: JSON with instances array containing id, name, host, status, version, and nodes (with status, role, uptime, load).

Use when: User asks about mail gateway status, PMG health, or mail server infrastructure.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: specific PMG instance name or ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetPMGStatus(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_mail_stats",
			Description: `Get mail flow statistics (counts, spam, virus, bounces).

Returns: JSON with mail statistics including total in/out, spam in/out, virus counts, bounces, greylist count, and average processing time.

Use when: User asks about mail flow, email statistics, spam counts, or virus detections.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: specific PMG instance name or ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetMailStats(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_mail_queues",
			Description: `Get mail queue status (active, deferred, hold).

Returns: JSON with queue status for each node including active, deferred, hold, incoming counts and oldest message age.

Use when: User asks about mail queues, deferred messages, mail delivery issues, or queue backlogs.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: specific PMG instance name or ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetMailQueues(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_spam_stats",
			Description: `Get spam quarantine statistics and distribution.

Returns: JSON with quarantine counts (spam, virus, attachment, blacklisted) and spam score distribution buckets.

Use when: User asks about spam statistics, quarantine status, or spam score distribution.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: specific PMG instance name or ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetSpamStats(ctx, args)
		},
	})
}

func (e *PulseToolExecutor) executeGetPMGStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	var instances []PMGInstanceSummary
	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
			continue
		}

		var nodes []PMGNodeSummary
		for _, node := range pmg.Nodes {
			nodes = append(nodes, PMGNodeSummary{
				Name:    node.Name,
				Status:  node.Status,
				Role:    node.Role,
				Uptime:  node.Uptime,
				LoadAvg: node.LoadAvg,
			})
		}

		instances = append(instances, PMGInstanceSummary{
			ID:      pmg.ID,
			Name:    pmg.Name,
			Host:    pmg.Host,
			Status:  pmg.Status,
			Version: pmg.Version,
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

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	// If filtering, find that specific instance
	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
			continue
		}

		if pmg.MailStats == nil {
			if instanceFilter != "" {
				return NewTextResult(fmt.Sprintf("No mail statistics available for PMG instance '%s'.", instanceFilter)), nil
			}
			continue
		}

		response := MailStatsResponse{
			Instance: pmg.Name,
			Stats: PMGMailStatsSummary{
				Timeframe:            pmg.MailStats.Timeframe,
				TotalIn:              pmg.MailStats.CountIn,
				TotalOut:             pmg.MailStats.CountOut,
				SpamIn:               pmg.MailStats.SpamIn,
				SpamOut:              pmg.MailStats.SpamOut,
				VirusIn:              pmg.MailStats.VirusIn,
				VirusOut:             pmg.MailStats.VirusOut,
				BouncesIn:            pmg.MailStats.BouncesIn,
				BouncesOut:           pmg.MailStats.BouncesOut,
				BytesIn:              pmg.MailStats.BytesIn,
				BytesOut:             pmg.MailStats.BytesOut,
				GreylistCount:        pmg.MailStats.GreylistCount,
				RBLRejects:           pmg.MailStats.RBLRejects,
				AverageProcessTimeMs: pmg.MailStats.AverageProcessTimeMs,
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

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	// Collect queue data from all instances (or filtered instance)
	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
			continue
		}

		var queues []PMGQueueSummary
		for _, node := range pmg.Nodes {
			if node.QueueStatus != nil {
				queues = append(queues, PMGQueueSummary{
					Node:             node.Name,
					Active:           node.QueueStatus.Active,
					Deferred:         node.QueueStatus.Deferred,
					Hold:             node.QueueStatus.Hold,
					Incoming:         node.QueueStatus.Incoming,
					Total:            node.QueueStatus.Total,
					OldestAgeSeconds: node.QueueStatus.OldestAge,
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
			Instance: pmg.Name,
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

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
			continue
		}

		quarantine := PMGQuarantineSummary{}
		if pmg.Quarantine != nil {
			quarantine = PMGQuarantineSummary{
				Spam:        pmg.Quarantine.Spam,
				Virus:       pmg.Quarantine.Virus,
				Attachment:  pmg.Quarantine.Attachment,
				Blacklisted: pmg.Quarantine.Blacklisted,
				Total:       pmg.Quarantine.Spam + pmg.Quarantine.Virus + pmg.Quarantine.Attachment + pmg.Quarantine.Blacklisted,
			}
		}

		var distribution []PMGSpamBucketSummary
		for _, bucket := range pmg.SpamDistribution {
			distribution = append(distribution, PMGSpamBucketSummary{
				Score: bucket.Score,
				Count: bucket.Count,
			})
		}

		response := SpamStatsResponse{
			Instance:     pmg.Name,
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
