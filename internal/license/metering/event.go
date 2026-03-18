package metering

import pkgmetering "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"

type EventType = pkgmetering.EventType

const (
	EventAgentSeen  = pkgmetering.EventAgentSeen
	EventRelayBytes = pkgmetering.EventRelayBytes
)

type Event = pkgmetering.Event
type AggregatedBucket = pkgmetering.AggregatedBucket
