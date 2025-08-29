package mock

import (
	"os"
	"strconv"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

var (
	mockData       models.StateSnapshot
	mockAlerts     []models.Alert
	mockEnabled    bool
	lastUpdate     time.Time
	updateInterval = 2 * time.Second
)

func init() {
	// Check if mock mode is enabled
	mockEnabled = os.Getenv("PULSE_MOCK_MODE") == "true"
	
	if mockEnabled {
		log.Info().Msg("Mock mode enabled - using simulated data")
		
		// Load configuration from env vars or use defaults
		config := LoadMockConfig()
		
		// Generate initial mock data
		mockData = GenerateMockData(config)
		mockAlerts = GenerateAlerts(mockData.Nodes, mockData.VMs, mockData.Containers)
		lastUpdate = time.Now()
		
		// Start update ticker
		go func() {
			ticker := time.NewTicker(updateInterval)
			defer ticker.Stop()
			
			for range ticker.C {
				if mockEnabled {
					UpdateMetrics(&mockData, config)
					// Occasionally regenerate alerts
					if time.Now().Unix()%30 == 0 {
						mockAlerts = GenerateAlerts(mockData.Nodes, mockData.VMs, mockData.Containers)
					}
				}
			}
		}()
	}
}

// LoadMockConfig loads mock configuration from environment variables
func LoadMockConfig() MockConfig {
	config := DefaultConfig
	
	if val := os.Getenv("PULSE_MOCK_NODES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			config.NodeCount = n
		}
	}
	
	if val := os.Getenv("PULSE_MOCK_VMS_PER_NODE"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			config.VMsPerNode = n
		}
	}
	
	if val := os.Getenv("PULSE_MOCK_LXCS_PER_NODE"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			config.LXCsPerNode = n
		}
	}
	
	if val := os.Getenv("PULSE_MOCK_RANDOM_METRICS"); val != "" {
		config.RandomMetrics = val == "true"
	}
	
	if val := os.Getenv("PULSE_MOCK_STOPPED_PERCENT"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			config.StoppedPercent = f / 100.0
		}
	}
	
	log.Info().
		Int("nodes", config.NodeCount).
		Int("vms_per_node", config.VMsPerNode).
		Int("lxcs_per_node", config.LXCsPerNode).
		Bool("random_metrics", config.RandomMetrics).
		Float64("stopped_percent", config.StoppedPercent).
		Msg("Mock configuration loaded")
	
	return config
}

// IsMockEnabled returns whether mock mode is enabled
func IsMockEnabled() bool {
	return mockEnabled
}

// GetMockState returns the current mock state snapshot
func GetMockState() models.StateSnapshot {
	if !mockEnabled {
		return models.StateSnapshot{}
	}
	
	// Return the current mock data with alerts
	mockData.ActiveAlerts = mockAlerts
	return mockData
}

// ToggleMockMode enables or disables mock mode at runtime
func ToggleMockMode(enable bool) {
	if enable && !mockEnabled {
		mockEnabled = true
		config := LoadMockConfig()
		mockData = GenerateMockData(config)
		mockAlerts = GenerateAlerts(mockData.Nodes, mockData.VMs, mockData.Containers)
		log.Info().Msg("Mock mode enabled dynamically")
	} else if !enable && mockEnabled {
		mockEnabled = false
		log.Info().Msg("Mock mode disabled dynamically")
	}
}

// SetMockConfig updates the mock configuration dynamically
func SetMockConfig(nodeCount, vmsPerNode, lxcsPerNode int) {
	if !mockEnabled {
		return
	}
	
	config := MockConfig{
		NodeCount:      nodeCount,
		VMsPerNode:     vmsPerNode,
		LXCsPerNode:    lxcsPerNode,
		RandomMetrics:  true,
		StoppedPercent: 0.2,
	}
	
	mockData = GenerateMockData(config)
	mockAlerts = GenerateAlerts(mockData.Nodes, mockData.VMs, mockData.Containers)
	
	log.Info().
		Int("nodes", nodeCount).
		Int("vms", vmsPerNode).
		Int("lxcs", lxcsPerNode).
		Msg("Mock configuration updated")
}