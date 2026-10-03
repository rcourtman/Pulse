# Migrating Pulse

**Create Backup** in **Settings → System → Recovery** creates an encrypted
configuration export, not a full backup of the installation. Check its scope
before moving Pulse or retiring the old server. Do not use it alone when you
need to preserve history, agent enrolment state or settings outside that scope.

## Choose the backup you need

### Configuration transfer

Use export/import to move the included settings between installations. Pulse
decrypts the export with its passphrase and saves the imported credentials with
the destination's encryption key. Do not copy individual encrypted files into
a fresh installation with a different key.

The current export format includes:

| Included | Not included |
| :--- | :--- |
| Proxmox VE, PBS and PMG connections and credentials | TrueNAS, vSphere and Machine Availability connections |
| Alert configuration, overrides and intent policies | Active alerts, incidents, notification queue and delivery history |
| Email, webhook, Apprise and external watchdog configuration | Metrics history, audit history and report schedules |
| System settings | Local login credentials, environment overrides and sessions |
| API-token records | Server-side host/Docker/Kubernetes agent inventory and enrolment state |
| SSO providers configured in Pulse | Legacy environment-configured OIDC settings, external CA/certificate files and custom RBAC roles |
| Guest metadata and notes | Host/Docker metadata, agent profiles and assignments |
| — | AI settings, findings, usage and chat history |
| — | Licence files, existing Relay configuration, binaries, service definitions and update backups |

Contents depend on the version that created the export. Older formats can omit
settings included above. Imported API-token records retain their scopes and
expiry; they do not reveal the original tokens or transfer every agent's state.
SSO configuration **is included**, even though local login credentials are not.

### Full-state recovery

If you need the same installation's history and excluded state, retain a
consistent filesystem/volume backup as well. Stop Pulse before taking that
backup; monitoring and alert delivery pause while it is stopped. Preserve every
effective data path, the matching `.encryption.key`, audit data and signing key,
environment files, external certificates, binaries and service/container
configuration together. Custom paths and organisation directories matter.
Copying a live database file alone can omit its sidecar files.

Keep that backup private and test recovery on an isolated destination before
retiring the source. Do not mix encrypted files and keys from different
installations or restore over a running Pulse. See [configuration
paths](CONFIGURATION.md) and [audit storage and recovery](AUDIT_LOGGING.md#storage).
The [updater's installation snapshot](AUTO_UPDATE.md#what-an-update-snapshot-contains)
is a third, limited backup; it is not a substitute for either procedure above.

## Transfer the included configuration

1. On the old server, sign in and open **Settings → System → Recovery →
   Create Backup**. Use a strong, unique passphrase of at least 12 characters
   and store it separately from the downloaded file. With password login,
   choose **Use a custom passphrase** if you do not want to use that password.
   Keep the old instance and your separate full-state backup until validation
   is complete.
2. Install Pulse at the destination using the [installation guide](INSTALL.md).
   Configure destination-local administrator access before importing. Keep the
   destination isolated from agents and notification destinations during the
   trial so two servers do not send duplicate alerts or remote actions.
3. If the destination already has useful settings, export them first. Open
   **Settings → System → Recovery → Restore Configuration**, select the file
   and enter its passphrase. Import replaces the included configuration and
   API-token records; it does not merge them with the destination's records.
4. Check the result before changing DNS or agent addresses. A success message
   establishes neither full recovery nor fresh monitoring. If import reports a
   reload/apply failure, settings may already have been written: preserve both
   installations and inspect the error before repeating it.

Keep the archive, passphrase, configuration files and credentials out of issue
threads. Use the authenticated settings form rather than a command containing
a password, API token or session cookie.

## Cut over and verify

- **Login and permissions:** verify destination-local administrator access and
  any imported SSO provider, group mappings and callback address. Re-create
  custom roles and supply external CA/certificate files if needed. Environment
  overrides can take precedence over imported settings.
- **Connections and history:** check every expected platform, guest and backup
  against a fresh observation, not a cached row. Re-create excluded TrueNAS,
  vSphere and Machine Availability connections. Missing historical data after
  configuration-only transfer is not proof that collection has failed.
- **Agents:** preserve each agent's existing identity and private credential.
  With a changed server address, use the [agent retargeting
  guide](UNIFIED_AGENT.md#moving-pulse-to-a-new-address) on the monitored host.
  Do not create another token just for a server move or paste it into `--token`.
  Confirm fresh reports and any enrolment/authorisation result; imported token
  records alone do not prove that every agent is admitted. Re-create excluded
  profiles and assignments before enabling remote actions.
- **Alerts and notifications:** avoid running both instances against the same
  destinations. After cutover, check thresholds, schedules and one controlled
  notification test with its actual delivery result. Importing destination
  settings does not migrate pending notifications or incident history.
- **Licence and remaining settings:** re-activate Pro if required and verify
  any excluded settings you rely on. Existing paired phones need their retained
  configuration; configuration export alone is not a Mobile/Relay migration.
  Relay is no longer sold; do not enable it as a new migration route.

Retire the old installation only after these checks succeed. Keep the private,
consistent pre-move backup for recovery. If validation fails, stop the trial
destination before returning traffic to the intact source; check agent URLs
and avoid two active writers. There is no guaranteed five-minute recovery time.

## Troubleshooting

- **Wrong passphrase or rejected file:** use the original export file and its
  exact passphrase. Do not delete destination data or regenerate encryption
  keys to make an import succeed.
- **A setting is missing:** compare it with the export scope above and the
  exporting version. Use the old installation or full-state backup rather
  than assuming import contained it.
- **Agents are absent:** check their primary URL, TLS trust and fresh admission
  evidence using the linked guide. Do not uninstall, re-enrol or delete saved
  identities just because the dashboard is empty.
- **Need help:** retain the source/destination versions, deployment types,
  operation time and exact redacted error. Follow [safe
  reporting](TROUBLESHOOTING.md#getting-help); do not attach the recovery backup.
