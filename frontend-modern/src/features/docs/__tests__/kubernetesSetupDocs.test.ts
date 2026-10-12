import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8');
const guide = readDoc('KUBERNETES');

function articleFor(name: string) {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc(name), name);
  return article;
}

describe('Kubernetes credential setup and diagnosis', () => {
  it('keeps both copied credential recipes intact and distinct from deployment', () => {
    const article = articleFor('KUBERNETES');
    const recipes = [...article.querySelectorAll('pre code')].map((code) => code.textContent!);
    const preparation = recipes.find((text) => text.includes('vi "$token_file"'))!;
    const provisioning = recipes.find((text) => text.includes('--from-file=PULSE_TOKEN='))!;
    for (const recipe of [preparation, provisioning]) {
      expect(recipe).toContain('[ -L "$token_file" ]');
      expect(recipe).toContain('umask 077');
      expect(recipe).toContain('chmod 600 "$token_file"');
    }
    expect(preparation).toContain('set -eu');
    expect(provisioning).toContain('set -euo pipefail');
    expect(provisioning).toContain('[ ! -s "$token_file" ]');
    const text = article.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('Stop if preparation or the editor fails');
    expect(text).toContain('Existing token contents are preserved');
    expect(text).toContain('provisioning operation that can change cluster state');
    expect(text).toContain('not a diagnostic for a missing reading');
  });

  it('shows four failure paths without promoting pods or tokens for diagnosis', () => {
    const article = articleFor('KUBERNETES');
    const heading = article.querySelector('[id="4-troubleshooting"]')!;
    expect(heading).not.toBeNull();
    let table = heading.nextElementSibling;
    while (table && table.tagName !== 'TABLE') table = table.nextElementSibling;
    const rows = [...table!.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(4);
    expect(rows.map((row) => row.firstElementChild!.textContent)).toEqual([
      'Pulse rejects reporting with HTTP 401/403',
      'Kubernetes API reports Forbidden',
      'Inventory appears but CPU/memory usage is absent',
      'Connection refused or TLS validation fails',
    ]);
    const text = table!.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('does not grant Kubernetes API permissions');
    expect(text).toContain('Do not add cluster-admin');
    expect(text).toContain('missing usage is not zero');
    expect(text).toContain('retain certificate verification');
    expect(article.textContent).not.toContain('Ensure securityContext.privileged: true');
  });

  it('routes authentication and private evidence to actual destination headings', () => {
    const article = articleFor('KUBERNETES');
    for (const [name, anchor] of [
      ['UNIFIED_AGENT', 'agent-lookup-returns-http-401-or-403'],
      ['TROUBLESHOOTING', 'collect-diagnostics-safely'],
    ]) {
      expect(article.querySelector(`a[href="/docs/${name}#${anchor}"]`)).not.toBeNull();
      expect(articleFor(name).querySelector(`#${anchor}`)).not.toBeNull();
    }
    expect(article.textContent!.replace(/\s+/g, ' ')).toContain(
      'Keep tokens, Secret YAML, kubeconfigs, Helm values, pod environments and full logs private',
    );
  });

  it('ships the exact canonical guide without changing the optional host profile', () => {
    expect(readFileSync(path.join(frontendRoot, 'public/docs/KUBERNETES.md'), 'utf8')).toBe(guide);
    const hostProfile = guide.split('#### Add Host Metrics (Optional)')[1].split('#### RBAC')[0];
    expect(hostProfile).toContain('privileged: true');
    expect(hostProfile).toContain('readOnly: true');
    expect(hostProfile).toContain('PULSE_ENABLE_HOST');
  });
});
