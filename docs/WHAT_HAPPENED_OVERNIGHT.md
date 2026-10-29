# What happened overnight on my Proxmox and Docker fleet?

You wake up, a service is slow, and you want to know what changed while you
slept. An AI agent with shell access can read what each host still holds when
it asks (`pvesh`, `docker ps`, `journalctl`). Pulse kept watching all night,
so the agent can also ask it what changed across the whole fleet, and when.

This walkthrough connects Claude Code, Codex or OpenCode to Pulse with a
read-only token, asks that question, and shows what the agent gets back and
what it can't.

## What you need

Everything here is free with Pulse Community.

1. **Pulse v6.7.0 or later.** Earlier releases don't have
   `list_recent_changes` or `get_metric_history`. Install it with Docker or
   the Proxmox LXC installer from the [installation guide](INSTALL.md).
2. **Your systems added to Pulse.** Proxmox connects with a read-only API
   token under **Settings → Infrastructure**. Docker hosts need the Pulse
   agent: **Settings → Infrastructure → Install on a host**.
3. **`pulse-mcp`** on the machine where your agent runs. Install it with the
   [verified installer steps](AGENT_SUBSTRATE.md#install-the-mcp-adapter-safely),
   then open **Settings → API Access → Agent integrations**. That tab mints a
   `monitoring:read` token and generates the configuration for Claude Code and
   OpenCode. Other MCP clients, such as Codex, use the same command and token.

A `monitoring:read` token can read history and status. It can't change
anything, and it never receives command text or output.

## Ask the question

In your agent, ask in plain words:

> What happened on my Proxmox and Docker fleet overnight? Use Pulse.

Pulse gives the agent four reads for this. The agent chooses which to call;
here is what each returns, using Pulse's demo data.

### 1. What changed: `list_recent_changes`

The agent asks for changes since last night, for example
`since: 2026-10-28T22:00:00Z`. Without `since` it gets the last 24 hours.
Changes come back newest first: guests and hosts stopping, starting or
rebooting, containers restarting, resources appearing or disappearing, alerts
firing and resolving, and configuration or relationship changes. Here is one
alert, trimmed:

```json
{
  "observedAt": "2026-10-29T19:06:22Z",
  "resourceId": "agent-b9070eb05b18bb77",
  "resourceName": "Field Office Edge 01",
  "kind": "alert_fired",
  "reason": "Docker host 'Field Office Edge 01' is offline",
  "metadata": { "alert_level": "critical", "alert_type": "docker-host-offline" }
}
```

A busy night can match more than one read returns (up to 200). The response
then says `"truncated": true` and gives the full `count`. The agent should
narrow the read instead of paging: by `kind` (for example
`restart,state_transition,alert_fired`), by `sourceType`, or by one
`resourceId`.

### 2. How a metric moved: `get_metric_history`

Once a resource looks interesting, the agent can ask how its metrics moved,
for example whether a disk filled gradually or all at once, or whether memory
climbed before a restart. Points are averaged into buckets, each with its
minimum and maximum:

```json
{
  "resourceId": "nebula-1-mock",
  "range": "24h",
  "metrics": {
    "disk": [
      { "timestamp": 1793214000000, "value": 46.32, "min": 46.28, "max": 46.35 },
      { "timestamp": 1793221200000, "value": 46.50, "min": 46.38, "max": 46.62 }
    ]
  }
}
```

Community keeps seven days of metric history; a longer `range` returns
`license_required`.

### 3. Where things stand now: `get_fleet_context` and `get_resource_context`

`get_fleet_context` is one read across every resource, with finding and
approval counts, so the agent can see what still needs attention.
`get_resource_context` gives one resource in depth: its identity, the state
you set (maintenance window, intentionally offline), active findings and
recent actions.

## What Pulse can't tell you

The answer is only as good as what Pulse saw. Read these before trusting an
empty or short answer.

- **Pulse polls.** Proxmox is read every 10 seconds by default. A VM that
  stopped and started between two reads is still recorded as a restart,
  because its uptime reset. A status flap shorter than one read is not seen.
- **Reboots inside a guest.** A VM that rebooted from inside, without its QEMU
  process restarting, keeps its Proxmox uptime. Pulse sees that reboot only if
  the Pulse agent runs inside the guest.
- **While Pulse was down.** Nothing is recorded while Pulse itself is stopped
  or updating. When it starts again it compares each resource with what it
  last recorded. A state that changed in the gap is recorded once, as
  "resource state changed while Pulse was stopped", at the time Pulse noticed
  rather than when it happened. Resources that appeared in the gap are
  recorded as discovered. Removals and reboots during the gap are not
  recorded.
- **Logs and task history.** Pulse doesn't read system journals, container
  logs or the Proxmox task log. Your agent can still read those on the host.
- **Retention.** The change history keeps 30 days. Metric history keeps seven
  days on Community.
- **An empty answer is not proof that nothing happened.** It means Pulse
  recorded nothing in that window.

## Get changes as they happen

`pulse-mcp --emit-notifications` also forwards alerts firing and resolving
and status changes as they happen. Each event carries the `changeId` that
`list_recent_changes` returns, so the agent can read the history around it.
Not every MCP client shows these notifications; see
[What ships to help you connect](AGENT_SUBSTRATE.md#what-ships-to-help-you-connect).

## Tell us how it went

If this answered a real question for you, or failed to, a short note in
[GitHub Discussions](https://github.com/rcourtman/Pulse/discussions) helps
decide where this goes next. It's never required.
