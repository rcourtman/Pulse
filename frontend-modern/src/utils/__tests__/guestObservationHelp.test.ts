import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');
const api = read('docs/API.md');
const section =
  api.split('#### Guest readings: availability and age')[1]?.split('\nAvailability is')[0] ?? '';

function render(markdown: string, name: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
}

const rendered = () => render(section, 'API');
const text = () => rendered().textContent?.replace(/\s+/g, ' ');

describe('guest observation API help', () => {
  it('distinguishes the legacy sentinel from an omitted unified metric and an observed zero', () => {
    expect(read('frontend-modern/public/docs/API.md')).toBe(api);
    expect(text()).toContain('disk.usage: -1 means unavailable, not negative usage');
    expect(text()).toContain('omitted as metrics.disk');
    expect(text()).toContain('Do not replace an absent metric with zero');
    expect(text()).toContain('a genuinely observed zero is valid');
    expect(text()).toContain('Allocated virtual-disk capacity alone does not establish used space');
    // These are the production omission/presence rules, not invented client fields.
    const types = read('internal/unifiedresources/types.go');
    expect(types).toMatch(/Metrics\s+\*ResourceMetrics\s+`json:"metrics,omitempty"`/);
    expect(types).toMatch(/Disk\s+\*MetricValue\s+`json:"disk,omitempty"`/);
    const metrics = read('internal/unifiedresources/metrics.go');
    expect(metrics).toContain('if hasObservedDiskUsage(disk)');
    expect(metrics).toContain('disk.Usage >= 0 && disk.Usage <= 100');
  });

  it('keeps retained disk evidence and an ambiguous error separate from a service diagnosis', () => {
    expect(text()).toContain('proxmox.diskStatusReason');
    expect(text()).toContain('A prev- reason means retained disk evidence, not a fresh reading');
    expect(text()).toContain('general guest-agent HTTP 500 error');
    expect(text()).toContain("does not establish that the guest's service is absent or stopped");
    expect(text()).toContain('An absent reason alone does not prove freshness');
    const collector = read('internal/monitoring/guest_disk_stability.go');
    expect(collector).toContain('"prev-" + diskStatusReason');
    // The warning also covers older/retained payloads. Requiring the old
    // HTTP-500 classifier here would forbid Core's typed-error safety repair.
  });

  it('renders selected-source memory states without renewing old or missing observations', () => {
    const rows = [...rendered().querySelectorAll('tbody tr')].map((row) =>
      [...row.querySelectorAll('td')].map((cell) => cell.textContent?.trim()),
    );
    expect(rows.map((row) => row[0])).toEqual([
      'current',
      'last-known',
      'unavailable',
      'Missing or unrecognised',
    ]);
    expect(rows[0][1]).toContain('original observedAt');
    expect(rows[1][1]).toContain('do not record it as a fresh sample');
    expect(rows[2][1]).toContain(
      'do not treat retained numbers or zero-valued fields as a measurement',
    );
    expect(rows[3][1]).toContain('Do not infer current');
    expect(text()).toContain('metrics.memory.observation');
    expect(text()).toContain("selected metric's source and age");
    expect(text()).toContain("do not substitute the resource's lastSeen or updatedAt");
    expect(text()).toContain('usageUnavailable: true');
    expect(text()).toContain('An absent metrics.memory is not 0% usage');
    expect(text()).toContain('disk state is not memory provenance');
    const model = read('internal/models/models.go');
    expect(model).toContain('`json:"observation,omitzero"`');
    expect(model).toContain('`json:"observedAt,omitzero"`');
    expect(model).toContain('`json:"usageUnavailable,omitempty"`');
    expect(read('internal/unifiedresources/metrics.go')).toContain(
      'Observation: memory.Observation',
    );
  });

  it('makes the existing safety and missing-reading guidance reachable without live probe commands', () => {
    expect(text()).toContain('an OK backup does not prove fresh guest readings or successful thaw');
    expect(text()).toContain('Do not install or restart an agent or send live guest-agent probes');
    expect(text()).toContain(
      'fresh successful writes to every covered filesystem and workload liveness',
    );
    expect(text()).toContain('retained API values cannot establish it');
    expect(rendered().querySelector('pre')).toBeNull();
    const target = render(read('docs/VM_DISK_MONITORING.md'), 'VM_DISK_MONITORING');
    for (const [label, fragment] of [
      ['Backup safety', 'backup-safety'],
      ['missing-reading guidance', 'a-missing-reading-is-not-an-installation-diagnosis'],
    ]) {
      const link = rendered().querySelector(`a[href="/docs/VM_DISK_MONITORING#${fragment}"]`);
      expect(link?.textContent).toBe(label);
      expect(target.querySelector(`#${fragment}`)).not.toBeNull();
    }
  });
});
