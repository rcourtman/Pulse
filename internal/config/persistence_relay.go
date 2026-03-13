package config

import (
	"errors"
	"io/fs"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rs/zerolog/log"
)

// SaveRelayConfig stores relay settings, encrypting when a crypto manager is available.
func (c *ConfigPersistence) SaveRelayConfig(cfg relay.Config) error {
	if err := saveJSON(c, c.relayFile, cfg, true); err != nil {
		return err
	}

	log.Info().Str("file", c.relayFile).Bool("enabled", cfg.Enabled).Msg("Relay configuration saved")
	return nil
}

// LoadRelayConfig retrieves the persisted relay settings. Returns default config if none exists.
func (c *ConfigPersistence) LoadRelayConfig() (*relay.Config, error) {
	cfg := relay.DefaultConfig()
	if err := loadJSON(c, c.relayFile, true, cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return relay.DefaultConfig(), nil
		}
		return nil, err
	}

	return cfg, nil
}
