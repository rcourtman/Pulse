package main

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

func main() {
	// Initialize logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	
	if err := config.MigrateToUnified("/etc/pulse", "/etc/pulse/pulse.yml"); err != nil {
		log.Fatal().Err(err).Msg("Migration failed")
	}
}