# See everything on one map

The map draws everything Pulse monitors on one screen: how traffic gets in,
what runs, and the storage, backups and hardware underneath. It draws itself
from what Pulse already collects. There is nothing to place by hand.

Open it with the map button at the top left of the header.

## Read the map

| Part | What it shows |
| --- | --- |
| **Ways in** | Reverse proxies, VPNs, DNS, and your router, switches and access points |
| Network pills | Each network, with how many things are on it and live throughput when active |
| Boxes such as **Media**, **Monitoring**, **Databases** | Guests and containers, grouped by what they are for |
| **Checks** | Availability checks on websites, services, machines without an agent and other devices |
| **Storage**, **Backups**, **Hardware** | Pools, backup targets and the machines everything runs on |

Each tile has a status dot: healthy, needs attention, critical, not reporting
or off. When there is more than one network, a coloured mark beside a tile
matches the dot on its network's pill.

- **Hover** a tile to read what it is, its address, Docker user-defined
  networks, what it is connected to, and collected metrics (CPU, memory, live
  network throughput, check protocol and HTTP status with latency, TLS
  certificate warnings, NIC link speed, link-down count and Proxmox firewall
  status). Tiles it is connected to are ringed and joined by curved connection
  arcs (with gentle animated flow when **Live traffic** is on).
- **Click** a tile to open it.
- **With a keyboard**, Tab reaches the map and the arrow keys move between
  tiles. Enter opens a tile or a **+N** overflow drawer, Escape closes a
  tooltip or drawer and returns focus, and Tab again leaves the map.
- **On a touch screen**, the first tap reads the tile and the second opens it.

The map fits the window. A long name is written out, on a wider tile or
on two lines, wherever that leaves the map nearly as large. Where it would
not, the name is cut short, and hovering the tile reads it whole. When there
is too much to name, the map switches to icons alone. **Names** and
**Compact** let you choose for yourself.

On a phone the map is drawn at full size in one column and the page
scrolls, so every tile is big enough to tap and every label to read. It
opens with icons alone. **Names** writes every name out.

## Search, filter, inspect and zoom

Large estates can hold hundreds of guests, containers and networks. The map
toolbar and drawers let you find any resource without leaving the map:

- **Search map** filters across every monitored resource, including items
  folded into a **+N** tile or switched-off summary. Matching items appear
  directly on the map with a count bar above it.
- **N need attention** beside the title is a filter button. Click it to show
  only critical and degraded resources, and click it again (or choose **Clear
  filters**) to return to the full estate.
- **Estate live traffic pill** appears beside the title whenever combined
  network throughput across the estate reaches at least 10 KB/s. Click it to
  open the **Live traffic** drawer and inspect the busiest talkers across
  every group, sorted by throughput by default (or switch to **Attention**).
- **Group and network drawers** open when you click a **+N** overflow tile,
  a box header, or any network pill (including the **+N more** networks
  pill). Each drawer sorts items needing attention first, lets you switch
  to sorting by **Traffic** when live throughput is present, lets you search
  and filter by status, opens any item directly, and lets you filter the whole
  map to one network.
- **Zoom and pan** controls (`-`, percentage reset, `+`, or Ctrl/Cmd with the
  scroll wheel) zoom into dense maps up to 400%. While zoomed in, drag the
  map to pan around and click **Fit** (or the zoom percentage) to reset.

## Live traffic and network details

Pulse surfaces real network telemetry it already collects across your estate:

- **Live traffic** in the toolbar is on by default. When enabled, active tiles
  and network pills carrying at least 10 KB/s get a subtle accent highlight
  and live throughput readout, connectors feeding active groups and networks
  show gentle rate-paced motion, and hover connection arcs carry a subtle
  animated flow overlay. Click **Live traffic** to turn highlights and motion
  off at any time.
- **Estate live traffic summary and Top Talkers** aggregate throughput across
  workloads and standalone hosts without double-counting Proxmox hosts
  alongside their guests. Click the speed pill beside the title to open the
  **Live traffic** drawer and jump straight to the busiest resources.
- **Network throughput** appears on active tiles (in **Names** mode, while
  preserving stated connection captions such as `via` a container or a Traefik
  or Caddy hostname), on active network pills, in hover tooltips
  (`Net ↓ ... ↑ ...`), and on rows and network summaries in the slide-over
  drawer. Inside group and network drawers, switch the sort toggle from
  **Attention** to **Traffic** to bring the busiest talkers to the top.
- **Docker user-defined networks** appear in container tooltips and drawer
  rows (`Docker net: ...`), omitting default Docker bridges such as `bridge`
  and `host`. Enabling **Hide names** strips user-defined Docker network names
  from both the page and saved images.
