import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const guide = readFileSync(path.join(repoRoot, 'docs/DOCKER.md'), 'utf8');

function renderGuide(): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(guide, 'DOCKER');
  return article;
}

function section(article: HTMLElement, id: string): string {
  const heading = article.querySelector(`#${id}`);
  expect(heading, `missing guide section ${id}`).not.toBeNull();
  const parts: string[] = [];
  for (let element = heading?.nextElementSibling; element; element = element.nextElementSibling) {
    if (/^H[1-3]$/.test(element.tagName)) break;
    parts.push(element.textContent ?? '');
  }
  return parts.join(' ').replace(/\s+/g, ' ');
}

describe('shipped monitored-container update guide', () => {
  it('distinguishes a cached comparison and its timestamp from fresh registry evidence', () => {
    const article = renderGuide();
    const text = section(article, 'reading-an-update-indicator');
    for (const phrase of [
      'not an installation result or an application health check',
      'a difference alone does not establish which image is newer',
      'Successful registry lookups are cached for up to six hours',
      'checked timestamp can advance',
      'not proof that every container was checked',
      "Reloading the page does not refresh the agent's registry cache",
      'unknown, not evidence that the image is up to date',
    ]) {
      expect(text).toContain(phrase);
    }
    expect(section(article, 'how-it-works')).toContain('that lookup can be cached');
    expect(section(article, 'how-it-works')).not.toContain('the latest digest');
  });

  it('keeps the documented cache and clock limits bound to their producers', () => {
    // Source consistency only: this does not execute a registry check or
    // reproduce the reporter's failed comparison after recreation.
    const registry = readFileSync(path.join(repoRoot, 'internal/dockeragent/registry.go'), 'utf8');
    const collector = readFileSync(path.join(repoRoot, 'internal/dockeragent/collect.go'), 'utf8');
    const adapter = readFileSync(
      path.join(repoRoot, 'internal/unifiedresources/adapters.go'),
      'utf8',
    );
    expect(registry).toMatch(/defaultCacheTTL\s*=\s*6\s*\*\s*time.Hour/);
    // A changed local image can now bypass old evidence through the bounded
    // refresh helper. Bind the clock assertion to the remaining cache-hit
    // branch, not the unconditional lookup that preceded that correction.
    expect(registry).toContain('cached, refresh := r.cachedForLocalImage(cacheKey, currentDigest)');
    const cacheHit = registry
      .split('if cached != nil && !refresh {')[1]
      ?.split('// Fetch latest digest from registry')[0];
    expect(cacheHit, 'the guarded cache-hit branch must remain inspectable').toBeDefined();
    expect(cacheHit).toMatch(/LatestDigest:\s+cached.latestDigest/);
    expect(cacheHit).toMatch(/CheckedAt:\s+time.Now\(\)/);
    expect(collector).toMatch(/LastChecked:\s+result.CheckedAt/);
    expect(adapter).toContain('container.UpdateStatus.LastChecked.After(updatesLastCheckedAt)');
  });

  it('does not compare local image IDs with registry or other-platform manifest digests', () => {
    const text = section(renderGuide(), 'reading-an-update-indicator');
    for (const phrase of [
      'The same tag text can refer to different images',
      'not a registry manifest. Do not compare it directly with latestDigest',
      'top-level index and its platform-specific manifest have different digests',
      'same repository, tag, platform and manifest layer',
    ]) {
      expect(text).toContain(phrase);
    }
  });

  it('leads a returning indicator to existing read-only state checks, not a second update', () => {
    const article = renderGuide();
    const text = section(article, 'reading-an-update-indicator');
    for (const phrase of [
      'do not repeat the update just to clear it',
      'existing action receipt',
      "application's ordinary health first",
      'they do not pull or recreate anything',
      'Do not restart or reinstall the agent, broaden a token, disable TLS verification',
      'delete images or volumes to test the indicator',
      'Keep full inspections, environments and registry credentials private',
    ]) {
      expect(text).toContain(phrase);
    }
    const link = Array.from(article.querySelectorAll('a')).find(
      (anchor) => anchor.textContent === 'read-only workload state checks',
    );
    expect(link?.getAttribute('href')).toBe('#action-was-sent-or-its-outcome-is-unknown');
    expect(link?.hasAttribute('data-doc-link')).toBe(false);
    expect(article.querySelector(link!.hash)?.textContent).toBe(
      'Action was sent or its outcome is unknown',
    );
  });

  it('separates a legacy monitoring credential from independent runner posture and operator intent', () => {
    const text = section(renderGuide(), 'command-agent-is-not-connected');
    for (const phrase of [
      "root identifies the collector's OS user, not its server-side permission",
      'commands legacy identifies an older, unmarked compatibility profile',
      'cannot authenticate the legacy command channel',
      'Do not add execution rights to that token as a repair',
      'Credential not issued and Runner not connected',
      'An absent runner alone does not rule out an authorised legacy command connection',
      'not proof of a broken collector',
      'your usual Docker or Podman administration tool',
      'Neither is a diagnostic prerequisite',
      'unknown is valid',
    ]) {
      expect(text).toContain(phrase);
    }
  });

  it('links deliberate command-profile choices to shipped security guidance rather than a forced conversion', () => {
    const article = renderGuide();
    const security = document.createElement('article');
    security.innerHTML = renderDocMarkdown(
      readFileSync(path.join(repoRoot, 'docs/AGENT_SECURITY.md'), 'utf8'),
      'AGENT_SECURITY',
    );
    for (const id of ['agent-privilege-model', 'least-privilege-agent-profile']) {
      expect(article.querySelector(`a[href="/docs/AGENT_SECURITY#${id}"]`)).not.toBeNull();
      expect(security.querySelector(`#${id}`)).not.toBeNull();
    }
  });

  it('separates a refused plan from a sent or unknown action before any shell check', () => {
    const article = renderGuide();
    const checks = section(article, 'check-a-failed-or-pending-workload-update');
    for (const phrase of [
      'existing error and action stage',
      'refused that plan before dispatch',
      '409 alone has other causes',
      'does not establish what an earlier action did',
      'outcome is uncertain, even beside a failed banner',
      'do not assume nothing changed',
      'A missing Privilege line is unknown',
      'not a connected session',
      'do not enable commands, broaden a token',
      'do not reinstall or retry the workload update as a connection test',
      'not a full Agent Doctor report, token hint, agent ID',
    ]) {
      expect(checks).toContain(phrase);
    }
    const stages = article.querySelector('#check-a-failed-or-pending-workload-update')!;
    const commands = article.querySelector('#command-agent-is-not-connected')!;
    const uncertain = article.querySelector('#action-was-sent-or-its-outcome-is-unknown')!;
    const firstShellCheck = uncertain.nextElementSibling!.nextElementSibling!.nextElementSibling!;
    expect(firstShellCheck.tagName).toBe('PRE');
    for (const [before, after] of [
      [stages, commands],
      [commands, uncertain],
      [uncertain, firstShellCheck],
    ]) {
      expect(before.compareDocumentPosition(after) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0);
    }
  });

  it('hands command-capable hosts to the actual shipped connection guide, not an install command', () => {
    const article = renderGuide();
    const link = Array.from(article.querySelectorAll('a')).find(
      (anchor) => anchor.textContent === 'command-channel connection and admission checks',
    );
    expect(link?.getAttribute('href')).toBe(
      '/docs/UNIFIED_AGENT#commands-enabled-but-remote-control-blocked',
    );
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    const agentGuide = document.createElement('article');
    agentGuide.innerHTML = renderDocMarkdown(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/UNIFIED_AGENT.md'), 'utf8'),
      'UNIFIED_AGENT',
    );
    expect(agentGuide.querySelector(link!.hash)?.textContent).toBe(
      'Commands enabled but remote control blocked',
    );
    expect(section(agentGuide, link!.hash.slice(1))).toContain('not a connected session');
    expect(guide).toBe(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/DOCKER.md'), 'utf8'),
    );
  });

  it('places independent data backup and deployment-manager limits before the action', () => {
    const article = renderGuide();
    const precautions = section(article, 'before-updating-a-workload');
    expect(precautions).toContain('Plan for downtime');
    expect(precautions).toContain('independent, consistent backup');
    expect(precautions).toContain('not a backup of mounted data');
    expect(precautions).toContain('same mounts and can change their contents');
    expect(precautions).toContain("does not edit your Compose file or manager's desired state");
    expect(precautions).toContain('Do not use an update as a diagnostic test');
    const precautionHeading = article.querySelector('#before-updating-a-workload')!;
    const actionHeading = article.querySelector('#updating-a-container')!;
    expect(precautionHeading.compareDocumentPosition(actionHeading)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
    expect(section(article, 'updating-a-container')).toContain('/docker/overview');
    expect(section(article, 'updating-a-container')).not.toContain('Workloads page');
  });

  it('does not promise durable backup, complete rollback or application readiness', () => {
    const limits = section(renderGuide(), 'safety-features');
    expect(limits).toContain('not a separate copy of its volumes or bind mounts');
    expect(limits).toContain('Removal, rename or restart can fail too');
    expect(limits).toContain('A failed banner does not prove');
    expect(limits).toContain('starting health status is not proof');
    expect(limits).toContain('check the current state before retrying');
    expect(limits).toContain('Do not delete the old container or volumes');
    expect(limits).toContain('old image alone does not undo a data migration');
    expect(limits).toContain('do not post full docker inspect output');
    expect(limits).not.toContain('the old one is restored');
    expect(limits).not.toContain('are all preserved');
  });

  it('matches the implemented early-removal window, not the orphan cleanup interval', () => {
    // Static source binding, not a claim that a real update or rollback ran.
    const update = readFileSync(
      path.join(repoRoot, 'internal/dockeragent/container_update.go'),
      'utf8',
    );
    const cleanup = readFileSync(path.join(repoRoot, 'internal/dockeragent/cleanup.go'), 'utf8');
    expect(update).toMatch(/waitForAsyncDelay\(5 \* time.Minute\)/);
    expect(cleanup).toMatch(/time.Since\(backupTime\) > 15\*time.Minute/);
    const limits = section(renderGuide(), 'safety-features');
    expect(limits).toContain('five minutes after a successful update');
    expect(limits).toContain('Do not rely on a 15-minute recovery window');
    expect(limits).not.toContain('Clean up the backup after 15 minutes');
  });

  it('keeps image-update suppression separate from command authority and links the real requirements', () => {
    const article = renderGuide();
    const disabled = section(article, 'disabling-update-features');
    expect(disabled).toContain('not a monitoring-only security boundary');
    expect(disabled).toContain('does not disable container lifecycle actions');
    expect(disabled).toContain('leave command execution disabled on the agent');
    expect(disabled).toContain('do not grant command execution permission to monitoring tokens');
    expect(disabled).not.toContain('Read-Only Mode');
    const link = Array.from(article.querySelectorAll('a')).find(
      (anchor) => anchor.textContent === 'container lifecycle requirements',
    );
    expect(link).toBeDefined();
    const targetId = decodeURIComponent(link!.hash.slice(1));
    const target = Array.from(article.querySelectorAll('h2')).find(
      (heading) => heading.id === targetId,
    );
    expect(target?.textContent).toBe('▶️ Container Lifecycle Actions');
  });
});
