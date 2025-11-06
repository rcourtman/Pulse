package dockeragent

// Version is the semantic version of the Pulse Docker agent binary. It is
// overridden at build time via -ldflags for release artifacts. When building
// from source without ldflags, it defaults to "dev" to prevent auto-update
// loops in development builds.
var Version = "dev"
