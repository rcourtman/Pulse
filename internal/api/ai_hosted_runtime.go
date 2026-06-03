package api

import (
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func loadHostedAwareAIConfig(hostedMode bool, billingBaseDir, orgID string, persistence *config.ConfigPersistence) (*config.AIConfig, error) {
	if persistence == nil {
		return nil, fmt.Errorf("Pulse Assistant config persistence unavailable")
	}

	cfg, err := persistence.LoadAIConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func hostedModeEnabledFromEnv() bool {
	return os.Getenv("PULSE_HOSTED_MODE") == "true"
}
