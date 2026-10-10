import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');

function render(markdown: string, name: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
}

const entries = [
  {
    name: 'README',
    source: 'README.md',
    start: '### Do you need an agent?',
    end: '\n> [!IMPORTANT]',
  },
  {
    name: 'INSTALL',
    source: 'docs/INSTALL.md',
    start: '### Step 2: Create Admin Account',
    end: '\n> **Note**: If you configure authentication',
  },
];

describe.each(entries)('$name API-only onboarding safety', ({ name, source, start, end }) => {
  const section = () => read(source).split(start)[1]?.split(end)[0] ?? '';
  const article = () => render(section(), name);
  const text = () => article().textContent?.replace(/\s+/g, ' ');

  it('distinguishes absence of a Pulse host agent from active QEMU guest-agent reads', () => {
    expect(section()).not.toBe('');
    expect(text()).toContain('API-only does not mean guest-agent-free');
    expect(text()).toContain('VM filesystem and memory requests through QEMU Guest Agent');
    expect(text()).toContain('channel used by freeze-enabled backups');
    expect(text()).toContain('Read-only permissions do not prove backup safety');
    expect(text()).not.toContain('nothing runs on the host');
    if (name === 'README') expect(text()).toContain('no Pulse host agent is required');
  });

  it('does not prescribe probes or agent changes during a backup and explains the outage', () => {
    expect(text()).toContain(
      'Do not add permissions, enable or restart an agent, or send manual guest-agent probes during a backup, freeze or thaw',
    );
    expect(text()).toContain('Stopping Pulse also stops its monitoring and alerts');
    expect(text()).toContain('OK backup task does not prove successful thaw');
    expect(article().querySelector('pre')).toBeNull();
  });

  it('resolves the existing full recovery precaution from the onboarding section', () => {
    const link = article().querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
    expect(link?.textContent).toBe('backup safety precaution');
    const target = render(read('docs/VM_DISK_MONITORING.md'), 'VM_DISK_MONITORING');
    expect(target.querySelector('#backup-safety')).not.toBeNull();
    const targetText = target.textContent?.replace(/\s+/g, ' ');
    expect(targetText).toContain('every filesystem covered by the backup');
    expect(targetText).toContain('workload liveness');
    expect(targetText).toContain('only if it was active beforehand');
  });
});

it('ships the tested installation guidance without a divergent mirror', () => {
  expect(read('frontend-modern/public/docs/INSTALL.md')).toBe(read('docs/INSTALL.md'));
});
