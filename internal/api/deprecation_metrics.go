package api

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const legacyHostAliasLogInterval = 15 * time.Minute

var legacyHostAliasLastLogged sync.Map

func noteLegacyHostAliasUsage(aliasPath string) {
	recordDeprecatedAPIUsage("host_agent_api_alias", aliasPath)

	now := time.Now()
	if lastRaw, ok := legacyHostAliasLastLogged.Load(aliasPath); ok {
		if last, ok := lastRaw.(time.Time); ok && now.Sub(last) < legacyHostAliasLogInterval {
			return
		}
	}

	legacyHostAliasLastLogged.Store(aliasPath, now)
	log.Warn().
		Str("path", aliasPath).
		Msg("Deprecated host-agent API alias used; upgrade agents to canonical /api/agents/agent/* endpoints")
}
