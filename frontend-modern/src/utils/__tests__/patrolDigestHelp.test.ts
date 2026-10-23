import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (relative: string) => readFileSync(path.join(root, relative), 'utf8');
const markdown = read('docs/AI.md');
const article = document.createElement('article');
const rendered = document.createElement('article');
rendered.innerHTML = renderDocMarkdown(markdown, 'AI');
for (
  let element = rendered.querySelector('#weekly-summary-and-email');
  element;
  element = element.nextElementSibling
) {
  if (element.tagName === 'H2' && element.id !== 'weekly-summary-and-email') break;
  article.append(element.cloneNode(true));
}
const text = article.textContent!.replace(/\s+/g, ' ');

// Exercise the actual shipped Markdown, not a second copy of the advice.
describe('Patrol weekly summary help', () => {
  it('ships operator help instead of a private build or sales plan', () => {
    expect(read('frontend-modern/public/docs/AI.md')).toBe(markdown);
    expect(text).toContain('included in v6.5.0');
    expect(text).not.toMatch(/Status: building|past due|stops paying|FEATURE_REQUESTS/);
    expect(read('docs/PATROL_WEEKLY_DIGEST.md')).toContain('(AI.md#weekly-summary-and-email)');
    expect(article.querySelector('#read-the-summary')).not.toBeNull();
    expect(article.querySelector('#check-a-missing-email-or-empty-summary')).not.toBeNull();
  });

  it('makes enabled mail and reporting access prerequisites of the setup flow', () => {
    const steps = Array.from(article.querySelectorAll('ol li'), (item) =>
      item.textContent!.replace(/\s+/g, ' '),
    );
    expect(steps).toHaveLength(5);
    expect(text).toContain('advanced reporting entitlement');
    expect(steps[0]).toContain(
      'Listing recipients on a schedule does not configure a mail provider or enable email',
    );
    expect(steps[1]).toContain('Report type to Patrol weekly summary');
    expect(steps[2]).toContain('Timezone');
    expect(steps[2]).toContain('not a PDF/CSV attachment');
  });

  it('warns about recipient fallback and real Run now delivery', () => {
    expect(text).toContain('delivery can fall back to the sender address');
    expect(text).toContain('Run now sends a real email; it is not a preview');
    expect(text).toContain("Pulse's sender returned success");
  });

  it('does not turn run or action counters into whole-estate acceptance', () => {
    expect(text).toContain('largest number checked in one run, not a count of distinct resources');
    expect(text).toContain('older open findings are not included');
    expect(text).toContain('current mode, not a history of mode changes');
    expect(text).toContain('zero in this summary is not proof that no action took place');
    expect(text).toContain('Pending approvals can include actions created before the window');
  });

  it('separates missing inputs, partial history, unknown spend and inbox receipt', () => {
    expect(text).toContain('partial-window notice concerns run retention only');
    expect(text).toContain('displayed amount is incomplete, not a zero-cost result');
    expect(text).toContain('not that the message reached the recipient');
    expect(text).toContain('unavailable Patrol service, not proof');
    expect(text).toContain(
      'Manual resolutions, dismissals and investigation outcomes on purged findings',
    );
  });

  it('preserves API errors and routes private evidence to real help anchors', () => {
    expect(text).toContain('ai:execute scope');
    expect(text).toContain('invalid values return 400');
    expect(text).toContain('Authentication, scope or transport errors are not a zero result');
    expect(text).toContain('never extract a session cookie');
    for (const [slug, fragment] of [
      ['AI', 'cost-tracking'],
      ['API', '-authentication'],
      ['TROUBLESHOOTING', '-getting-help'],
    ]) {
      const link = article.querySelector(`a[href="/docs/${slug}#${fragment}"]`);
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      const target = document.createElement('article');
      target.innerHTML = renderDocMarkdown(read(`docs/${slug}.md`), slug);
      expect(target.querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
  });
});
