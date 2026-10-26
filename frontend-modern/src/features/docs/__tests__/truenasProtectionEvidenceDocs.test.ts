import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const readGuide = (name: string) =>
  readFileSync(path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`), 'utf8');
const renderGuide = createGuideFixture((name) =>
  name === 'TRUENAS' && process.env.PULSE_TRUENAS_PROTECTION_INPUT
    ? readFileSync(process.env.PULSE_TRUENAS_PROTECTION_INPUT, 'utf8')
    : readGuide(name),
);

function protectionSection() {
  const article = renderGuide('TRUENAS');
  const heading = article.querySelector('#read-truenas-protection-evidence');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && node.tagName !== 'H2';
    node = node.nextElementSibling
  ) {
    section.appendChild(node.cloneNode(true));
  }
  return section;
}

const prose = (element: HTMLElement) => element.textContent!.replace(/\s+/g, ' ').trim();

describe('TrueNAS protection evidence help', () => {
  it('distinguishes observed snapshots and task state from an independent completed copy', () => {
    const section = protectionSection();
    const text = prose(section);
    expect(section.querySelectorAll('tbody tr')).toHaveLength(2);
    expect(text).toContain('The Healthy filter selects successful events');
    expect(text).toContain('does not certify that every dataset is protected');
    expect(text).toContain('It is not an independent copy');
    expect(text).toContain('Running or Unknown is not a completed copy');
    expect(text).toContain(
      'remote mode alone does not establish a separate machine or failure domain',
    );
  });

  it('keeps the bounded collection and timestamp limitations visible beside the comparison', () => {
    const text = prose(protectionSection());
    const mapper = readFileSync(
      path.join(repoRoot, 'internal/recovery/mapper/truenas/mapper.go'),
      'utf8',
    );
    const snapshots = mapper.match(/maxTrueNASSnapshotsPerIngest\s*=\s*(\d+)/)![1];
    const replications = mapper.match(/maxTrueNASReplicationsPerIngest\s*=\s*(\d+)/)![1];
    expect(text).toContain(
      `${snapshots} snapshots and ${replications} replication tasks per collection`,
    );
    expect(text).toContain('retained events can outlive their source artifact');
    expect(text).toContain('Missing snapshot creation time is Unknown');
    expect(text).toContain('the row has no Completed timestamp');
    expect(text).toContain('A new collection cannot make an undated snapshot newer');
    expect(text).toContain('corrected on upgrade without deleting snapshot history');
    expect(text).toContain("confirm the native snapshot's creation time");
    expect(text).toContain('Running remains incomplete even if a timestamp is present');
    expect(text).toContain('Replication reporting success with no known last-run time is Unknown');
    expect(text).toContain('Failed, Warning and Running remain visible even without that time');
    expect(text).toContain('missing dates do not clear a reported problem');
    expect(text).toContain(
      'Failed and warning events remain in Attention, without an invented timestamp',
    );
  });

  it('uses current detail labels to compare the same source and destination without active work', () => {
    const section = protectionSection();
    const text = prose(section);
    expect(section.querySelectorAll('ol > li')).toHaveLength(3);
    for (const label of [
      'Host',
      'Dataset',
      'Full name',
      'Task ID',
      'Source datasets',
      'Target dataset',
      'Direction',
      'Last snapshot',
    ]) {
      expect([...section.querySelectorAll('strong')].map((node) => node.textContent)).toContain(
        label,
      );
    }
    expect(text).toContain('existing received-snapshot and task records');
    expect(text).toContain(
      'unavailable evidence, not a failed verification or a zero-sized backup',
    );
    expect(text).toContain('Snapshot Size is reported used space');
    expect(text).toContain('does not prove that snapshot or replication reads succeeded');
    expect(text).toContain('Do not run a new snapshot, replication, verification or restore');
    expect(text).toContain('delete records, change retention or widen permissions');
    expect(text).toContain(
      'Keep full dataset paths, appliance names, credentials and inventories private',
    );
    expect(section.querySelector('pre')).toBeNull();
  });

  it('ships the same guide and links to actual polling, recovery and guest-safety headings', () => {
    const article = renderGuide('TRUENAS');
    const section = protectionSection();
    expect(article.querySelector('#stale-truenas-data')).not.toBeNull();
    expect(section.querySelector('a[href="#stale-truenas-data"]')).not.toBeNull();
    for (const [doc, fragment] of [
      ['RECOVERY', 'what-the-evidence-means'],
      ['VM_DISK_MONITORING', 'backup-safety'],
    ]) {
      expect(section.querySelector(`a[href="/docs/${doc}#${fragment}"]`)).not.toBeNull();
      expect(renderGuide(doc).querySelector(`#${fragment}`)).not.toBeNull();
    }
    expect(readGuide('TRUENAS')).toBe(readFileSync(path.join(repoRoot, 'docs/TRUENAS.md'), 'utf8'));
    const text = prose(section);
    expect(text).toContain('an isolated destination, not the live dataset');
    expect(text).toContain(
      'Neither a successful event nor a corrected display proves application recovery or guest thaw',
    );
  });
});
