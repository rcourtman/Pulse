package main

import (
	"context"
	"fmt"
	"os"
	"time"
	
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	
	// Load config
	appConfig, err := config.Load()
	if err != nil || len(appConfig.PVEInstances) == 0 {
		log.Fatal().Msg("No PVE instances configured")
	}
	
	// Test first instance
	pveConfig := appConfig.PVEInstances[0]
	cfg := proxmox.ClientConfig{
		Host:       pveConfig.Host,
		User:       pveConfig.User,
		Password:   pveConfig.Password,
		TokenName:  pveConfig.TokenName,
		TokenValue: pveConfig.TokenValue,
	}
	
	// Create client
	client, err := proxmox.NewClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create client")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Get nodes
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get nodes")
	}
	
	fmt.Printf("Testing ZFS integration on %d nodes\n", len(nodes))
	
	for _, node := range nodes {
		if node.Status != "online" {
			fmt.Printf("Skipping offline node: %s\n", node.Node)
			continue
		}
		
		fmt.Printf("\n=== Node: %s ===\n", node.Node)
		
		// Test the new ZFS function
		pools, err := client.GetZFSPoolsWithDetails(ctx, node.Node)
		if err != nil {
			log.Error().Err(err).Str("node", node.Node).Msg("Failed to get ZFS pools")
			continue
		}
		
		fmt.Printf("Found %d ZFS pools\n", len(pools))
		
		for _, pool := range pools {
			fmt.Printf("\nPool: %s\n", pool.Name)
			fmt.Printf("  Health: %s\n", pool.Health)
			fmt.Printf("  State: %s\n", pool.State)
			fmt.Printf("  Status: %s\n", pool.Status)
			fmt.Printf("  Errors: %s\n", pool.Errors)
			fmt.Printf("  Scan: %s\n", pool.Scan)
			
			// Convert to model
			modelPool := pool.ConvertToModelZFSPool()
			if modelPool != nil {
				fmt.Printf("  Model State: %s\n", modelPool.State)
				fmt.Printf("  Total Errors: Read=%d, Write=%d, Checksum=%d\n",
					modelPool.ReadErrors, modelPool.WriteErrors, modelPool.ChecksumErrors)
				
				if len(modelPool.Devices) > 0 {
					fmt.Printf("  Devices with issues:\n")
					for _, dev := range modelPool.Devices {
						fmt.Printf("    - %s (%s): State=%s, Errors: R=%d W=%d C=%d\n",
							dev.Name, dev.Type, dev.State,
							dev.ReadErrors, dev.WriteErrors, dev.ChecksumErrors)
					}
				} else {
					fmt.Printf("  All devices healthy\n")
				}
			}
		}
		
		// Get storage to see ZFS storage
		storage, err := client.GetStorage(ctx, node.Node)
		if err != nil {
			log.Error().Err(err).Str("node", node.Node).Msg("Failed to get storage")
			continue
		}
		
		fmt.Printf("\nZFS Storage on this node:\n")
		for _, s := range storage {
			if s.Type == "zfspool" || s.Type == "zfs" {
				fmt.Printf("  - %s (type=%s, active=%d)\n", s.Storage, s.Type, s.Active)
			}
		}
	}
	
	fmt.Println("\n✓ ZFS integration test complete")
}