- **Check protocol, HTTP status, latency and TLS certificate warnings** appear
  on monitored router, switch and access point tiles in **Ways in**, in check
  tooltips (such as `HTTPS 200 · 14 ms`), and in drawer rows. Expiring,
  expired, invalid or untrusted certificates also surface as warning or danger
  badges in drawer rows.
- **NIC link speed and link-down warnings** appear in host tooltips, drawer
  rows, and network drawer summaries (for example `10G` link speed and a
  **Link down** warning badge when an online host has a down interface).
- **Proxmox NIC firewall flag** appears as **NIC firewall flag** in guest
  tooltips and a **NIC firewall** badge in drawer rows when a Proxmox guest
  interface has its firewall flag enabled (it does not claim active firewall
  rules).

Live traffic motion only appears where Pulse actually collects traffic. Idle
resources, unmonitored hops, and the Internet connector never display
fabricated motion. If your operating system requests reduced motion, connector
and connection-arc animations stop automatically while static traffic
highlights and arcs remain visible.

## How Pulse recognises an app

Pulse knows more than 150 self-hosted apps, such as Jellyfin, Home Assistant
and Traefik, from a container's image or a guest's name. A recognised app
gets its logo and goes in the box for what it does.

Anything else shows its initials and goes in **Other**. A Proxmox tag such as
`media`, `monitoring`, `database`, `security` or `dev` puts a guest in that
box without renaming it.

## Add your router, switch or access point

Pulse cannot see network gear by itself. An
[availability check](AVAILABILITY.md) puts it on the map, and the check's
**Target type** says what it is.

**Your router.** A gateway nothing monitors is drawn as a dashed tile that
says "not monitored". Click it. The check form opens as a router with the
address filled in. Save it, and you are back on the map with the tile as your
router and a live status.

**Anything else.** Add a check in **Settings → Monitoring → Availability
checks** and choose its type:

| Target type | Where the map draws it |
| --- | --- |
| Router or firewall, Switch, Access point | In **Ways in**, as that device |
| Website or service, Machine or server, Other device | In **Checks** |

The type decides, not the name. When a check's name says what it is ("Core
switch"), or its address is exactly a monitored machine's default gateway,
the form offers the matching type. Nothing changes until you choose it. To
move a check on the map, change its type.

Click a check's tile to open that check.

## Connections

The map draws no permanent lines between tiles. Connections are written under
tiles and drawn as curved arcs with ringed endpoints on hover or keyboard
focus (up to 16 arcs at once so dense hosts stay readable, while all linked
targets remain ringed):

- a container that uses another container's network, such as a download
  client behind a VPN container, reads "via" that container, and they are
  drawn side by side in the box most of them belong in,
- a container routed by Traefik or Caddy Docker Proxy (`caddy` labels) shows
  the hostname it answers on and links to its proxy container,
- an availability check linked to a monitored resource connects to that
  resource on hover and names the check and latency on the resource's card,
- a machine's default gateway (including an unmonitored dashed gateway tile)
  and DNS server name the machines that use them and arc to those machines on
  hover,
- a machine's hover counts what it runs by kind, such as "Media 5 ·
  Monitoring 4", and rings every one of them and its storage,
- a backup target names the hosts it is set up on, and each host names the
  targets it backs up to,
- a guest is joined to the pool its data is on, and to the pool a share or
  folder it mounts is on, such as a media server that mounts a NAS array.

Gateways, DNS servers and shared container networks come from the Pulse
agent, so keep agents on the same release as the server.

A mount has no name that says where it comes from, so Pulse matches it by
size: it is joined to a pool only when they are the same size and as full,
to within a twentieth of a percent, and only one pool fits. Two copies of
the same data on identical disks look the same to it. When guests mount the
same filesystem and no pool Pulse monitors holds it, each says which others
share it. A backup target found the same way on a monitored pool says it
is stored there.

## Share a picture

**Save image** downloads the map as a picture. Turn on **Hide names** first
to replace every name, address and network name with a stand-in such as
"Proxmox 2" or "Container 14". Recognised apps keep the app's own name, which
says what you run and nothing about where.

The picture has the detail the page has, rendered as a clean static image
without traffic animations. A phone opens with icons alone, so choose
**Names** first there for a picture with names in it.

The picture is made in your browser. It is not sent to Pulse.

## What the map does not show

- The Internet tile is drawn, not monitored, and never shows live traffic
  motion.
- Firewall rules, and which app talks to which (only whether the Proxmox
  guest NIC firewall flag is set).
- Devices with no check and no agent.
