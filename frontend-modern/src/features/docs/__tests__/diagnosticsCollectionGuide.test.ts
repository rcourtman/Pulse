import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { getSettingsNavItem } from '@/components/Settings/settingsNavCatalog';
import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';
import { createGuideFixture } from './guideFixture';

const read = (name: string) =>
  readFileSync(resolve(process.cwd(), '../docs', `${name}.md`), 'utf8');
const article = createGuideFixture(read);
const text = (element: Element) => element.textContent!.replace(/\s+/g, ' ').trim();

function collectionSection() {
  const guide = article('TROUBLESHOOTING');
  const heading = guide.querySelector('#collect-diagnostics-safely');
  expect(heading?.tagName).toBe('H3');
  const section = document.createElement('section');
  for (
    let next = heading!.nextElementSibling;
    next && next.tagName !== 'H3';
    next = next.nextElementSibling
  ) {
    section.append(next.cloneNode(true));
  }
  return section;
}

describe('safe diagnostic collection guidance', () => {
  it('matches the existing Support panel and labels without turning its path into a GitHub link', () => {
    const section = collectionSection();
    const content = text(section);
    expect(content).toContain(
      `Settings → Support → ${getSettingsNavItem('support-diagnostics')?.label}`,
    );
    expect(section.querySelector('code')?.textContent).toBe(settingsTabPath('support-diagnostics'));
    expect(section.querySelector('a[href^="/settings"]')).toBeNull();
    expect(content).toContain(
      `${DIAGNOSTICS_PANEL_COPY.runActionLabel} (shown as ${DIAGNOSTICS_PANEL_COPY.runShortLabel} on phones)`,
    );
  });

  it('distinguishes safe panel opening, absent exports and a live check before any download advice', () => {
    const paragraphs = [...collectionSection().querySelectorAll('p')].map(text);
    expect(paragraphs[0]).toContain('Opening this panel does not run diagnostics');
    expect(paragraphs[0]).toContain('the download buttons are absent');
    expect(paragraphs[0]).toContain('use the original errors or observations');
    expect(paragraphs[0]).toContain('Diagnostics are optional, not a condition of reporting');
    expect(paragraphs[1]).toContain('live Proxmox/PBS API and guest-agent requests');
    expect(paragraphs[1]).toContain(
      'Do not run it during a backup, freeze/thaw or an unresponsive-host incident',
    );
    expect(paragraphs[2]).toContain('reuse that result without running the checks again');
    expect(paragraphs[2]).toContain('not an upload');
  });

  it('keeps both diagnostic downloads distinct from the full System Logs bundle', () => {
    const section = collectionSection();
    const content = text(section);
    for (const label of [
      DIAGNOSTICS_PANEL_COPY.exportFullLabel,
      DIAGNOSTICS_PANEL_COPY.exportGithubLabel,
    ]) {
      expect(content).toContain(label);
    }
    expect(content).toContain('not a guarantee that every free-text error');
    expect(content).toContain('Open the file locally and review it before sharing');
    expect(content).toContain(`Settings → Support → ${getSettingsNavItem('support-logs')?.label}`);
    expect(content).toContain('not limited to the lines visible in the panel');
    const logsLink = section.querySelector('a[href="/docs/CONFIGURATION#log-levels"]');
    expect(logsLink?.hasAttribute('data-doc-link')).toBe(true);
    expect(article('CONFIGURATION').querySelector('#log-levels')).not.toBeNull();
  });

  it('ships the same current navigation and safety instructions at all three intake entry points', () => {
    const contributing = readFileSync(resolve(process.cwd(), '../CONTRIBUTING.md'), 'utf8');
    for (const [name, source] of [
      ['TROUBLESHOOTING', read('TROUBLESHOOTING')],
      ['TRUENAS', read('TRUENAS')],
      ['CONTRIBUTING', contributing],
    ]) {
      expect(readFileSync(resolve(process.cwd(), `public/docs/${name}.md`), 'utf8')).toBe(source);
      expect(source.replace(/\s+/g, ' ')).toContain('Settings → Support → Diagnostics & Health');
      expect(source).not.toContain('Settings → Diagnostics');
    }
    const gettingHelp = read('TROUBLESHOOTING').split('## 🆘 Getting Help')[1].replace(/\s+/g, ' ');
    expect(gettingHelp).toContain('use existing logs, errors or observations first');
    expect(gettingHelp).toContain('If a result is already displayed');
  });
});
