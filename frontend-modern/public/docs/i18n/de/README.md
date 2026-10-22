# Pulse Einstieg auf Deutsch

Diese Seite ist der deutschsprachige Einstieg für Installation, erste Anmeldung
und Planwahl. Die vollständige, kanonische Dokumentation bleibt in der
englischen Dokumentation unter [docs/README.md](../../README.md). Befehle,
Image-Namen, Konfigurationsschlüssel, Aktivierungsschlüssel und UI-Pfade bleiben
absichtlich unverändert.

## Was ist Pulse?

Pulse ist ein self-hosted Monitoring-Arbeitsbereich für Proxmox, Docker,
Kubernetes, TrueNAS und verwandte Infrastruktur. Community deckt das
Kernmonitoring kostenlos ab. Pro ergänzt Ursachenanalyse, sichere
Remediation-Workflows, Operations-Werkzeuge, Governance-Funktionen und 90 Tage
Verlauf.

Relay wird nicht mehr verkauft. Bestehende Relay-Abonnenten erhalten
Pro-Funktionen zum bisherigen Preis, solange ihr Abonnement weiterläuft.
Bereits gekoppelte Telefone funktionieren bis zum **31. März 2027** weiter.
Relay verbindet Pulse mit der App, nicht mit der Weboberfläche. Für den
Remote-Zugriff auf die Weboberfläche nutze ein eigenes VPN oder einen Tunnel.
Für Warnmeldungen auf dem Telefon füge ein Ziel für ntfy, Gotify oder Pushover
hinzu und öffne Pulse im Browser deines Telefons.

## Bezahlte Relay-, Pro- und Legacy-Kunden

