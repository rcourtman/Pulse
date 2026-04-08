// Package aws implements the hypervisor.Provider interface for Amazon Web Services.
//
// Uses the AWS SDK for Go v2 to monitor:
//   - EC2 instances → hypervisor.VM
//   - Availability Zones → hypervisor.Node (logical grouping)
//   - EBS volumes → hypervisor.Storage
//   - CloudWatch → metrics
//
// Required dependency: github.com/aws/aws-sdk-go-v2
package aws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rs/zerolog/log"
)

// Config holds AWS connection parameters.
type Config struct {
	ID        string
	Name      string
	Region    string // AWS region (e.g., "us-east-1")
	AccessKey string // AWS access key ID
	SecretKey string // AWS secret access key
	Profile   string // AWS CLI profile name (alternative to key/secret)
	RoleARN   string // IAM role to assume (for cross-account)
}

// Provider implements hypervisor.Provider for AWS.
type Provider struct {
	cfg     Config
	mu      sync.RWMutex
	healthy bool

	cachedNodes   []hypervisor.Node
	cachedVMs     []hypervisor.VM
	cachedStorage []hypervisor.Storage
	lastPoll      time.Time
}

// New creates a new AWS provider.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() hypervisor.ProviderType { return hypervisor.ProviderAWS }
func (p *Provider) ID() string                     { return p.cfg.ID }
func (p *Provider) Name() string                   { return p.cfg.Name }

// Connect initializes the AWS SDK client.
//
// Full implementation:
//
//	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
//	    awsconfig.WithRegion(p.cfg.Region),
//	    awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.cfg.AccessKey, p.cfg.SecretKey, "")),
//	)
//	p.ec2Client = ec2.NewFromConfig(awsCfg)
//	p.cwClient = cloudwatch.NewFromConfig(awsCfg)
func (p *Provider) Connect(ctx context.Context) error {
	if p.cfg.Region == "" {
		return fmt.Errorf("aws: region is required")
	}
	if p.cfg.AccessKey == "" && p.cfg.Profile == "" {
		return fmt.Errorf("aws: access key or profile is required")
	}

	log.Info().
		Str("region", p.cfg.Region).
		Str("id", p.cfg.ID).
		Msg("AWS provider connected (stub)")

	p.mu.Lock()
	p.healthy = true
	p.mu.Unlock()
	return nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthy = false
	return nil
}

func (p *Provider) Healthy(_ context.Context) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// GetNodes returns AWS Availability Zones as logical nodes.
//
// Full implementation:
//
//	output, _ := p.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
//	for _, az := range output.AvailabilityZones {
//	    // Map AZ to hypervisor.Node
//	}
func (p *Provider) GetNodes(_ context.Context) ([]hypervisor.Node, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("aws: not connected")
	}
	return p.cachedNodes, nil
}

// GetVMs returns EC2 instances as hypervisor.VM resources.
//
// Full implementation:
//
//	output, _ := p.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
//	for _, reservation := range output.Reservations {
//	    for _, inst := range reservation.Instances {
//	        // Map EC2 instance to hypervisor.VM
//	        // inst.InstanceId, inst.InstanceType, inst.State.Name, etc.
//	    }
//	}
func (p *Provider) GetVMs(_ context.Context, nodeID string) ([]hypervisor.VM, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("aws: not connected")
	}
	if nodeID == "" {
		return p.cachedVMs, nil
	}
	var filtered []hypervisor.VM
	for _, vm := range p.cachedVMs {
		if vm.NodeID == nodeID {
			filtered = append(filtered, vm)
		}
	}
	return filtered, nil
}

// GetContainers returns nil. ECS/EKS containers should be monitored via
// Pulse's existing Kubernetes/Docker agent integration.
func (p *Provider) GetContainers(_ context.Context, _ string) ([]hypervisor.Container, error) {
	return nil, nil
}

// GetStorage returns EBS volumes as storage resources.
//
// Full implementation:
//
//	output, _ := p.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{})
//	for _, vol := range output.Volumes {
//	    // Map EBS volume to hypervisor.Storage
//	}
func (p *Provider) GetStorage(_ context.Context, _ string) ([]hypervisor.Storage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("aws: not connected")
	}
	return p.cachedStorage, nil
}

// UpdateCache is called by the polling loop.
func (p *Provider) UpdateCache(nodes []hypervisor.Node, vms []hypervisor.VM, storage []hypervisor.Storage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedNodes = nodes
	p.cachedVMs = vms
	p.cachedStorage = storage
	p.lastPoll = time.Now()
}
