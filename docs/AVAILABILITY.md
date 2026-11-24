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

1. Open **Settings → Monitoring → Availability checks** and add a target.
2. Choose its **Target type**, give it a name, choose its protocol, and enter
   the address and any required port or path. The Pulse server must be able to
   resolve and reach that address.
3. Review the polling interval, timeout and failure threshold. A missed check
   is evaluated under those settings, not a promise of an immediate alert.
4. Save the enabled target and inspect its observations. Use **This Pulse
   server** as the observation location for a local check.
5. Configure [alert destinations and verify delivery](ALERTING.md#choose-where-notifications-go).
   A saved check alone does not establish that a recipient will be notified.

Keep credentials and private target addresses out of public screenshots and
support reports. Use a service you own or are authorised to monitor.

The target type says what the check is on:

| Target type | Use it for |
| --- | --- |
| Website or service | A site, an API, a port |
| Machine or server | A computer without a Pulse agent |
| Router or firewall, Switch, Access point | Network gear |
| Other device | A printer, a UPS, a sensor |

A machine check does not add the computer to **Machines**; that takes a Pulse
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

## Check that DNS answers correctly

Use the **DNS** probe protocol (or the **DNS resolver** preset) to verify that a resolver such as Pi-hole, AdGuard Home, Technitium, or Unbound answers queries correctly.

1. Set **Address** to the DNS server IP or hostname (and **Port** if it listens on a port other than `53`).
2. Set **Query hostname** to a domain the resolver should answer (for example, `example.com` or an internal hostname such as `nas.home.arpa`).
3. Choose the **Record type** (`A`, `AAAA`, `CNAME`, `TXT`, `MX`, `NS`, `SRV`, or `PTR`) and **Transport** (`UDP` or `TCP`).
4. Optionally set **Expected answer** to verify that at least one returned record matches a specific IP, target host, or TXT substring.

Pulse records transport reachability separately from DNS answer validation. A timeout or refused socket marks the resolver unreachable, while a `SERVFAIL`, `NXDOMAIN`, empty answer set, or value mismatch records that the resolver answered but failed the DNS check.

Native DNS checks run locally on the Pulse server in Community, and run from remote host agents when **Observation locations** are configured with Pro. No Pulse agent is required on the DNS server itself.
