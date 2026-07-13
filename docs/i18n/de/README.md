# Pulse Einstieg auf Deutsch

Diese Seite ist der deutschsprachige Einstieg für Installation, erste Anmeldung
und Planwahl. Die vollständige, kanonische Dokumentation bleibt in der
englischen Dokumentation unter [docs/README.md](../../README.md). Befehle,
Image-Namen, Konfigurationsschlüssel, Aktivierungsschlüssel und UI-Pfade bleiben
absichtlich unverändert.

## Was ist Pulse?

Pulse ist ein self-hosted Monitoring-Arbeitsbereich für Proxmox, Docker,
Kubernetes, TrueNAS und verwandte Infrastruktur. Community deckt das
Kernmonitoring kostenlos ab. Relay ergänzt sicheren Remote-Zugriff auf die
Pulse-Weboberfläche, Pulse-Mobile-Handoff-Pairing, Push-Benachrichtigungen und
14 Tage Verlauf. Pro ergänzt Ursachenanalyse, sichere Remediation-Workflows,
Operations-Werkzeuge, Governance-Funktionen und 90 Tage Verlauf.

## Bezahlte Relay-, Pro- und Legacy-Kunden

GitHub-Release-Assets und das öffentliche Docker-Image `rcourtman/pulse` sind
Community-Builds. Aktiviere deinen Lizenzschlüssel unter
**Settings → Plans → Existing purchases**, um Pro-Funktionen freizuschalten.

Diese Community-Builds enthalten nicht die privaten Pulse-Pro-Runtime-Hooks
für Audit Log, Audit Webhooks, RBAC und governed remediation. Für diese
Runtime verwende <https://pulserelay.pro/download.html> mit einem
**v6 activation key**. Ein v6 activation key beginnt mit `ppk_live_`. Ein v5-
oder Legacy-Lizenzschlüssel ist kein `ppk_live_` activation key und
funktioniert auf dieser Download-Seite nicht.

## Schnellstart: Proxmox LXC

Wenn du Proxmox VE verwendest, ist der offizielle LXC-Installer der
Pulse-native Einstieg. Er installiert den Pulse-Server. Ersetze `vX.Y.Z` durch
den exakten Release-Tag, den du installieren möchtest, prüfe die signierte
Installer-Datei und führe den Installer auf deinem Proxmox-Host aus:

```bash
export PULSE_VERSION=vX.Y.Z
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig"
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh
bash install.sh --version "${PULSE_VERSION}"
rm -f install.sh install.sh.sshsig
```

Agent-Installationen und v5-zu-v6-Agent-Upgrades verwenden den Befehl, den
Pulse unter **Settings → Infrastructure → Install on a host** erzeugt. Dieser
Befehl wird von deinem Pulse-Server über `/install.sh` bereitgestellt.

## Schnellstart: Docker

Für Container-Umgebungen oder Tests kannst du Pulse direkt mit Docker starten:

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Öffne danach Pulse unter `http://<your-ip>:7655`.

Für Docker Compose kann dieselbe Compose-Datei Community- und private Pro-Images
verwenden, wenn sie die `PULSE_IMAGE`-Variable nutzt:

```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - PULSE_AUTH_USER=admin
      - PULSE_AUTH_PASS=secret123

volumes:
  pulse_data:
```

## Erste Anmeldung

Beim ersten Start musst du ein **Bootstrap Token** abrufen und damit den
Admin-Account erstellen.

| Plattform | Befehl |
|---|---|
| Docker | `docker exec pulse /app/pulse bootstrap-token` |
| Kubernetes | `kubectl exec -it <pod> -- /app/pulse bootstrap-token` |
| Systemd | `sudo pulse bootstrap-token` |

Füge den Token-String ein, den der Befehl ausgibt. Füge nicht direkt den Inhalt
der Datei `.bootstrap_token` ein. In v6 kann diese Datei einen verschlüsselten
JSON-Snapshot enthalten, nicht den verwendbaren Setup-Token.

1. Öffne `http://<your-ip>:7655`.
2. Füge das **Bootstrap Token** ein.
3. Schließe den **Quick Security Setup**-Assistenten ab.
4. Öffne **Settings → Infrastructure → Install on a host**, wenn du den
   unified agent auf Hosts installieren möchtest.

Für Proxmox solltest du mit API-only Monitoring starten, wenn Inventar,
Node-Status, VM-/Container-Status und Storage-Metriken ausreichen. Agenten sind
für inside-guest Docker/Podman-Sichtbarkeit, Host-SMART-/Temperaturdaten,
lokale ZFS-/Ceph-/mdadm-Details oder andere Telemetrie nötig, die lokalen
Host-Zugriff braucht.

## Wo geht es weiter?

- [Installation Guide](../../INSTALL.md) für vollständige Installationspfade.
- [Configuration](../../CONFIGURATION.md) für Authentifizierung,
  Benachrichtigungen und Systemeinstellungen.
- [Troubleshooting](../../TROUBLESHOOTING.md) für Logs und häufige Probleme.
- [Agent Security](../../AGENT_SECURITY.md) für Agent-Rechte,
  Proxmox-API-only-Optionen und Signaturprüfung.
- [Plans and entitlements](../../PULSE_PRO.md) für Community, Relay, Pro und
  Cloud.
