# Pulse v6.4.0-rc.3 Release Notes

`v6.4.0-rc.3` is a release candidate for the next v6 minor release. It improves large-estate performance, notification visibility, agent recovery and subscription options, while carrying forward the latest stable fixes.

## What's improved

- **Faster large-estate navigation** — Storage and platform cold loads fell from roughly 15.6 seconds to 1.1 seconds in the measured large-estate case, with windowed tables keeping scrolling responsive.
- **Lighter live updates** — The `/api/state` response fell from 4.75 MB to 4.09 MB, while changed-item catch-up and faster patch merging reduce work after reconnects and open mobile Alerts reliably.
- **Visible notification delivery** — A recent-delivery log shows redacted outcomes, and test sends now say when delivery is paused instead of implying that a message was sent.
- **Translation-ready webhooks** — Generic alert payloads include a stable message key and explicit resource context, so external systems can rebuild notifications in another language without parsing Pulse's English message.
- **Safer agent operation** — A least-privilege install profile limits the agent account and sudo access, while Docker and Kubernetes recovery controls allow deliberate re-enrolment after removal.
- **Clearer plan choices** — MSP evaluation mode supports provider trials, and the Business tier is now available for teams that need the annual business plan.
- **Simpler saved filtering** — Saved views have been removed; bookmarkable filtered URLs remain available for returning to useful table filters.

## Fixes

- Metrics history now releases oversized backing memory after dense data ages out, preventing the memory growth first corrected in v6.3.2.
- Patrol retries provider setup on each scheduled run, reports the real blocked reason and recovers from an early-start timing race.
- Proxmox backup roll-ups now calculate total pages from the normalised page size, so recovery pagination remains accurate.
- The Proxmox storage drawer now stacks the TLS and command-execution checkboxes, keeping both controls usable in issue #1775.
- Storage rows mark retained values stale and show the last successful refresh after a source poll fails.
- Stopped Docker and Podman containers no longer repeat health alerts from a stale retained result.
- Docker, Kubernetes and host agents expose recovery controls when a removed agent needs permission to re-enrol.
- A disabled Proxmox backup collection scope remains disabled after configuration reloads.
- Fixed polling intervals, offline-alert policy and command-execution intent remain intact across monitoring and re-enrolment flows.
- Proxmox RAID controller member disks are visible again in storage details.
- API-only Proxmox nodes now retain node metrics history; disk I/O is shown only when a linked agent supplies it.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid and no manual data migration is required.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.
- No Pulse Mobile update is required for this candidate.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate; use the published checksum and detached signature when verifying Windows agent downloads.
