# Agent Registration Journey

## Goal

Validate the agent registration flow end-to-end: an agent sends its first
report, Pulse registers the agent, the agent appears in the unified state API, and
the agent is visible in the infrastructure UI.

## Steps

1. **Login** — Authenticate as admin via the Pulse UI or API.
2. **Register agent** — Send a POST to `/api/agents/agent/report` with a
   synthetic unified agent report payload containing `agent.type = "unified"`
   plus hostname, OS info, CPU, memory, disks, and network interfaces.
3. **Verify API state** — GET `/api/state` and confirm the new agent appears in
   the canonical `resources[]` collection as an `agent` resource with correct
   hostname, platform, and OS metadata.
4. **Verify metrics** — Confirm the agent's CPU usage and memory metrics are
   populated on that unified resource entry.
5. **Send follow-up report** — POST a second report with different CPU/memory
   values and verify `lastSeen` updates.
6. **Verify UI** — Navigate to `/infrastructure` and confirm the agent's
   hostname or display name is visible on the page.
7. **Delete agent** — DELETE `/api/agents/agent/{agentId}` and verify the agent is
   removed from `/api/state` `resources[]`.

## Success Criteria

- Agent report returns `{ success: true, agentId: "..." }`.
- Agent appears in `/api/state` `resources[]` with correct metadata.
- Agent is visible on the infrastructure page in the browser.
- After deletion, agent no longer appears in state.
