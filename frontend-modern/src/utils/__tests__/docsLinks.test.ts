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
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';

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
  it('keeps report collection precautions usable without unpublished documentation links', () => {
    for (const form of ['bug_report.yml', 'v6_rc_feedback.yml']) {
      const source = readFileSync(path.join(repoRoot, '.github', 'ISSUE_TEMPLATE', form), 'utf8');
      expect(source).toContain('do not run it during backups, freeze/thaw or an unresponsive-host incident');
      expect(source).toContain('downloads that result without running checks again');
      expect(source).toContain('Review files and screenshots locally before posting');
      expect(source).not.toMatch(/https:\/\/github\.com\/rcourtman\/Pulse\/blob\/[^\s)]+\/docs\//);
    }
  });

  const probeGuides = () =>
    [
      readFileSync(path.join(repoRoot, 'docs', 'CONFIGURATION.md'), 'utf8')
        .split('### External probes (Pro)')[1]
        .split('### ICMP probe privileges')[0],
      readFileSync(path.join(repoRoot, 'docs', 'UNIFIED_AGENT.md'), 'utf8')
        .split('## External Probes (Pro)')[1]
        .split('## Custom metrics')[0],
    ].map((section) => section.replace(/\s+/g, ' '));

  it('keeps external probes dependent on server-side alert delivery', () => {
    for (const guide of probeGuides()) {
      expect(guide).toContain('The agent does not send notifications directly.');
      expect(guide).toContain(
        'Pulse server must be running and able to reach the notification destination',
      );
      expect(guide).toContain('agent cannot deliver Pulse alerts in its place');
      expect(guide).toContain('not proof');
      expect(guide).not.toContain('cannot disappear silently');
    }
  });

  it('explains the existing missing-report and bounded-buffer limits', () => {
    for (const guide of probeGuides()) {
      expect(guide).toContain('five minutes or three check intervals');
      expect(guide).toContain('server receipt time');
      expect(guide).toContain('host-offline alert');
      expect(guide).toContain('policies');
      expect(guide).toMatch(/changes (?:its|the) observation location/);
    }
    const agentGuide = probeGuides()[1];
    expect(agentGuide).toContain('up to 200 observations');
    expect(agentGuide).toContain('Oldest pending observations are dropped');
    expect(agentGuide).toContain('agent restart loses the queue');
    expect(agentGuide).toContain('not a complete outage record');
    // These limits come from the current implementation, not a new policy.
    const monitor = readFileSync(
      path.join(repoRoot, 'internal', 'monitoring', 'availability_probe_agent.go'),
      'utf8',
    );
    expect(monitor).toContain('availabilityProbeStaleFloor = 5 * time.Minute');
    expect(monitor).toContain('target.EffectivePollIntervalSecs()) * 3 * time.Second');
    const agent = readFileSync(
      path.join(repoRoot, 'internal', 'hostagent', 'availability.go'),
      'utf8',
    );
    expect(agent).toContain('availabilityPendingCapacity = 200');
  });

  it('links independent outage coverage without a permanent Mobile guarantee', () => {
    for (const guide of probeGuides()) {
      expect(guide).toContain('TROUBLESHOOTING.md#no-alert-when-pulse-power-or-internet-goes-down');
      expect(guide).toContain('independently reachable notification destination');
      expect(guide).toContain('authorised test environment');
      expect(guide).toContain('Existing paired Pulse Mobile/Relay users');
      expect(guide).toContain('31 March 2027');
      expect(guide).toContain('Relay is no longer sold');
      expect(guide).toContain('not a permanent substitute');
      expect(guide).toContain('[Mobile retirement](RELAY.md)');
      expect(guide).not.toContain('push covers the dark-site case');
    }
  });

  it('matches the existing observation-location editor and compatibility fields', () => {
    for (const guide of probeGuides()) {
      expect(guide).toContain('Observation locations');
      expect(guide).toContain('This Pulse server');
      expect(guide).toContain('not a universal outage');
      expect(guide).not.toContain('Run from');
      expect(guide).not.toContain('an assigned check is not also run locally');
    }
    const api = readFileSync(path.join(repoRoot, 'docs', 'API.md'), 'utf8');
    expect(api).toContain('`observationLocationIds` - Observation locations');
    expect(api).toContain('`pulse:local` for this Pulse server');
    expect(api).toContain('`agent:<host-agent-id>` for a connected agent');
    expect(api).toContain('omit `observationLocationIds`');
    expect(api).toContain('clear `probeAgentId` to `""`');
    const editor = readFileSync(
      path.join(
        frontendRoot,
        'src',
        'components',
        'Settings',
        'ConnectionEditor',
        'CredentialSlots',
        'AvailabilityTargetSlot.tsx',
      ),
      'utf8',
    );
    expect(editor).toContain('Observation locations</legend>');
    expect(editor).toContain('observationLocationIds: [...form.observationLocationIds]');
    const handler = readFileSync(
      path.join(repoRoot, 'internal', 'api', 'availability_handlers.go'),
      'utf8',
    );
    expect(handler).toContain(
      'compatibility.ObservationLocationIDs == nil && compatibility.ProbeAgentID != nil',
    );
    expect(handler).toContain('agentID != "" && len(*compatibility.ObservationLocationIDs) == 1');
  });

  it('separates server removal from persistent-data erasure', () => {
    const installation = readFileSync(path.join(repoRoot, 'docs', 'INSTALL.md'), 'utf8');
    const removal = installation.split('## 🗑️ Uninstall')[1];
    expect(removal).toContain('stops monitoring and alert delivery');
    expect(removal).toContain('Keep persistent\ndata by default');
    expect(removal).toContain('every effective data path');
    expect(removal).toContain('[full-state backup](MIGRATION.md#full-state-recovery)');
    expect(removal).toContain('A configuration export\nalone is not a full backup');
    expect(removal).toContain('Let any in-progress Pulse update finish');
    expect(removal).toContain('without a\nkeep-data prompt');
    expect(removal).toContain('Do not remove\nthe service account');
    expect(removal).toContain('Do not delete `/bin/update` unless');
    expect(removal).toContain('Removing the server does not remove agents');

    // Pin the copyable recipes, not merely the surrounding warnings. The old
    // Docker/root commands erased the complete data store as part of removal.
    const commands = [...removal.matchAll(/```bash\n([\s\S]*?)```/g)].flatMap((match) =>
      match[1]
        .replace(/\\\n\s*/g, ' ')
        .trim()
        .split('\n')
        .map((line) => line.replace(/\s+/g, ' ').trim()),
    );
    expect(commands).toEqual([
      'docker stop pulse',
      'docker rm pulse',
      'docker compose stop pulse',
      'docker compose rm pulse',
      'kubectl scale deployment pulse --namespace pulse --replicas=0',
      'sudo systemctl disable --now pulse-update.timer',
      'sudo systemctl disable --now pulse.service',
    ]);
  });

  it('bounds container-removal advice to retained persistent mounts', () => {
    const installation = readFileSync(path.join(repoRoot, 'docs', 'INSTALL.md'), 'utf8');
    const docker = (
      installation.split('### Docker and Compose: retain the data mount')[1] ?? ''
    ).split('### Kubernetes:')[0];
    expect(docker).toContain('persistent named volume or bind');
    expect(docker).toContain("container's writable layer or temporary storage");
    expect(docker).toContain('`--rm` can also delete anonymous volumes');
    expect(docker).toContain('**existing** project');
    expect(docker).toContain('reattach the **same** data mount');
    expect(docker).toContain('prefixes volume names');
    expect(docker).toContain('Do not add `-v`/`--volumes`');
    expect(docker).not.toContain('docker rm -f');
    expect(docker).not.toContain('docker volume rm');
  });

  it('does not promise Helm retention where the current chart owns the claim', () => {
    const installation = readFileSync(path.join(repoRoot, 'docs', 'INSTALL.md'), 'utf8');
    const kubernetes = (
      installation.split('### Kubernetes: check claim ownership before uninstalling')[1] ?? ''
    ).split('### Systemd / Proxmox LXC:')[0];
    expect(kubernetes).toContain('Do not assume `helm uninstall pulse -n pulse` retains data');
    expect(kubernetes).toContain('without a keep policy');
    expect(kubernetes).toContain('reclaim policy can delete its backing data');
    expect(kubernetes).toContain('`persistence.existingClaim`');
    expect(kubernetes).toContain('`emptyDir` storage, lost when the pod is removed');
    expect(kubernetes.indexOf('After verifying persistent storage and its backup')).toBeLessThan(
      kubernetes.indexOf('```bash'),
    );
    expect(kubernetes).toContain('suspend any controller');
    const chart = readFileSync(
      path.join(repoRoot, 'deploy', 'helm', 'pulse', 'templates', 'pvc.yaml'),
      'utf8',
    );
    expect(chart).toContain('kind: PersistentVolumeClaim');
    expect(chart).toContain('(not .Values.persistence.existingClaim)');
    expect(chart).not.toContain('helm.sh/resource-policy');
    const values = readFileSync(
      path.join(repoRoot, 'deploy', 'helm', 'pulse', 'values.yaml'),
      'utf8',
    );
    expect(values.split('persistence:')[1].split('server:')[0]).toContain('annotations: {}');
  });

  it('keeps custom systemd server help on the signed non-root install path', () => {
    const installation = readFileSync(path.join(repoRoot, 'docs', 'INSTALL.md'), 'utf8');
    const custom = installation
      .split('<summary><strong>Manual or custom systemd services (advanced)</strong></summary>')[1]
      .split('</details>')[0];
    expect(custom).toContain('use the signed installer above');
    expect(custom).toContain('`User=pulse`, `Group=pulse`');
    expect(custom).toContain('`ProtectSystem=strict` and `ProtectHome=true`');
    expect(custom).toContain('`LoadState=loaded`');
    expect(custom).toContain('not that a root service is running');
    expect(custom).toContain('an empty `User=`');
    expect(custom).toContain('the installer preserves existing service units during updates');
    expect(custom).toContain('Do not overwrite a working unit, change only its service user');
    expect(custom).toContain('preserve the data directory and its encryption');
    expect(custom).toContain('separate host **agent** has different privilege requirements');
    const commands = [...custom.matchAll(/```bash\n([\s\S]*?)```/g)].map((match) => match[1]);
    expect(commands).toHaveLength(1);
    expect(commands[0]).toContain('systemctl show pulse.service');
    expect(commands[0]).toContain('--property=LoadState');
    expect(Math.max(...commands[0].split('\n').map((line) => line.length))).toBeLessThan(34);
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
    expect(help).toContain(DIAGNOSTICS_PANEL_COPY.exportGithubLabel);
    expect(help).toContain('#collect-diagnostics-safely');
    expect(help).toContain('Review before posting');
    expect(help).toContain('session cookies, webhook URLs');
    expect(help).toContain('If installation never started Pulse');
  });

  it('separates live diagnostics from private local downloads and manual sharing review', () => {
    const troubleshooting = readFileSync(path.join(repoRoot, 'docs', 'TROUBLESHOOTING.md'), 'utf8');
    const collection = troubleshooting
      .split('### Collect diagnostics safely')[1]
      .split('\n### ')[0]
      .replace(/\s+/g, ' ');
    expect(collection).toContain('live Proxmox/PBS API and guest-agent requests');
    expect(collection).toContain('Do not run it during a backup, freeze/thaw');
    expect(collection).toContain('unresponsive-host incident');
    expect(collection).toContain('does not prove that normal monitoring has recovered');
    expect(collection).toContain('without running the checks again');
    expect(collection).toContain('not an upload');
    expect(collection).toContain(DIAGNOSTICS_PANEL_COPY.exportFullLabel);
    expect(collection).toContain(DIAGNOSTICS_PANEL_COPY.exportGithubLabel);
    expect(collection).toContain('not a guarantee');
    expect(collection).toContain('review it before sharing');
    expect(collection).toContain('Do not paste a **Copy as cURL** command');
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
