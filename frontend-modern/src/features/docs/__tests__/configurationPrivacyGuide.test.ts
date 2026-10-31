import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const read = (file: string) => readFileSync(path.resolve(process.cwd(), '..', file), 'utf8');
const article = createGuideFixture((name) => read(`frontend-modern/public/docs/${name}.md`));

describe('configuration and history privacy guidance', () => {
  it('identifies stored private content rather than treating it as ordinary settings', () => {
    const rows = [...article('CONFIGURATION').querySelectorAll('tbody tr')];
    for (const [file, content] of [
      ['ai_findings.json', 'evidence and user notes'],
      ['ai_patrol_runs.json', 'tool inputs/results'],
      ['ai_chat_sessions.json', 'usernames and tool inputs/results'],
      ['host_metadata.json', 'Private host notes, URLs'],
      ['guest_metadata.json', 'Private guest notes, URLs'],
    ]) {
      const cells = rows
        .find((row) => row.querySelector('td')?.textContent === file)
        ?.querySelectorAll('td');
      expect(cells, file).toHaveLength(3);
      expect(cells![1].textContent, file).toContain(content);
      expect(cells![2].textContent, file).toContain('Sensitive');
      expect(cells![2].textContent, file).not.toContain('Standard');
    }
  });

  it('distinguishes storage protection from safe public evidence without deleting recovery data', () => {
    const rendered = article('CONFIGURATION');
    const heading = rendered.querySelector('#handle-configuration-and-history-privately');
    expect(heading).not.toBeNull();
    const section = document.createElement('section');
    for (
      let node = heading!.nextElementSibling;
      node && node.tagName !== 'H2';
      node = node.nextElementSibling
    ) {
      section.appendChild(node.cloneNode(true));
    }
    const copy = section.textContent!.replace(/\s+/g, ' ');
    for (const distinction of [
      'Standard does not mean safe to publish',
      'not a promise of anonymisation or encryption',
      'their backups and decrypted or exported copies private',
      'Encryption and file permissions do not redact',
      'locally reviewed and redacted excerpt',
      'do not attach the configuration directory or whole history files',
      'Do not delete history, reset keys or disable collection',
    ]) {
      expect(copy).toContain(distinction);
    }
    const help = section.querySelector('a[href="/docs/TROUBLESHOOTING#-getting-help"]');
    expect(help?.textContent).toBe('safe reporting guidance');
    expect(article('TROUBLESHOOTING').querySelector('#-getting-help')).not.toBeNull();
    expect(section.querySelector('pre')).toBeNull();
  });

  it('ships the same privacy guidance as the repository guide', () => {
    expect(read('frontend-modern/public/docs/CONFIGURATION.md')).toBe(
      read('docs/CONFIGURATION.md'),
    );
  });
});
