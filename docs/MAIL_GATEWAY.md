# Proxmox Mail Gateway (PMG) monitoring

Pulse reads PMG mail statistics, queues, quarantine totals and cluster information.
It does not send mail through PMG, release quarantined messages or repair its
queues. Pulse's own alert email uses a separate SMTP destination in
**Alerts → Notifications**; see [email troubleshooting](TROUBLESHOOTING.md#emails-not-sending).

## Add a PMG connection

1. Open **Settings → Infrastructure → Add infrastructure**. Search for
   **Proxmox Mail Gateway**, or use **Show more sources** to reveal its card.
2. Give the connection a recognisable name and the PMG API address, normally
   `https://pmg.example.com:8006`. Use the address reachable **from the Pulse
   server**, not just from your browser. Keep HTTPS and certificate verification
   enabled; correct the certificate trust or name rather than bypassing them.
3. Choose **Username & Password**. Use a dedicated PMG service account, such
   as `pulse-monitor@pmg`, with the minimum read permissions supported by your
   PMG version for the data you need. Enter its password only in the private
   settings form; do not put it in a command, URL or issue report.
4. Review the connection and save it. Check ordinary collection afterwards in
   **Proxmox → Mail Gateway** (`/proxmox/mail`). A successful connection test
   does not establish that queues, quarantine and every cluster node were read.

The current PMG credential form uses password authentication, not the PVE/PBS
API-token setup. Do not follow a PVE/PBS token or privilege-separation recipe
for PMG, or switch to `root@pam` merely to make a read succeed. An older setup
hint that refers to PMG API-token fields does not change this form's contract.
Use the PMG version's own account-management tools for the service account;
do not grant mail-management or administrator privileges just to fill a panel.

### Discovery is optional

If you already know the endpoint, add it directly; a network scan is not needed.
Discovery in **Settings → Infrastructure** finds candidate platform APIs on
configured networks for review. It does not authenticate or add a PMG connection
by itself, and a missing candidate does not prove the gateway is unavailable.
Only scan networks you administer and intend to scan. Do not broaden a scan or
repeat discovery merely to diagnose missing statistics.

### Collection scope is not a no-request guarantee

The connection form offers **Mail statistics & trends**, **Queue health
insights**, **Quarantine totals** and **Domain-level statistics**. Domain
statistics are off by default for a new connection. Do not use the individual
scope switches as a privacy or no-request boundary: mail, queue and quarantine
requests can still run with their options off. A saved checkbox or an absent
chart is not evidence that those requests stopped.

If PMG collection must stop, pause the **PMG connection** in
**Settings → Infrastructure**, rather than deleting it or only disabling
alerts. Pausing prevents subsequent ordinary polls; it does not cancel a
request already in flight. PMG readings and alerts are unavailable or stale
while paused, so use the gateway's own monitoring and arrange independent
coverage. Restore only the connection you deliberately paused when it is safe
to resume. Do not restart Pulse or alter PMG queues to verify an opt-out.

## Read the Mail Gateway view

There is no PMG filter on a top-level Infrastructure monitoring page. Use
**Proxmox → Mail Gateway** and clear its search and status filters before
treating an absent row as a missing connection. Expand the intended gateway's
row to inspect its detail; compare connection and node identity, not just a
similar name in another installation.

| Reading | Meaning and limits |
| --- | --- |
| **Mail**, **Spam**, **Virus** | Collected counts, not a mail-delivery receipt or a configurable spam-rate threshold. The drawer labels its mail-statistics timeframe; compare the same window in PMG. |
| **Queue**, **Deferred** | Reported backlog across collected nodes. The node details separate active, deferred, held and incoming messages, and show the oldest-message age where available. |
| **Quarantine** | Collected category totals, not a list of messages awaiting your review or permission to release them. |
| Gateway/node state | Connection and alert context, not proof that all datasets are complete or that intended recipients received mail. Inspect any attention reason and the affected reading separately. |
| Domain detail | Optional domain statistics; an absent section is not proof of zero traffic. The drawer shows only the top eight reported domains, not the entire domain inventory. |

**Healthy is not complete collection.** The gateway can remain online after a
mail-statistics, node-queue or quarantine read fails. Some missing fields can
display as zero or an empty section. A recent overall update time, a green
badge or one populated panel does not prove that every dataset was read. Compare
the affected value and observation window with PMG's own existing view before
acting on an apparent empty queue or quarantine.

## Alerts

Open **Alerts → Thresholds → Proxmox** and find **Mail Gateway Thresholds**.
The configurable PMG thresholds cover total, deferred and held queue counts,
oldest-message age in **minutes**, spam/virus quarantine counts, and quarantine
growth **percentage plus minimum message growth**. Check the saved values and
the intended gateway's overrides; a queue-count threshold is not a percentage.

PMG also has connectivity and mail anomaly checks, but the displayed spam/virus
counts do not expose a user-configurable spam-rate or delivery-failure
threshold. An alert in Pulse does not guarantee a notification was routed or
received. Follow [notification checks](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
for that separate path. Do not send test mail, fill a queue, release quarantine
or lower a production threshold merely to generate an alert.

## Troubleshooting without changing the gateway

Use the original error, time and affected dataset before changing credentials
or permissions:

| Observation | First check |
| --- | --- |
| Connection refused or timed out | The saved scheme, host and port, and the network path from the Pulse server. Browser access is a different path; refusal is not a password verdict. |
| Certificate validation error | The certificate name, expiry and trusted chain for that endpoint. Do not disable verification or change to HTTP. |
| 401/403 | The configured account and its read permissions for the specific failed request, privately. Do not grant administrator access as a diagnostic shortcut. |
| Gateway is present but statistics are missing | Clear view filters, check whether the connection is paused, and compare the particular dataset and time window with PMG. A successful test or version read does not establish dataset access. |
| Cluster nodes or node queues are missing | Compare the gateway's existing cluster inventory and each affected node's native queue view. One reachable node does not prove every node was collected. |

For a new connection, allow its configured polling interval rather than a fixed
one- or two-cycle promise. If a previously working reading stops advancing,
retain that observation and the original bounded error; do not repeatedly test,
scan, restart, re-add the connection or change the mail system to reproduce it.
Use the [bounded Pulse log readers](TROUBLESHOOTING.md#inspect-notification-logs)
for your actual deployment if needed. Do not enable Debug to obtain more data.

A useful report names the running Pulse/PMG versions, affected view and dataset,
original time/sequence, expected versus displayed value and matching native
time window, plus the relevant redacted error. Use consistent placeholders for
private gateway/node/domain identities. Keep passwords, tickets, tokens, mail
addresses, subjects, message contents, full API responses, configuration and
unredacted screenshots private. Unavailable evidence is unknown, not zero;
do not recreate mail traffic or repeat a failing action to obtain it.
