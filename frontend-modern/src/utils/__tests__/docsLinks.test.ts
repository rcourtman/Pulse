import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  API_TOKEN_SCOPES_DOC_URL,
  CONFIGURATION_DOC_URL,
  MIGRATION_GUIDE_DOC_URL,
  PRIVACY_DOC_URL,
  PROXY_AUTH_DOC_URL,
  README_DOC_URL,
  SECURITY_DOC_URL,
  TERMS_DOC_URL,
  TROUBLESHOOTING_DOC_URL,
  SHIPPED_DOCS_ROOT,
  getShippedDocUrl,
} from '@/utils/docsLinks';
import apiAccessPanelSource from '@/components/Settings/APIAccessPanel.tsx?raw';
import aiRuntimeControlsSectionSource from '@/components/Settings/AIRuntimeControlsSection.tsx?raw';
import apiTokenManagerModelSource from '@/components/Settings/apiTokenManagerModel.ts?raw';
import securityOverviewPanelSource from '@/components/Settings/SecurityOverviewPanel.tsx?raw';
import selfHostedCommercialRecoverySectionSource from '@/components/Settings/SelfHostedCommercialRecoverySection.tsx?raw';
import securityWarningSource from '@/components/SecurityWarning.tsx?raw';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..', '..', '..');
const repoRoot = path.resolve(frontendRoot, '..');
const runtimeDocsLinkScanTimeoutMs = 15_000;

function getRuntimeSourceFiles(dir: string): string[] {
  const entries = readdirSync(dir, { withFileTypes: true });
  const files: string[] = [];

  for (const entry of entries) {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === '__tests__') {
        continue;
      }
      files.push(...getRuntimeSourceFiles(entryPath));
      continue;
    }

    if (!entry.isFile()) {
      continue;
    }

    if (!/\.(ts|tsx)$/.test(entry.name)) {
      continue;
    }

    if (/(\.test|\.spec)\.(ts|tsx)$/.test(entry.name)) {
      continue;
    }

    files.push(entryPath);
  }

  return files;
}

