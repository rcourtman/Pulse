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
