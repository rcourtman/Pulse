import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, vi } from 'vitest';
import { renderDocMarkdown, scrollToDocFragment } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const source = (name: string): string =>
  readFileSync(path.join(root, 'frontend-modern/public/docs', `${name}.md`), 'utf8');
const renderGuide = (name: string): HTMLElement => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source(name), name);
  return article;
};
const prose = (article: HTMLElement): string => article.textContent!.replace(/\s+/g, ' ');
const section = (article: HTMLElement, id: string): HTMLElement => {
  const heading = article.querySelector(`[id="${id}"]`)!;
  expect(heading, id).not.toBeNull();
  const result = document.createElement('div');
  for (
    let node = heading.nextElementSibling;
    node && !/^H[1-3]$/.test(node.tagName);
    node = node.nextElementSibling
  )
    result.append(node.cloneNode(true));
  return result;
};

describe('shipped Cloud availability guidance', () => {
  it('keeps the availability boundary ahead of the setup instructions', () => {
    const article = renderGuide('CLOUD');
    const content = prose(article);
    expect(content.indexOf('Pulse Cloud is not generally available.')).toBeLessThan(
      content.indexOf('How It Works'),
    );
    expect(content).toContain('buying Pro does not create a hosted server');
    expect(content).toContain('Do not invent a Cloud hostname');
    expect(content).not.toContain('Sign up via the Pulse Cloud portal');
    expect(content).not.toContain('provisioned automatically after checkout');
    expect(article.querySelector('a[href="/docs/INSTALL"]')).not.toBeNull();
    expect(article.querySelector('a[href="/docs/PULSE_PRO"]')).not.toBeNull();
  });

  it('routes the existing FAQ and plan entry points to scoped hosted help', () => {
    const faq = section(renderGuide('FAQ'), 'what-do-pro-and-cloud-unlock');
    const plans = section(renderGuide('PULSE_PRO'), 'cloud');
    for (const content of [prose(faq), prose(plans)]) {
      expect(content).toMatch(/Cloud is not generally available/);
      expect(content).toContain('not a public sign-up or provisioning offer');
    }
    expect(
      faq.querySelector('a[href="/docs/CLOUD#1-confirm-existing-hosted-access"]'),
    ).not.toBeNull();
    expect(faq.querySelector('a[href="/docs/PULSE_PRO#cloud"]')).not.toBeNull();
    expect(plans.querySelector('a[href="/docs/CLOUD"]')).not.toBeNull();
  });

  it('keeps existing-instance access and migration advice, without a purchase instruction', () => {
    const article = renderGuide('CLOUD');
    const content = prose(section(article, '1-confirm-existing-hosted-access'));
    expect(content).toContain('operator has supplied an instance and an access method');
    expect(content).toContain('rather than purchasing again');
    expect(content).toContain('Keep any working source installation intact');
    expect(prose(article)).toContain('do not cancel it or discard the source');
    expect(
      article.querySelector(
        'a[href="/docs/UNIFIED_AGENT#private-file-installation-linux-macos-and-nas"]',
      ),
    ).not.toBeNull();
    for (const fragment of [
      'migrating-tofrom-cloud',
      'cloud--self-hosted',
      'verify-the-move-before-retiring-the-source',
      '4-set-up-mobile-access',
    ]) {
      expect(article.querySelector(`[id="${fragment}"]`), fragment).not.toBeNull();
    }
  });

  it('preserves the old account fragment in the actual sanitised renderer', () => {
    const article = renderGuide('CLOUD');
    const target =
      article.querySelector('[id="1-create-your-account"]') ??
      article.querySelector('[id="user-content-1-create-your-account"]');
    expect(target).not.toBeNull();
    expect(target!.textContent).toBe('1. Confirm Existing Hosted Access');
    expect(target!.closest('h3')?.id).toBe('1-confirm-existing-hosted-access');
    const scroll = vi.fn();
    target!.scrollIntoView = scroll;
    document.body.append(article);
    try {
      scrollToDocFragment(article, '#1-create-your-account');
      expect(scroll).toHaveBeenCalledWith({ block: 'start' });
      expect(document.activeElement).toBe(target);
    } finally {
      article.remove();
    }
    expect(article.querySelector('script, iframe, form')).toBeNull();
  });

  it('ships byte-identical canonical and served guides and retains the FAQ fragment', () => {
    for (const name of ['CLOUD', 'PULSE_PRO', 'FAQ']) {
      expect(source(name)).toBe(readFileSync(path.join(root, 'docs', `${name}.md`), 'utf8'));
    }
    expect(renderGuide('FAQ').querySelector('#what-do-pro-and-cloud-unlock')).not.toBeNull();
  });
});