describe('docsLinks', () => {
  it('keeps custom systemd server help on the signed non-root install path', () => {
    const installation = readFileSync(path.join(repoRoot, 'docs', 'INSTALL.md'), 'utf8');
    const custom = installation
      .split('<summary><strong>Manual or custom systemd services (advanced)</strong></summary>')[1]
      .split('</details>')[0];
    expect(custom).toContain('use the signed installer above');
    expect(custom).toContain('`User=pulse`, `Group=pulse`');
    expect(custom).toContain('`ProtectSystem=strict` and `ProtectHome=true`');
    expect(custom).toContain('An empty `User=` means');
    expect(custom).toContain('the installer preserves existing service units during updates');
    expect(custom).toContain('Do not overwrite a working unit, change only its service user');
    expect(custom).toContain('preserve the data directory and its encryption');
    expect(custom).toContain('separate host **agent** has different privilege requirements');
    const commands = [...custom.matchAll(/```bash\n([\s\S]*?)```/g)].map((match) => match[1]);
    expect(commands).toHaveLength(1);
    expect(commands[0]).toContain('systemctl show pulse.service');
    expect(commands[0]).not.toMatch(/--property=(Environment|ExecStart)|sudo|tee|install -m/);
    expect(installation).not.toContain('sudo tee /etc/systemd/system/pulse.service');
    expect(installation).not.toContain('ExecStart=/usr/local/bin/pulse');
  });

  it('keeps TrueNAS setup acceptance separate from a system-information probe', () => {
    const truenas = readFileSync(path.join(repoRoot, 'docs', 'TRUENAS.md'), 'utf8');
    const setup = truenas.split('## Quick Start')[1].split('## Creating a TrueNAS API Key')[0];
    expect(setup).toContain('it does not validate inventory or metric collection');
    expect(setup).toContain('an elapsed interval is not proof');
    expect(setup).toContain('[polling checks](#stale-truenas-data)');
    expect(setup).not.toContain('Data appears within one configured polling cycle');
    const noData = truenas
      .split('### No data appearing after adding connection')[1]
      .split('### Inventory works')[0];
    expect(noData).toContain('Pools,\n  datasets, disks and alerts can still fail');
    expect(noData).toContain('does not establish live CPU, memory');
    expect(truenas).toContain('Some older builds, including Pulse\n6.4.1 and 6.4.5');
    expect(truenas).toContain('use the inventory observation time rather than the test time');
    expect(truenas).toContain('do not copy a token or cookie');
    expect(truenas).toContain('Do not post the full connection response');
  });

  it('keeps restored Proxmox node network details in the candidate packet', () => {
    const releaseNotes = readFileSync(
      path.join(repoRoot, 'docs', 'releases', 'RELEASE_NOTES_v6.4.0-rc.1.md'),
      'utf8',
    );
    expect(releaseNotes).toContain(
      'Proxmox node details show the configured interface names and IPv4/IPv6',
    );
  });

  // These now resolve to the in-app viewer route rather than the raw asset.
  // The raw file is still served at the same path plus .md, which is what the
  // viewer fetches, so the source remains reachable.
  it('returns canonical shipped doc URLs', () => {
    expect(SHIPPED_DOCS_ROOT).toBe('/docs');
    expect(getShippedDocUrl('PRIVACY.md')).toBe('/docs/PRIVACY');
    expect(PRIVACY_DOC_URL).toBe('/docs/PRIVACY');
    expect(README_DOC_URL).toBe('/docs/README');
    expect(MIGRATION_GUIDE_DOC_URL).toBe('/docs/MIGRATION_UNIFIED_NAV');
    expect(TROUBLESHOOTING_DOC_URL).toBe('/docs/TROUBLESHOOTING');
    expect(CONFIGURATION_DOC_URL).toBe('/docs/CONFIGURATION');
    expect(PROXY_AUTH_DOC_URL).toBe('/docs/PROXY_AUTH');
    expect(SECURITY_DOC_URL).toBe('/docs/SECURITY');
    expect(TERMS_DOC_URL).toBe('/docs/TERMS');
    expect(API_TOKEN_SCOPES_DOC_URL).toBe('/docs/CONFIGURATION');
  });

  it('keeps shipped docs content synced with repo docs', () => {
    // Derived from what is actually shipped rather than a hand-maintained
    // list, so a doc copied into public/docs can never silently drift from
    // its repo source and a new one cannot be added without a source.
    const shippedDocsRoot = path.join(frontendRoot, 'public', 'docs');
    // Shipped from the repository root rather than docs/.
    const rootSourcedDocs = new Set([
      'SECURITY.md',
      'TERMS.md',
      'ARCHITECTURE.md',
      'CONTRIBUTING.md',
    ]);

    function collectShippedDocs(dir: string, prefix = ''): string[] {
      return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
        const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
        if (entry.isDirectory()) {
          return collectShippedDocs(path.join(dir, entry.name), relative);
        }
        return entry.name.endsWith('.md') ? [relative] : [];
      });
    }

    const docPairs = collectShippedDocs(shippedDocsRoot)
      .sort()
      .map((target) => ({
        source: rootSourcedDocs.has(target)
          ? path.join(repoRoot, target)
          : path.join(repoRoot, 'docs', ...target.split('/')),
        target,
      }));

    expect(docPairs.length).toBeGreaterThan(0);
    expect(docPairs.map(({ target }) => target)).toContain('UPGRADE_v6.md');

    for (const { source, target } of docPairs) {
      const rootDoc = readFileSync(source, 'utf8');
      const publicDoc = readFileSync(path.join(frontendRoot, 'public', 'docs', target), 'utf8');
      expect(publicDoc).toBe(rootDoc);
      expect(publicDoc).not.toContain('https://github.com/rcourtman/Pulse/blob/main/');
    }
  });

  it('keeps reverse-proxy help explicit about forwarded HTTPS and peer trust', () => {
    const proxy = readFileSync(path.join(repoRoot, 'docs', 'REVERSE_PROXY.md'), 'utf8');
    const configuration = readFileSync(path.join(repoRoot, 'docs', 'CONFIGURATION.md'), 'utf8');
    const setup = proxy
      .split('## Before configuring the proxy')[1]
      ?.split('## ⚡ Quick Configs')[0];
    expect(setup).toBeDefined();
    expect(setup).toContain('configured immediate peer');
    expect(setup).toContain('PULSE_TRUSTED_PROXY_CIDRS=127.0.0.1/32');
    expect(setup).toContain('::1/128');
    expect(setup).toContain('Wildcard ranges `0.0.0.0/0` and `::/0` are rejected');
    expect(setup).toContain('not an authentication bypass');
    expect(setup).toContain('[proxy authentication guide](PROXY_AUTH.md)');
    expect(setup).toContain('including its Header Trust Boundary');
    const nginx = proxy.split('### Nginx')[1].split('### Caddy')[0];
    expect(nginx).toContain('HTTPS `server` block');
    expect(nginx).toContain('proxy_set_header X-Forwarded-Proto $scheme;');
    expect(nginx).toContain('proxy_set_header X-Forwarded-For $remote_addr;');
    expect(nginx).toContain('proxy_set_header X-Forwarded-Host $host;');
    expect(nginx).toContain('proxy_set_header Forwarded "";');
    expect(proxy).not.toContain('$proxy_add_x_forwarded_for');
    const troubleshooting = proxy.split('### "HTTPS: HTTP only"')[1].split('### Other Issues')[0];
    expect(troubleshooting).toContain('Adding the header alone is not sufficient');
    expect(troubleshooting).toContain('inside** its\n`reverse_proxy` block');
    expect(troubleshooting).toContain('does not verify authentication, WebSockets or proxy');
    expect(configuration).toContain('REVERSE_PROXY.md#before-configuring-the-proxy');
    expect(configuration).toContain('forwarded client IP, scheme, host and port');
  });

  it('keeps configuration transfer separate from full-state recovery', () => {
    const migration = readFileSync(path.join(repoRoot, 'docs', 'MIGRATION.md'), 'utf8');
    const agent = readFileSync(path.join(repoRoot, 'docs', 'UNIFIED_AGENT.md'), 'utf8');
    const scope = migration
      .split('### Configuration transfer')[1]
      .split('### Full-state recovery')[0];
    expect(scope).toContain('Proxmox VE, PBS and PMG');
    expect(scope).toContain('| TrueNAS, vSphere and Machine Availability');
    expect(scope).toContain('SSO configuration **is included**');
    expect(scope).toContain(
      'Server-side host/Docker/Kubernetes agent inventory and enrolment state',
    );
    expect(migration).toContain('not a full backup of the installation');
    expect(migration).toContain('Stop Pulse before taking that');
    expect(migration).toContain('matching `.encryption.key`');
    expect(migration).toContain('reload/apply failure');
    expect(migration).toContain('it does not merge');
    expect(migration).toContain('UNIFIED_AGENT.md#moving-pulse-to-a-new-address');
    expect(migration).not.toContain('Update the `--token` flag');
    expect(migration).not.toContain('Restored in < 5 minutes');
    expect(migration).not.toContain('re-enable in Settings');
    expect(agent).toContain('restores API-token records, not the server-side');
    expect(agent).toContain('MIGRATION.md#configuration-transfer');
  });

  it('keeps lockout and incident reporting guidance safe for the affected client', () => {
    const troubleshooting = readFileSync(path.join(repoRoot, 'docs', 'TROUBLESHOOTING.md'), 'utf8');
    const recovery = troubleshooting.split('### Recovery Mode')[1].split('\n---')[0];
    expect(recovery).toContain('[I forgot my password](#i-forgot-my-password)');
    expect(recovery).toContain('browser-bound recovery');
    expect(recovery).toContain('A successful curl response does not unlock a separate');
    expect(recovery).toContain('remote and reverse-proxy');
    expect(recovery).not.toContain('generate_token');
    expect(recovery).not.toMatch(/```(?:bash|sh)[\s\S]*?X-Recovery-Token/);

    const help = troubleshooting.split('## 🆘 Getting Help')[1];
    expect(help).toContain('Do not repeat an update, outage or notification storm');
    expect(help).toContain('Export for GitHub (sanitized)');
    expect(help).toContain('Review before posting');
    expect(help).toContain('session cookies, webhook URLs');
    expect(help).toContain('If installation never started Pulse');
  });

  it('keeps port troubleshooting specific without exposing deployment credentials', () => {
    const troubleshooting = readFileSync(path.join(repoRoot, 'docs', 'TROUBLESHOOTING.md'), 'utf8');
    const ports = troubleshooting.split("### Port change didn't take effect")[1].split('\n### ')[0];

    expect(ports).toContain('CONFIGURATION.md#common-overrides-environment-variables');
    expect(ports).toContain('`PORT` alias applies only when `FRONTEND_PORT` is unset');
    expect(ports).toContain('`frontendPort` in `system.json` has no effect');
    expect(ports).toContain('separate agent listener, not the web UI port');
    expect(ports).toContain('`systemctl is-active pulse`');
    expect(ports).toContain('`sudo systemctl daemon-reload`');
    expect(ports).toContain('inside\n  the Pulse container, not on the Proxmox host');
    expect(ports).toContain('`docker compose up -d pulse`');
    expect(ports).toContain('same image and mounted data volume');
    expect(ports).toContain('Restarting an existing container does not apply a new port mapping');
    expect(ports).toContain('do not delete the volume');
    expect(ports).toContain('Do not post full service environments');
    expect(ports).not.toContain('--property=Environment');
    expect(ports).not.toMatch(
      /`(?:sudo )?(?:systemctl (?:status|cat)|docker inspect|docker compose config)[^`]*`(?! output)/,
    );
  });

  it('keeps clone identity recovery separate from destructive OS or credential resets', () => {
    const troubleshooting = readFileSync(path.join(repoRoot, 'docs', 'TROUBLESHOOTING.md'), 'utf8');
    const agentGuide = readFileSync(path.join(repoRoot, 'docs', 'UNIFIED_AGENT.md'), 'utf8');
    const shortGuide = troubleshooting
      .split('#### Docker hosts appearing/disappearing')[1]
      .split('\n### ')[0];
    const cloneGuide = agentGuide.split('### Duplicate Agents')[1].split('\n### ')[0];

    expect(shortGuide).toContain('UNIFIED_AGENT.md#duplicate-agents');
    for (const section of [shortGuide, cloneGuide]) {
      expect(section).toContain('saved');
      expect(section).toContain('Agent Doctor');
      expect(section).toContain('original host unchanged');
      expect(section).not.toContain('systemd-machine-id-setup');
      expect(section).not.toMatch(/(?:sudo\s+)?rm\s+(?:-\S+\s+)*\/etc\/machine-id/);
    }
    expect(cloneGuide).toContain('argument wins over the environment variable');
    expect(cloneGuide).toContain('PULSE_AGENT_ID_FILE');
    expect(cloneGuide).toContain('Environment="PULSE_AGENT_ID=vm-clone-02"');
    expect(cloneGuide).toContain('fresh, distinct IDs');
    expect(cloneGuide).toContain('does not split or recover historical');
    expect(cloneGuide).toContain('Do not upload `connection.env`, token files');
    expect(cloneGuide).toContain('do not delete tokens, re-enrol');
  });

  it('ships a bounded maintenance example separately from incident snoozing', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const maintenance = apiReference
      .split('### Resource Maintenance and Operator State')[1]
      .split('### Fleet Connections')[0];
    const example = JSON.parse(/```json\s*([\s\S]*?)```/.exec(maintenance)![1]);
    expect(Date.parse(example.maintenanceEndAt)).toBeGreaterThan(
      Date.parse(example.maintenanceStartAt),
    );
    expect(example.maintenanceScope).toBe('resource_and_descendants');
    expect(maintenance).toContain('PUT replaces the whole record');
    expect(maintenance).toContain('Do not DELETE the record unless you also intend');
    expect(apiReference).toContain('](#resource-maintenance-and-operator-state)');
  });

  it('ships the Pulse Mobile retirement in the published API reference', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain('## 📱 Relay / Pulse Mobile (retiring 31 March 2027)');
    expect(apiReference).not.toContain('Mobile Remote Access');
  });

  it('ships credential-free Proxmox setup instructions and a separate token prompt', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shipped = readFileSync(path.join(frontendRoot, 'public', 'docs', 'API.md'), 'utf8');
    const setup = apiReference.split('### Setup Script URL')[1].split('### Auto-Register')[0];
    expect(shipped).toBe(apiReference);
    expect(setup).toContain('identical credential-free commands');
    expect(setup).toContain('`downloadURL` equals the tokenless `url`');
    expect(setup).toContain('paste the token only at its prompt');
    expect(setup).not.toContain('token-bearing `commandWithEnv`');
  });

  it('ships the per-alert snooze and resume API contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain('`POST /api/alerts/snooze`');
    expect(apiReference).toContain('`POST /api/alerts/unsnooze`');
    expect(apiReference).toContain(
      'pauses delivery and escalation for up to 30 days while monitoring continues',
    );
    expect(apiReference).toContain('resumes normal policy without resolving the incident');
  });

  it('ships the Patrol weekly digest API contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain('`GET /api/ai/patrol/digest`');
    expect(apiReference).toContain('"what Patrol did for you" rollup');
    expect(apiReference).toMatch(/nothing new is\s+persisted/);
    expect(apiReference).toContain('docs/PATROL_WEEKLY_DIGEST.md');
  });

  it('documents the Patrol weekly summary report schedule kind for providers', () => {
    const mspGuide = readFileSync(path.join(repoRoot, 'docs', 'MSP.md'), 'utf8');
    expect(mspGuide).toContain('`patrol_digest`');
    expect(mspGuide).toMatch(/Digest schedules are weekly and\s+email-only/);
  });

  it('ships the readable SSO identity and user deprovisioning contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const rbacGuide = readFileSync(path.join(repoRoot, 'docs', 'RBAC.md'), 'utf8');

    expect(apiReference).toContain('`DELETE /api/admin/users/{username}`');
    expect(apiReference).toContain('`displayName`, `email`, `providerType`, `providerId`');
    expect(rbacGuide).toMatch(/revokes its active\s+sessions/);
    expect(rbacGuide).toMatch(/Removal does not disable\s+the upstream IdP account/);
  });

  it('ships the Community SSO and Pro RBAC role boundary', () => {
    const rbacGuide = readFileSync(path.join(repoRoot, 'docs', 'RBAC.md'), 'utf8');
    const shippedRBACGuide = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'RBAC.md'),
      'utf8',
    );
    const compactGuide = rbacGuide.replace(/\s+/g, ' ');

    expect(shippedRBACGuide).toBe(rbacGuide);
    expect(compactGuide).toContain(
      'built-in roles can be automatically assigned based on group membership on every plan',
    );
    expect(compactGuide).toContain(
      'Creating custom roles and manually managing user assignments require Pro RBAC',
    );
    expect(compactGuide).toContain('SSO authentication is not an administrator grant');
    expect(compactGuide).toContain(
      'map at least one trusted IdP group to the built-in `admin` role',
    );
    expect(compactGuide).toContain(
      'never elevates that user merely because no local administrator is configured',
    );
  });

  it('ships the configuration transfer authorization contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    const exportContract = apiReference
      .split('### Export Configuration')[1]
      ?.split('### Import Configuration')[0]
      ?.replace(/\s+/g, ' ');
    const importContract = apiReference
      .split('### Import Configuration')[1]
      ?.split('---')[0]
      ?.replace(/\s+/g, ' ');
    expect(exportContract).toContain('tenant manager');
    expect(exportContract).toContain('settings:read');
    expect(importContract).toContain('tenant manager');
    expect(importContract).toContain('settings:write');
  });

  it('ships the retained notification recovery API contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain('`POST /api/notifications/terminal-failures/retry`');
    expect(apiReference).toContain('`POST /api/notifications/terminal-failures/dismiss`');
    expect(apiReference).toContain('Existing per-attempt delivery history is preserved');
    expect(apiReference).toContain('without deleting delivery history');
  });

  it('ships the mixed-retention notification delivery-log contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );
    const queueToolsSection = (reference: string) =>
      reference.split('### Queue and Dead-Letter Tools')[1]?.split('\n### ')[0];

    const canonicalQueueTools = queueToolsSection(apiReference);
    expect(queueToolsSection(shippedAPIReference)).toBe(canonicalQueueTools);
    const compactQueueTools = canonicalQueueTools?.replace(/\s+/g, ' ');
    expect(compactQueueTools).toContain('`GET /api/notifications/delivery-log?limit=200`');
    expect(compactQueueTools).toContain('Completed attempts are retained for 7 days');
    expect(compactQueueTools).toContain('dead-letter attempts are retained for 30 days');
    expect(compactQueueTools).toContain('`limit` defaults to 50 and is capped at 200');
  });

  it('ships the bulk active-alert delivery diagnosis contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain(
      '`GET /api/alerts/delivery-diagnosis?alertIdentifier=<alert-id>`',
    );
    expect(apiReference).toContain(
      'omit `alertIdentifier` to get the diagnosis array for every active alert',
    );
    expect(apiReference).toContain('`GET /api/alerts/events?alertIdentifier=<alert-id>');
    expect(apiReference).toContain('append-only alert event log');
  });

  it('ships the credential-safe external watchdog API contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain('`GET /api/alerts/deadman/config`');
    expect(apiReference).toContain('`PUT /api/alerts/deadman/config`');
    expect(apiReference).toContain('`GET /api/alerts/deadman/status`');
    expect(apiReference).toContain('`pingUrl` is `***REDACTED***` when present');
    expect(apiReference).toContain('never includes the URL or endpoint fingerprint');
  });

  it('ships the truthful Patrol objective API contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedAPIReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );

    expect(shippedAPIReference).toBe(apiReference);
    expect(apiReference).toContain('`POST /api/ai/patrol/objectives`');
    expect(apiReference).toContain('`PATCH /api/ai/patrol/objectives/{id}`');
    expect(apiReference).toContain('Clients cannot submit either');
    expect(apiReference).toContain('validated, installed read-only observer with a live');
    expect(apiReference).toContain('`409 patrol_objective_revision_conflict`');
  });

  it('routes runtime docs links through shipped local docs instead of GitHub main', () => {
    expect(apiAccessPanelSource).toContain('API_TOKEN_SCOPES_DOC_URL');
    expect(apiAccessPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
    expect(apiTokenManagerModelSource).toContain("from '@/utils/docsLinks'");
    expect(apiTokenManagerModelSource).toContain('SHIPPED_API_TOKEN_SCOPES_DOC_URL');
    expect(apiTokenManagerModelSource).toContain('export const API_TOKEN_SCOPES_DOC_URL =');
    expect(apiTokenManagerModelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
    expect(securityOverviewPanelSource).toContain('PROXY_AUTH_DOC_URL');
    expect(securityOverviewPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
    expect(securityWarningSource).toContain('SECURITY_DOC_URL');
    expect(securityWarningSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/',
    );
    expect(aiRuntimeControlsSectionSource).toContain('TERMS_DOC_URL');
    expect(aiRuntimeControlsSectionSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/TERMS.md',
    );
    expect(selfHostedCommercialRecoverySectionSource).toContain('TERMS_DOC_URL');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/TERMS.md',
    );
  });

  it(
    'keeps non-test frontend runtime files free of GitHub main doc links',
    () => {
      const runtimeFiles = getRuntimeSourceFiles(path.join(frontendRoot, 'src'));

      for (const filePath of runtimeFiles) {
        const source = readFileSync(filePath, 'utf8');
        expect(
          source,
          `${path.relative(frontendRoot, filePath)} should use shipped/local docs owners`,
        ).not.toContain('https://github.com/rcourtman/Pulse/blob/main/');
      }
    },
    runtimeDocsLinkScanTimeoutMs,
  );

  it('ships the Patrol cost preview and model guidance contract', () => {
    const apiReference = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    const shippedApiReference = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'API.md'),
      'utf8',
    );
    for (const reference of [apiReference, shippedApiReference]) {
      expect(reference).toContain('`GET /api/ai/patrol/cost-preview`');
      expect(reference).toContain('`GET /api/ai/patrol/model-guidance`');
      expect(reference).toContain('recommended schedule');
    }
  });
});