GitHub-Release-Assets und das öffentliche Docker-Image `rcourtman/pulse` sind
Community-Builds. Aktiviere deinen Lizenzschlüssel unter
**Settings → Plans & Billing → License recovery**, um Pro-Funktionen freizuschalten.

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
Installer-Datei und führe den gesamten Block in **Bash** auf deinem
Proxmox-Host aus. Er verwendet ein privates temporäres Verzeichnis, ignoriert
lokale curl-Einstellungen, erlaubt nur HTTPS-Weiterleitungen und verlangt
für jede Datei innerhalb von 60 Sekunden HTTP 200 vor der Signaturprüfung.
Für einen normalen Linux-Host verwende stattdessen die
[systemd-Anleitung](../../INSTALL.md#2-bare-metal--systemd):

```bash
(
set -eu
umask 077
export PULSE_VERSION=vX.Y.Z
pulse_installer_dir="$(mktemp -d)"
trap 'rm -rf "$pulse_installer_dir"' EXIT
cd "$pulse_installer_dir"
status=$(curl --disable --fail --silent --show-error --location \
  --proto '=https' --proto-redir '=https' --connect-timeout 10 --max-time 60 \
  --output install.sh --write-out '%{http_code}' \
  "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh")
[ "$status" = 200 ]
status=$(curl --disable --fail --silent --show-error --location \
  --proto '=https' --proto-redir '=https' --connect-timeout 10 --max-time 60 \
  --output install.sh.sshsig --write-out '%{http_code}' \
  "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig")
[ "$status" = 200 ]
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh
bash install.sh --version "${PULSE_VERSION}"
)
```

Füge den gesamten Block ein: Schlägt ein Download oder die Signaturprüfung fehl,
wird der Installer nicht ausgeführt. Führe den letzten Befehl nicht separat aus,
um einen Fehler zu umgehen.

Agent-Installationen und v5-zu-v6-Agent-Upgrades verwenden den Befehl, den
Pulse unter **Settings → Infrastructure → Install on a host** erzeugt. Dieser
Befehl wird von deinem Pulse-Server über `/install.sh` bereitgestellt.

## Schnellstart: Docker

Für Container-Umgebungen oder Tests kannst du Pulse direkt mit Docker starten:

```bash
docker run -d \
  --name pulse \
  -p 127.0.0.1:7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Dieses Beispiel für Neuinstallationen bindet den Host-Port nur an IPv4-Loopback.
Nutze vor der Eingabe von Token oder Passwort eine
[geschützte Setup-Verbindung](../../INSTALL.md#open-a-protected-setup-connection):
HTTPS mit vertrauenswürdigem Zertifikat, einen Browser auf dem Docker-Host oder
einen SSH-Tunnel von deinem Arbeitsplatz. Sende keine Zugangsdaten über
unverschlüsseltes HTTP an einen entfernten Host.

Für Docker Compose kann dieselbe Compose-Datei Community- und private Pro-Images
verwenden, wenn sie die `PULSE_IMAGE`-Variable nutzt:

```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    container_name: pulse
    restart: unless-stopped
    ports:
      - "127.0.0.1:7655:7655"
    volumes:
      - pulse_data:/data

volumes:
  pulse_data:
```

Auch Compose veröffentlicht hier nur auf Host-Loopback. Ersetze damit keine
bestehende Installation; behalte Daten-Volume, Image-Edition und Netzkonfiguration.
Ein Proxy in einem anderen Container benötigt ein gemeinsames privates Netzwerk
und den Pulse-Servicenamen, nicht sein eigenes Loopback. Entfernte Agenten und
SSO-Callbacks benötigen eine erreichbare HTTPS-Adresse mit geprüftem Zertifikat,
nicht die lokale Adresse des Setup-Tunnels.

Lass `PULSE_AUTH_USER` und `PULSE_AUTH_PASS` bei einer neuen Installation
ungesetzt und richte den Zugang beim ersten Anmelden mit dem Bootstrap-Token
ein. Verwende kein gemeinsames Beispielpasswort. Für automatisierte Einrichtung
nutze eine private, vom Deployment verwaltete Zugangsdatenquelle; siehe
[Configuration](../../CONFIGURATION.md#private-docker-authentication-file).
Passwörter gehören nicht in Befehlsargumente oder eine geteilte Compose-Datei.
Ein Docker-Administrator kann Zugangsdaten in der Container-Umgebung weiterhin
lesen, auch wenn Pulse sie für die Anmeldung hasht.

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

1. Öffne die [geschützte Setup-Verbindung](../../INSTALL.md#open-a-protected-setup-connection).
2. Füge das **Bootstrap Token** ein.
3. Schließe den **Quick Security Setup**-Assistenten ab.
4. Öffne **Settings → Infrastructure → Install on a host**, wenn du den
   unified agent auf Hosts installieren möchtest.

Für Proxmox solltest du mit API-only Monitoring starten, wenn Inventar,
Node-Status, VM-/Container-Status und Storage-Metriken ausreichen. Agenten sind
für inside-guest Docker/Podman-Sichtbarkeit, Host-SMART-/Temperaturdaten,
lokale ZFS-/Ceph-/mdadm-Details oder andere Telemetrie nötig, die lokalen
Host-Zugriff braucht.

### Proxmox: Sicherheit bei Backups

**API-only bedeutet nicht ohne Gastagent.** Abfragen von VM-Dateisystemen und
Arbeitsspeicher über den QEMU Guest Agent können denselben Kanal wie Backups mit
Dateisystem-Freeze nutzen. Nur Leserechte zu haben, beweist keine Backup-Sicherheit.
Während Backup, Freeze/Thaw oder einer Störung eines nicht reagierenden Gasts
keine zusätzlichen Rechte vergeben, Agenten installieren oder neu starten und
keine manuellen Gastagent-Abfragen senden.

Für betroffene Installationen gilt die englische
[Backup-Sicherheitsvorkehrung](../../VM_DISK_MONITORING.md#backup-safety) **vor
einem geplanten Backup mit Dateisystem-Freeze**. Sie berücksichtigt den
Pulse-Server und automatische Updater, nicht nur den Gastagenten. Sie ist keine
Wiederherstellungsanleitung für einen laufenden Vorfall. Das Stoppen von Pulse
bricht bereits gesendete Gastagent-Aufrufe nicht ab. **Während Pulse gestoppt
ist, stehen Monitoring und Warnmeldungen nicht zur Verfügung**; unabhängige
Überwachung vorsehen.

Ein Backup-Ergebnis **OK** beweist nicht, dass der Gast wieder aufgetaut ist.
Pulse pausiert lassen, bis das Backup beendet ist und unabhängige Prüfungen
**nach dessen Ende**, ohne Pulse oder QEMU Guest Agent, alle drei Punkte bestätigen:

1. Die Dateisysteme sind wieder aufgetaut.
2. Frische, erfolgreiche Schreibvorgänge der Workloads auf **jedem vom Backup erfassten Dateisystem**.
3. Die Workloads funktionieren.

Fehlt eine Prüfung oder schlägt sie fehl, die Pause beibehalten und das
Wiederherstellungsverfahren der Plattform verwenden. Eine Konsolenverbindung
oder ein erfolgreicher Lesezugriff genügt nicht. Erst nach allen Prüfungen nur
die Dienste und Timer wiederherstellen, die **vor der Pause aktiv waren**;
unbekannte vorherige Zustände sind keine Erlaubnis zum Starten. Keine Backup-Sperren
löschen oder Freeze deaktivieren, um ein Monitoringproblem zu testen.

## Wo geht es weiter?

- [Installation Guide](../../INSTALL.md) für vollständige Installationspfade.
- [Configuration](../../CONFIGURATION.md) für Authentifizierung,
  Benachrichtigungen und Systemeinstellungen.
- [Troubleshooting](../../TROUBLESHOOTING.md) für Logs und häufige Probleme.
- [Agent Security](../../AGENT_SECURITY.md) für Agent-Rechte,
  Proxmox-API-only-Optionen und Signaturprüfung.
- [Plans and entitlements](../../PULSE_PRO.md) für Community, Relay, Pro und
  Cloud.
