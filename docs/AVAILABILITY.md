# Check websites, services and devices

Use availability checks when you need to know whether a website, service port
or device responds, without installing an agent on the target. Local checks
run on the Pulse server and are included in Community. Pro adds checks from
connected agents at other observation locations.

An availability check answers a narrower question than full machine monitoring.
A responding port does not prove the application behind it is working, and a
successful HTTP response does not prove every business operation succeeds.

## Choose the check for your question

| Question | Check | Limit to remember |
| --- | --- | --- |
| Does this device answer ping? | ICMP | A device or firewall can block ping even while a service works. |
| Can this location connect to the service port? | TCP | A connection is not an application transaction. |
| Does this UDP service return the expected response? | UDP, response required | Configure the request and expected response for that service. |
| Is there evidence a UDP port is closed? | UDP, open or filtered | Silence is indeterminate, not proof of reachability. |
| Does this DNS server resolve names as expected? | DNS | Tests the configured resolver over UDP or TCP (default port 53) and checks RCODE and optional expected record answers. |
| Does this web endpoint respond with an acceptable status? | HTTP or HTTPS | Pick a useful health path rather than assuming a landing page proves application health. |
| Is this HTTPS certificate valid and approaching expiry? | HTTPS | Certificate checks are enabled by default, with a 30-day warning unless overridden or disabled. |

