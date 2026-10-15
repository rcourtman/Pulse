import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shipped = path.join(root, 'frontend-modern/public/docs/AI.md');
const source = readFileSync(process.env.PULSE_COST_GUIDE_INPUT ?? shipped, 'utf8');
const prose = (element: Element) => (element.textContent ?? '').replace(/\s+/g, ' ').trim();

function guide(): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(stripLeadingTitle(source), 'AI');
  return article;
}

function costSection(): HTMLElement {
  const heading = guide().querySelector('#cost-tracking');
  expect(heading?.tagName).toBe('H2');
  const section = document.createElement('section');
  for (
    let next = heading!.nextElementSibling;
    next && next.tagName !== 'H2';
    next = next.nextElementSibling
  ) {
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

describe('shipped AI cost and budget guidance', () => {
  it('keeps the repository and consuming guide identical', () => {
    expect(readFileSync(shipped, 'utf8')).toBe(readFileSync(path.join(root, 'docs/AI.md'), 'utf8'));
  });

  it('separates the runtime rolling window from a chart range or invoice', () => {
    const section = costSection();
    const text = prose(section);
    expect(text).toContain('Provider Usage & Spend');
    expect(text).toContain('Select 30d');
    expect(text).toContain('preceding 30 days, not a calendar month');
    expect(text).toContain('changing the chart range does not change the runtime budget');
    expect(prose(section.querySelector('strong')!)).toBe(
      'not a provider invoice or a guaranteed spending cap',
    );
    expect(text).toContain('provider-side spending limit');
    expect(text).toContain('shorter-retention notice');
    expect(text).toContain('Prices as of date');
    expect(text).not.toContain('Set monthly budget limits');
  });

  it('gives different next steps for budget, unknown pricing and provider errors', () => {
    const rows = [...costSection().querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(4);
    expect(prose(rows[0])).toContain('Pulse has declined analysis');
    expect(prose(rows[0])).toContain('resetting a circuit breaker does not repair a budget refusal');
    expect(prose(rows[1])).toContain('unavailable, not free');
    expect(prose(rows[1])).toContain('partial total');
    expect(prose(rows[1])).toContain('do not guarantee');
    expect(prose(rows[2])).toContain('period, account scope, model and token buckets');
    expect(prose(rows[2])).toContain('does not prove billing or Patrol recovery');
    expect(prose(rows[3])).toContain('A budget change does not repair provider access');
  });

  it('makes the destructive reset warning prominent without an executable reset recipe', () => {
    const section = costSection();
    const warning = [...section.querySelectorAll('strong')].find(element =>
      prose(element).startsWith('Reset usage history'),
    );
    expect(warning).toBeDefined();
    expect(prose(warning!)).toBe(
      'Reset usage history clears retained events used by the budget check.',
    );
    expect(prose(section)).toContain('It does not refund provider charges');
    expect(prose(section)).toContain('Raising or disabling the budget permits more spend');
    expect(prose(section)).toContain('does not erase past usage');
    expect(prose(section)).toContain('a pricing correction does not require deleting those events');
    expect(section.querySelector('pre, form, input, button')).toBeNull();
    expect(prose(section)).not.toContain('POST /api/ai/cost/reset');
  });

  it('keeps reporting passive, private and linked to the existing help destination', () => {
    const section = costSection();
    expect(prose(section)).toContain('tasks, resources and sessions');
    expect(prose(section)).toContain('Unknown details are valid');
    expect(prose(section)).toContain(
      'no raw usage history, invoice, full export, credential, new diagnostic run or paid model call is required',
    );
    const link = section.querySelector('a[href="/docs/TROUBLESHOOTING#-getting-help"]');
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    expect(link?.hasAttribute('target')).toBe(false);
    expect(guide().querySelector('a[href="/docs/AI#cost-tracking"]')).not.toBeNull();
  });
});
