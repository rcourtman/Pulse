import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const guide = readFileSync(path.join(root, 'docs/WEBHOOKS.md'), 'utf8');

function render(): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(guide, 'WEBHOOKS');
  return article;
}

describe('copyable webhook example', () => {
  it('ships and renders the escaped JSON template without changing copied source', () => {
    expect(readFileSync(path.join(root, 'frontend-modern/public/docs/WEBHOOKS.md'), 'utf8')).toBe(
      guide,
    );
    const example = guide.split('**Example Payload:**')[1]?.match(/```json\n([\s\S]*?)\n```/)?.[1];
    expect(example).toContain(
      '"text": "Alert: {{.Level | jsonString}} - {{.Message | jsonString}}"',
    );
    expect(example).toContain('"value": {{.Value}}');
    expect([...render().querySelectorAll('pre code')].map((block) => block.textContent)).toContain(
      `${example}\n`,
    );
  });

  it('keeps organisation selection, access refusals and private authentication beside the API example', () => {
    const article = render();
    const section = article.querySelector('#select-the-organisation-without-widening-access');
    expect(section).not.toBeNull();
    const text = article.textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('Selection is not authorisation.');
    expect(text).toContain("the user's administrator access does not widen it");
    expect(text).toContain('Adding scopes does not grant access to another organisation');
    const rows = [...article.querySelectorAll('tr')].map((row) => row.textContent);
    for (const code of ['400 invalid_org', '403 access_denied', '403 org_suspended']) {
      expect(rows.some((row) => row?.includes(code))).toBe(true);
    }
    expect(article.querySelector('a[href="/docs/API#api-token-recommended"]')?.textContent).toBe(
      'private header file',
    );
    const example = [...article.querySelectorAll('pre code')].find((block) =>
      block.textContent?.startsWith('POST /api/notifications/webhooks\n'),
    )?.textContent;
    expect(example).toContain('X-Pulse-Org-ID: acme-corp');
    expect(example).toContain('Authorization: Bearer <token with settings:write>');
    const config = JSON.parse(example!.split('\n\n')[1]);
    expect(config.url).toBe('https://psa.example.com/inbound/pulse');
    expect(config.headers.Authorization).toBe('Bearer <psa-token>');
    expect(example).not.toMatch(/Cookie:|pulse_session|--insecure/);
  });

  it('explains the simple-test blind spot and links the existing structured group example', () => {
    const article = render();
    const text = article.textContent?.replace(/\s+/g, ' ');
    expect(text).toContain("Keep jsonString inside the JSON string's quotes");
    expect(text).toContain('Leave numeric .Value unquoted');
    expect(text).toContain('A simple test message can work without escaping while a real alert');
    expect(text).toContain('invalid or altered JSON');
    expect(text).toContain('not a structured record of every group member');
    expect(text).toContain(
      'do not use the summary alone to deduplicate incidents or close tickets',
    );
    expect(article.querySelector('a[href="#sample-psa-payloads"]')?.textContent).toBe(
      'full PSA payload',
    );
    expect(article.querySelector('#sample-psa-payloads')).not.toBeNull();
  });
});
