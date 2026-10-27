import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const guide = readFileSync(path.join(root, 'docs/UNIFIED_AGENT.md'), 'utf8');

function uninstallArticle(): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(guide, 'UNIFIED_AGENT');
  return article;
}

describe('shipped Windows removal guidance', () => {
  it('keeps the served producer equal to the source guide', () => {
    expect(
      readFileSync(path.join(root, 'frontend-modern/public/docs/UNIFIED_AGENT.md'), 'utf8'),
    ).toBe(guide);
  });

  it('renders exact-ID confirmation and partial-service recovery without asking for a new token', () => {
    const article = uninstallArticle();
    const heading = article.querySelector('#remove-the-whole-agent')!;
    const parts = [];
    for (
      let item = heading.nextElementSibling;
      item && item.tagName !== 'H3';
      item = item.nextElementSibling
    ) {
      parts.push(item.textContent);
    }
    const text = parts.join(' ').replace(/\s+/g, ' ');
    for (const fact of [
      'exact saved agent ID',
      'saved credential and TLS settings',
      'does not select an agent by hostname',
      'retains the binary, saved identity, credentials and logs',
      'retained files do not mean monitoring is still running',
      'generate a new token or bypass TLS',
      'cannot confirm server-side revocation',
      'older served or prepared Windows script',
      'do not rerun uninstall just to test',
    ]) {
      expect(text).toContain(fact);
    }
    const commands = Array.from(article.querySelectorAll('pre code')).map((node) => node.textContent);
    expect(commands).toContain('& $installerFile -Uninstall $true\n');
  });

  it('keeps the existing safe-preparation and private-log links inside shipped documentation', () => {
    const article = uninstallArticle();
    for (const id of ['windows-powershell-run-as-administrator', 'collect-agent-logs-safely']) {
      expect(article.querySelector(`a[href="#${id}"]`)).not.toBeNull();
      expect(article.querySelector(`#${id}`)).not.toBeNull();
    }
  });
});
