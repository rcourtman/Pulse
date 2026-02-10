package tools

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

	rs := e.getReadState()
	state := e.stateProvider.GetState()

	// Prefer ReadState for instance selection when available.
	var wantID, wantName string
	if rs != nil && instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.ID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	if rs == nil && len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}
	if rs != nil && len(rs.PMGInstances()) == 0 && len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	var instances []PMGInstanceSummary
	if rs != nil {
		// Build results in ReadState order; enrich from StateSnapshot when available.
		for _, v := range rs.PMGInstances() {
			if instanceFilter != "" && v.ID() != instanceFilter && v.Name() != instanceFilter && v.ID() != wantID && v.Name() != wantName {
				continue
			}

			var fromState *models.PMGInstance
			for i := range state.PMGInstances {
				p := &state.PMGInstances[i]
				if (wantID != "" && p.ID == wantID) || (wantName != "" && p.Name == wantName) || p.ID == v.ID() || p.Name == v.Name() {
					fromState = p
					break
				}
			}

			var nodes []PMGNodeSummary
			if fromState != nil {
				for _, node := range fromState.Nodes {
					nodes = append(nodes, PMGNodeSummary{
						Name:    node.Name,
						Status:  node.Status,
						Role:    node.Role,
						Uptime:  node.Uptime,
						LoadAvg: node.LoadAvg,
					})
				}
			}

			id := v.ID()
			name := v.Name()
			host := v.Hostname()
			status := string(v.Status())
			version := v.Version()
			if fromState != nil {
				if fromState.ID != "" {
					id = fromState.ID
				}
				if fromState.Name != "" {
					name = fromState.Name
				}
				if fromState.Host != "" {
					host = fromState.Host
				}
				if fromState.Status != "" {
					status = fromState.Status
				}
				if fromState.Version != "" {
					version = fromState.Version
				}
			}

			instances = append(instances, PMGInstanceSummary{
				ID:      id,
				Name:    name,
				Host:    host,
				Status:  status,
				Version: version,
				Nodes:   nodes,
			})
		}
	} else {
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
	rs := e.getReadState()
	var wantID, wantName string
	if rs != nil && instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.ID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	// If filtering, find that specific instance
	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" {
			if (wantID != "" || wantName != "") && pmg.ID != wantID && pmg.Name != wantName {
				continue
			}
			if wantID == "" && wantName == "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
				continue
			}
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

	// Prefer ReadState for instance selection when available.
	rs := e.getReadState()
	var wantID, wantName string
	if rs != nil && instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.ID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	// Collect queue data from all instances (or filtered instance)
	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" {
			if (wantID != "" || wantName != "") && pmg.ID != wantID && pmg.Name != wantName {
				continue
			}
			if wantID == "" && wantName == "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
				continue
			}
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

	// Prefer ReadState for instance selection when available.
	rs := e.getReadState()
	var wantID, wantName string
	if rs != nil && instanceFilter != "" {
		found := false
		for _, v := range rs.PMGInstances() {
			if v.ID() == instanceFilter || v.Name() == instanceFilter {
				wantID = v.ID()
				wantName = v.Name()
				found = true
				break
			}
		}
		if !found {
			return NewTextResult(fmt.Sprintf("PMG instance '%s' not found.", instanceFilter)), nil
		}
	}

	state := e.stateProvider.GetState()

	if len(state.PMGInstances) == 0 {
		return NewTextResult("No Proxmox Mail Gateway instances found. PMG monitoring may not be configured."), nil
	}

	for _, pmg := range state.PMGInstances {
		if instanceFilter != "" {
			if (wantID != "" || wantName != "") && pmg.ID != wantID && pmg.Name != wantName {
				continue
			}
			if wantID == "" && wantName == "" && pmg.ID != instanceFilter && pmg.Name != instanceFilter {
				continue
			}
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
