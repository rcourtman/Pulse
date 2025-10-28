package dockeragent

// Version is the semantic version of the Pulse Docker agent binary. It is
// overridden at build time via -ldflags for release artifacts. When building
// from source without ldflags, it defaults to this development value.
// Set to match deployed agents to prevent update loops in development.
var Version = "v4.30.0"
