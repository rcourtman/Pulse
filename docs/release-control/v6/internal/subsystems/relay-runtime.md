# Relay Runtime Contract

## Contract Metadata

```json
{
  "subsystem_id": "relay-runtime",
  "lane": "L7",
  "contract_file": "docs/release-control/v6/internal/subsystems/relay-runtime.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own the desktop relay runtime, its persisted client configuration, and the
canonical reconnect, encryption, and relay-trust behavior for Pulse instance
bridging.

## Canonical Files

1. `internal/relay/client.go`
2. `internal/config/persistence_relay.go`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change desktop relay reconnect, registration, drain, proxy-stream, or encrypted channel behavior through `internal/relay/`
2. Add or change persisted relay enablement, server URL, or reconnect-safe default loading through `internal/config/persistence_relay.go`
3. Keep desktop relay changes aligned with the governed mobile and server relay surfaces represented by the L7 lane evidence

## Forbidden Paths

1. Reintroducing relay client trust or TLS handling through ad hoc dialer configuration outside the canonical relay client
2. Letting encrypted DATA frames race ahead of nonce-order guarantees by deferring inbound decrypt work to background goroutines
3. Treating a missing persisted relay config file as a hard failure instead of falling back to the canonical disabled default

## Completion Obligations

1. Update this contract when new desktop relay runtime or persistence entry points become canonical
2. Keep relay client changes tied to explicit runtime proof in `internal/relay/client_test.go`
3. Keep persisted relay config loading tied to explicit proof in `internal/config/persistence_relay_test.go`

## Current State

The desktop relay client had been carrying real runtime behavior without any
explicit subsystem ownership even though L7 already treats relay readiness as a
governed release lane. This contract makes that desktop boundary explicit.
The canonical relay runtime now includes local CA-bundle trust via
`SSL_CERT_FILE`, so self-hosted relay deployments can use a private CA without
forking the dial path or disabling TLS verification globally.
Inbound DATA handling is also part of this boundary: encrypted relay frames
must be decrypted before they are handed to background stream handlers so the
channel nonce guard stays monotonic and relay arrival order cannot trip false
replay failures under concurrency.
Relay status truthfulness is part of the same contract. If the client is in a
governed reconnect or license-pause window, `ClientStatus.reconnect_in` must
reflect that delay instead of silently presenting a disconnected state with no
retry timing.
The relay HTTP proxy boundary is also part of this owned surface. Relay DATA
requests must normalize outer method/path whitespace before building the local
Pulse API request so trivial spacing drift does not cause a valid proxied
operation to fail at the desktop boundary. The same normalization rule applies
to allowlisted proxy header names, so safe headers like `Content-Type` are not
silently stripped just because the upstream frame wrapped them in outer
whitespace.
Persisted relay config loading is part of the same owned surface. A missing
`relay.enc` file must fall back cleanly to the canonical disabled default so a
fresh v6 install or a partially migrated instance does not fail closed just
because relay settings have never been saved.
That same persistence boundary also owns relay client secrets at rest:
`relay.enc` may only accept plaintext relay config as migration input. Once
the runtime can read legacy plaintext relay settings, it must rewrite
canonical encrypted storage immediately on load instead of leaving
`instance_secret` or the relay identity private key on disk as a normal
runtime path.
