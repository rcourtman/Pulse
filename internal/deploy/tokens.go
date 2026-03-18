package deploy

import "time"

// Metadata key constants for bootstrap token binding.
const (
	MetaKeyClusterID    = "deploy:cluster_id"
	MetaKeyNodeID       = "deploy:node_id"
	MetaKeyExpectedNode = "deploy:expected_node_name"
	MetaKeyJobID        = "deploy:job_id"
	MetaKeyTargetID     = "deploy:target_id"
	MetaKeySourceAgent  = "deploy:source_agent_id"
)

// BootstrapTokenRequest contains the parameters needed to mint a bootstrap token.
type BootstrapTokenRequest struct {
	ClusterID     string
	NodeID        string
	ExpectedNode  string
	JobID         string
	TargetID      string
	SourceAgentID string
	OrgID         string
	TTL           time.Duration // Recommended: 10-30 minutes
}

// BuildMetadata returns the metadata map for token binding.
func (r BootstrapTokenRequest) BuildMetadata() map[string]string {
	return map[string]string{
		MetaKeyClusterID:    r.ClusterID,
		MetaKeyNodeID:       r.NodeID,
		MetaKeyExpectedNode: r.ExpectedNode,
		MetaKeyJobID:        r.JobID,
		MetaKeyTargetID:     r.TargetID,
		MetaKeySourceAgent:  r.SourceAgentID,
	}
}
