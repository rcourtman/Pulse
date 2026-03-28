package portal

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
)

type CommercialIdentity struct {
	HasCommercialIdentity bool
}

type CommercialIdentityLookup func(ctx context.Context, email string) (*CommercialIdentity, error)

func resolveSelfHostedCommercial(ctx context.Context, lookup CommercialIdentityLookup, email string, accounts []portalPageAccount) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	if lookup == nil {
		return len(accounts) == 0
	}

	identity, err := lookup(ctx, email)
	if err != nil {
		log.Warn().Err(err).Str("email", email).Msg("cloudcp.portal: commercial identity lookup failed")
		return len(accounts) == 0
	}
	if identity == nil {
		return false
	}
	return identity.HasCommercialIdentity
}
