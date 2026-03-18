package monitoring

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

// PollResult represents the result of a polling operation
type PollResult struct {
	InstanceName string
	InstanceType string // "pve", "pbs", or "pmg"
	Success      bool
	Error        error
	StartTime    time.Time
	EndTime      time.Time
}

// PollTask represents a polling task to be executed
type PollTask struct {
	InstanceName string
	InstanceType string // "pve", "pbs", or "pmg"
	Run          func(ctx context.Context)
	PVEClient    PVEClientInterface
	PBSClient    *pbs.Client
	PMGClient    *pmg.Client
}