See [protocols and target settings](CONFIGURATION.md) for fields and UDP
policies, or [check DNS answers](#check-that-dns-answers-correctly).

## Add a local check

1. Open **Availability** in the main navigation and select **Add check**, or
   open **Settings → Monitoring → Availability checks** to manage saved targets.
2. Choose its **Target type**, give it a name, choose its protocol, and enter
   the address and any required port or path. The Pulse server must be able to
   resolve and reach that address.
3. Review the polling interval, timeout and failure threshold. A missed check
   is evaluated under those settings, not a promise of an immediate alert.
4. Save the enabled target and inspect its observations. Use **This Pulse
   server** as the observation location for a local check.
5. Configure [alert destinations and verify delivery](ALERTING.md#choose-where-notifications-go).
   A saved check alone does not establish that a recipient will be notified.

From a check's detail drawer on **Availability**, administrators can run
**Test now**, **Pause** or **Resume**, and **Edit**, and inspect its status
timeline and response-time history across **24h**, **7d**, **30d** and **1y**.
Pausing or deleting a saved check retires its pending local poll from the
scheduler queue so removed checks do not keep scheduling follow-up polls.
Deleting a check also clears its scheduler health, retry, failure, dead-letter
and staleness entries so removed checks do not leave stale errors behind.
Local probes cancelled during a server reload or shutdown are discarded rather
than recording false unreachable failures or history rows, and in-flight local
or remote results for a check that was paused, deleted, edited or reassigned
to remote-only locations are dropped instead of restoring outdated state.

Keep credentials and private target addresses out of public screenshots and
support reports. Use a service you own or are authorised to monitor.

The target type says what the check is on:

| Target type | Use it for |
| --- | --- |
| Website or service | A site, an API, a port |
| Machine or server | A computer without a Pulse agent |
| Router or firewall, Switch, Access point | Network gear |
| Other device | A printer, a UPS, a sensor |

A machine check does not add the computer to **Machines**. That takes a Pulse
agent.

The router, switch and access point types need a Pulse server that knows
them. Before downgrading to an older release, set those checks to **Other
device**: an older server cannot run a check of a type it does not know, and
reports it unreachable.

## Check from another network with Pro

In the check editor's **Observation locations**, select a connected Pulse agent
running at the location you care about, such as a cloud VM or another site.
You do not need an agent on the service being checked. Keep **This Pulse
server** selected for a local observation too, or deselect it for agent-only
checks.

The assigned agent receives signed configuration, executes the check and
reports its results to Pulse. Results stay separate by location: a service can
be reachable locally while the outside path fails. Choose the location to
match the path you actually want to test.

If a probe stops reporting, Pulse treats that location as indeterminate rather
than asserting that the target is down. Eligible stale probes can raise a
warning through the normal alert policies. The [external-probe reference](CONFIGURATION.md#external-probes-pro)
documents the missing-report window and how duplicate agent-offline warnings
are avoided.

Pausing a check clears its pre-pause observations and external-probe grace
tracking without raising stale-probe errors while paused. Resuming the check
starts a fresh first-report grace window instead of immediately reporting an
unavailable probe.

## Detect when Pulse cannot alert

Remote agents do not send notifications directly. Pulse must still be running,
receive the observations and reach the notification destination. Moving a
probe outside your site does not provide independent alert delivery if the
Pulse server or the site's outbound connection fails.

For that failure, configure **Alerts → Notifications → External watchdog**
using an independent Healthchecks-compatible service. Follow the
[external-watchdog recipe](TROUBLESHOOTING.md#no-alert-when-pulse-power-or-internet-goes-down)
to place it outside the relevant failure boundary and verify actual recipient
delivery in an authorised test environment. A healthy heartbeat indicates
monitoring-loop progress, not successful delivery of every alert or external
availability of every service.

The status badge in **External watchdog** separates live heartbeats from a quiet
sender or a stale browser status read:

| Status badge | What it means |
| --- | --- |
| **Heartbeat healthy** | Pulse sent an accepted success ping within the recommended 3-minute grace period. |
| **No recent heartbeat** | The browser read status from Pulse, but **Last success** has reached or passed the recommended grace period. A live status read does not mean a heartbeat was sent. |
| **Status read overdue** | The browser has not received a fresh watchdog status read for two 30-second poll intervals (60 seconds). Displayed figures are the last known snapshot until a new read succeeds. |
| **Status unavailable** | The latest watchdog status request failed. Use **Refresh** to retry. Any figures shown below remain the last known snapshot. |
| **Delivery failing** | Pulse could not deliver a heartbeat to the configured watchdog URL. Check **Consecutive failures** and any recorded error message. |
| **Monitoring stalled** | Pulse's monitoring loop stalled and sent a `/fail` signal to the watchdog. |
| **Starting** / **Not configured** / **Configuration invalid** / **Configuration unavailable** | Startup or watchdog configuration states before ordinary heartbeats can run. |

After a restart, **Last interruption** records any monitoring gap of two minutes
or longer and distinguishes a `clean restart` from an `unexpected stop`.

When monitoring progress recovers after a stall, **Monitoring stalled** clears
immediately and moves to **Starting** or **Delivery failing** if the recovery
heartbeat fails to deliver rather than staying stuck in **Monitoring stalled**.
Replacing, removing, or saving an invalid watchdog URL clears prior **Last
success** and **Last attempt** timestamps (and clears prior delivery alerts on
replacement) immediately, while re-saving the same configured URL preserves its
**Last success**. Concurrent shutdown or configuration replacement cancels
superseded watchdog cycles so an in-flight heartbeat cannot overwrite a clean
shutdown record or the new destination's status.

## Check that DNS answers correctly

Choose the **DNS resolution** protocol (or the **DNS resolver (Pi-hole / AdGuard / Unbound)** preset) to verify that a resolver such as Pi-hole, AdGuard Home, Technitium, or Unbound answers queries correctly.

1. Set **DNS server address** to the DNS server IP or hostname (and **Port** if it listens on a port other than `53`).
2. Set **Query hostname** to a domain the resolver should answer (for example, `example.com` or an internal hostname such as `nas.home.arpa`).
3. Choose the **Record type** (`A`, `AAAA`, `CNAME`, `TXT`, `MX`, `NS`, `SRV`, or `PTR`) and **Transport** (**UDP (default)** or **TCP**).
4. Optionally set **Expected answer (optional)** to verify that at least one returned record matches a specific IP, target host, or TXT substring.

Pulse checks that the resolver returns `NOERROR` with at least one matching record, and records transport reachability separately from DNS answer validation. A timeout or refused socket marks the resolver unreachable, while a `SERVFAIL`, `NXDOMAIN`, empty answer set, or value mismatch records that the resolver answered but failed the DNS check.

Native DNS checks run locally on the Pulse server in Community, and run from remote host agents when **Observation locations** are configured with Pro. No Pulse agent is required on the DNS server itself.

## Verify HTTPS certificates and credentials

For an HTTPS check, **Monitor TLS certificate validity** and **Expiry warning (days)** (default `30`) track certificate expiry on the target.

When an HTTPS check sends stored credentials, custom header values or a request body, Pulse does not send them to an untrusted certificate. The server certificate must be trusted by the checking host or match **Pinned certificate fingerprint (optional)** (SHA-256), unless you explicitly enable **Allow an unverified certificate**.

If a test is refused for an untrusted certificate, Pulse reports the **presented certificate SHA-256** without sending stored values. Verify that fingerprint through an independent trusted path before pinning it, rather than turning off certificate verification.
