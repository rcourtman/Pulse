# See everything on one map

The map draws everything Pulse monitors on one screen: how traffic gets in,
what runs, and the storage, backups and hardware underneath. It draws itself
from what Pulse already collects. There is nothing to place by hand.

Open it from the **Map** tab on the left edge of the window. On a phone it is
the map button in the header.

## Read the map

| Part | What it shows |
| --- | --- |
| **Ways in** | Reverse proxies, VPNs, DNS, and your router, switches and access points |
| Network pills | Each network, with how many things are on it |
| Boxes such as **Media**, **Monitoring**, **Databases** | Guests and containers, grouped by what they are for |
| **Checks** | Availability checks on things that are not network gear |
| **Storage**, **Backups**, **Hardware** | Pools, backup targets and the machines everything runs on |

Each tile has a status dot: healthy, needs attention, critical, not reporting
or off.

- **Hover** a tile to read what it is, its address and what it is connected
  to. Tiles it is connected to are ringed.
- **Click** a tile to open it.
- **On a touch screen**, the first tap reads the tile and the second opens it.

The map always fits the window. When there is too much to name, it switches
to icons alone; **Names** and **Compact** choose for yourself.

## How Pulse recognises an app

Pulse knows more than 150 self-hosted apps, such as Jellyfin, Home Assistant
and Traefik, from a container's image or a guest's name. A recognised app
gets its logo and goes in the box for what it does.

Anything else shows its initials and goes in **Other**. A Proxmox tag such as
`media`, `monitoring`, `database`, `security` or `dev` puts a guest in that
box without renaming it.

## Add your router, switch or access point

Pulse cannot see network gear by itself. An
[availability check](AVAILABILITY.md) puts it on the map.

**Your router.** A gateway nothing monitors is drawn as a dashed tile that
says "not monitored". Click it. The check form opens with the address filled
in; save it, and the tile becomes your router with a live status.

**A switch or access point.** Add a check for its address in **Settings →
Monitoring → Availability checks**, and put what it is in the name:

| To draw | Use one of these words in the check's name |
| --- | --- |
| A router | `router`, `firewall`, `modem`, `OPNsense`, `pfSense`, `MikroTik`, `OpenWrt`, `FritzBox`, `UDM`, `USG` |
| A switch | `switch` |
| An access point | `AP`, `WiFi`, `WLAN`, `access point` |

"Core switch" and "AP living room" both work. The word must stand on its own:
"apt-cache" is not an access point. A check named for nothing in the table
stays in **Checks**, unless it probes a machine's default gateway, which
makes it a router whatever it is called. To change how a check is drawn,
rename it.

## Connections

The map draws no lines between tiles. A connection is written under the tile
and shown on hover:

- a container that uses another container's network, such as a download
  client behind a VPN container, reads "via" that container;
- a container routed by Traefik shows the hostname it answers on;
- a machine's default gateway and DNS server are named on the tile that
  answers at that address.

Gateways, DNS servers and shared container networks come from the Pulse
agent, so keep agents on the same release as the server.

## Share a picture

**Save image** downloads the map as a picture. Turn on **Hide names** first
to replace every name, address and network name with a stand-in such as
"Proxmox 2" or "Container 14". Recognised apps keep the app's own name, which
says what you run and nothing about where.

The picture is made in your browser. It is not sent to Pulse.

## What the map does not show

- The Internet tile is drawn, not monitored.
- Firewall rules, and which app talks to which.
- Devices with no check and no agent.
