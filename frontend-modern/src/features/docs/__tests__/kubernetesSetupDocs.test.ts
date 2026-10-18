import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8');
const guide = readDoc('KUBERNETES');
// js-yaml is already in the committed lint/build graph. Parse the copied YAML,
// rather than accepting a permissions sentence elsewhere in the guide.
const yaml = createRequire(import.meta.url)('js-yaml') as {
  loadAll: (text: string) => unknown[];
};
type Rule = { apiGroups: string[]; resources: string[]; verbs: string[] };
type Role = { kind: string; rules: Rule[] };

function isMetricsRule(rule: Rule) {
  // Kubernetes API groups are complete array values, not URL substrings.
  return rule.apiGroups.some((group) => group === 'metrics.k8s.io');
}

function canReadMetrics(role: Role, resource: string) {
  return role.rules.some(
    (rule) =>
      isMetricsRule(rule) && rule.resources.includes(resource) && rule.verbs.includes('list'),
  );
}

function articleFor(name: string) {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc(name), name);
  return article;
}

describe('Kubernetes credential setup and diagnosis', () => {
  it.each([
    { apiGroups: ['metrics.k8s.io'] },
    { apiGroups: ['apps', 'metrics.k8s.io'] },
    { apiGroups: ['metrics.k8s.io', 'apps'] },
  ])('matches a complete metrics API group in $apiGroups', ({ apiGroups }) => {
    const rule = { apiGroups, resources: ['nodes', 'pods'], verbs: ['get', 'list'] };
    const role = { kind: 'ClusterRole', rules: [rule] };
    expect(isMetricsRule(rule)).toBe(true);
    for (const resource of ['nodes', 'pods']) {
      expect(canReadMetrics(role, resource), resource).toBe(true);
    }
  });

  it.each([
    { apiGroups: [] },
    { apiGroups: ['apps'] },
    { apiGroups: ['metrics.k8s.io.example'] },
    { apiGroups: ['other.metrics.k8s.io'] },
    { apiGroups: ['https://metrics.k8s.io'] },
    { apiGroups: ['metrics.k8s.io/v1beta1'] },
    { apiGroups: ['Metrics.k8s.io'] },
    { apiGroups: ['apps', 'metrics.k8s.io '] },
  ])('rejects non-matching metrics API groups in $apiGroups', ({ apiGroups }) => {
    const rule = { apiGroups, resources: ['nodes', 'pods'], verbs: ['get', 'list'] };
    const role = { kind: 'ClusterRole', rules: [rule] };
    expect(isMetricsRule(rule)).toBe(false);
    for (const resource of ['nodes', 'pods']) {
      expect(canReadMetrics(role, resource), resource).toBe(false);
    }
  });

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
    // The production renderer wraps tables in a horizontal scroll container.
    let container = heading.nextElementSibling;
    let table: Element | null = null;
    while (container && !table) {
      table = container.matches('table') ? container : container.querySelector('table');
      container = container.nextElementSibling;
    }
    expect(table).not.toBeNull();
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

  it('copies the metrics read role and binds it to the actual agent service account', () => {
    const article = articleFor('KUBERNETES');
    const block = [...article.querySelectorAll('pre code')].find((code) =>
      code.textContent?.includes('kind: ClusterRole\n'),
    )!.textContent!;
    const [account, role, binding] = yaml.loadAll(block) as [
      { kind: string; metadata: { name: string; namespace: string } },
      Role & { metadata: { name: string } },
      { subjects: { kind: string; name: string; namespace: string }[]; roleRef: { name: string } },
    ];
    expect(account.kind).toBe('ServiceAccount');
    expect(account.metadata).toEqual({ name: 'pulse-agent', namespace: 'pulse' });
    expect(role.kind).toBe('ClusterRole');
    expect(binding.subjects).toEqual([{ kind: 'ServiceAccount', ...account.metadata }]);
    expect(binding.roleRef.name).toBe(role.metadata.name);
    for (const resource of ['nodes', 'pods']) {
      expect(canReadMetrics(role, resource), resource).toBe(true);
    }
    expect(role.rules.find(isMetricsRule)).toEqual({
      apiGroups: ['metrics.k8s.io'],
      resources: ['nodes', 'pods'],
      verbs: ['get', 'list'],
    });
    for (const rule of role.rules) {
      expect(rule.verbs.every((verb) => ['get', 'list', 'watch'].includes(verb))).toBe(true);
      expect(
        rule.resources.some((resource) =>
          ['*', 'secrets', 'nodes/proxy', 'pods/exec'].includes(resource),
        ),
      ).toBe(false);
      expect(rule.apiGroups).not.toContain('*');
    }
    // Missing metrics group, wrong resource, and get-only all deny the actual
    // list requests; these controls would catch the predecessor's recipe.
    for (const changed of [
      block.replace('["metrics.k8s.io"]', '["other.example"]'),
      block.replace(
        'apiGroups: ["metrics.k8s.io"]\n    resources: ["nodes", "pods"]',
        'apiGroups: ["metrics.k8s.io"]\n    resources: ["nodes"]',
      ),
      block.replace('verbs: ["get", "list"]', 'verbs: ["get"]'),
    ]) {
      expect(changed).not.toBe(block);
      const changedRole = yaml.loadAll(changed)[1] as Role;
      expect(['nodes', 'pods'].every((resource) => canReadMetrics(changedRole, resource))).toBe(
        false,
      );
    }
  });

  it('keeps API usage, optional host identity and incomplete collection distinct', () => {
    const article = articleFor('KUBERNETES');
    const text = article.textContent!.replace(/\s+/g, ' ');
    for (const boundary of [
      'It does not require host mounts or privileged mode',
      'not a complete role for every Kubernetes tab',
      'These permissions do not install a metrics provider',
      'missing usage is not measured zero',
      'grants no Secret reads, nodes/proxy, writes or workload execution',
      'network and ephemeral-storage usage',
      'Its shared PULSE_AGENT_ID is not an identity for every host',
      'distinct, stable identity for each node',
      'preserve the current IDs, credentials and state',
      'Read-only host mounts do not remove the security exposure of a privileged pod',
    ])
      expect(text, boundary).toContain(boundary);
    const recipes = [...article.querySelectorAll('pre code')].map((code) => code.textContent!);
    expect(
      recipes.some((recipe) => recipe.includes('privileged: true') || recipe.includes('hostPath:')),
    ).toBe(false);
    for (const anchor of ['rbac', 'add-host-metrics-optional', '4-troubleshooting']) {
      expect(article.querySelector(`a[href="#${anchor}"]`)).not.toBeNull();
      expect(article.querySelector(`[id="${anchor}"]`)).not.toBeNull();
    }
    for (const anchor of ['configuration', 'duplicate-agents']) {
      expect(article.querySelector(`a[href="/docs/UNIFIED_AGENT#${anchor}"]`)).not.toBeNull();
      expect(articleFor('UNIFIED_AGENT').querySelector(`[id="${anchor}"]`)).not.toBeNull();
    }
  });

  it('ships the exact canonical guide with the scoped role and existing private-file recipes', () => {
    expect(readFileSync(path.join(frontendRoot, 'public/docs/KUBERNETES.md'), 'utf8')).toBe(guide);
  });
});
