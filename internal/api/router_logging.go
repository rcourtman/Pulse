package api

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type routerEventLogger interface {
	Error() *zerolog.Event
	Warn() *zerolog.Event
	Info() *zerolog.Event
}

func (r *Router) logEvents() routerEventLogger {
	if r != nil && r.eventLogger != nil {
		return r.eventLogger
	}
	return &log.Logger
}
