# Agent Registration Journey

## Goal

Validate the host agent registration flow end-to-end: an agent sends its first
report, Pulse registers the host, the host appears in the unified state API, and
the host is visible in the infrastructure UI.

## Steps

1. **Login** — Authenticate as admin via the Pulse UI or API.
2. **Register agent** — Send a POST to `/api/agents/host/report` with a
   synthetic host report payload containing hostname, OS info, CPU, memory,
   disks, and network interfaces.
3. **Verify API state** — GET `/api/state` and confirm the new host appears in
   the `hosts` array with correct hostname, platform, OS, and CPU count.
4. **Verify metrics** — Confirm the host's CPU usage and memory metrics are
   populated in the state response.
5. **Send follow-up report** — POST a second report with different CPU/memory
   values and verify `lastSeen` updates.
6. **Verify UI** — Navigate to `/infrastructure` and confirm the host's
   hostname or display name is visible on the page.
7. **Delete host** — DELETE `/api/agents/host/{hostId}` and verify the host is
   removed from `/api/state`.

## Success Criteria

- Agent report returns `{ success: true, hostId: "..." }`.
- Host appears in `/api/state` with correct metadata.
- Host is visible on the infrastructure page in the browser.
- After deletion, host no longer appears in state.
