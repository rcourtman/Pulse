import { describe, expect, it } from 'vitest';

import * as utils from '@/components/AI/aiChatUtils';

// Branch-coverage companion to aiChatUtils.test.ts. Both target functions
// (coerceMarkdownInput, configureDOMPurify) are module-private, so every
// branch below is driven through the public renderMarkdown entry point and
// asserted against concrete rendered output. renderMarkdown's signature is
// (content: unknown), so non-string inputs type-check without casts; a branch
// is pinned by showing renderMarkdown(<branch input>) equals renderMarkdown of
// the exact string that branch must coerce to.

// ---------------------------------------------------------------------------
// coerceMarkdownInput — every arm of its input-type ladder.
// ---------------------------------------------------------------------------

describe('coerceMarkdownInput branch coverage', () => {
  it('returns a string input unchanged (string branch -> rendered as markdown)', () => {
    // A real markdown string reaches marked.parse directly: the heading is
    // rendered, proving the value was NOT routed through String()/JSON first.
    const out = utils.renderMarkdown('# Heading One');
    expect(out).toContain('<h1');
    expect(out).toContain('Heading One');
  });

  it('extracts record.text from an object (object.text branch)', () => {
    const out = utils.renderMarkdown({ text: 'alpha' });
    expect(out).toEqual(utils.renderMarkdown('alpha'));
    // Discriminator: the JSON.stringify arm would have emitted the literal
    // {"text":"alpha"} payload instead.
    expect(out).not.toContain('"text"');
  });

  it('extracts record.content when record.text is absent (object.content branch)', () => {
    const out = utils.renderMarkdown({ content: 'beta' });
    expect(out).toEqual(utils.renderMarkdown('beta'));
    expect(out).not.toContain('"content"');
  });

  it('prefers record.text over record.content (precedence between the two arms)', () => {
    const out = utils.renderMarkdown({ text: 'from-text', content: 'from-content' });
    expect(out).toEqual(utils.renderMarkdown('from-text'));
    expect(out).not.toContain('from-content');
  });

  it('JSON.stringifies an object with neither text nor content (JSON branch)', () => {
    const out = utils.renderMarkdown({ foo: 'bar', n: 42 });
    expect(out).toEqual(utils.renderMarkdown('{"foo":"bar","n":42}'));
    expect(out).toContain('"foo":"bar"');
    expect(out).toContain('"n":42');
  });

  it('falls back to String() when JSON.stringify throws on a circular object (catch branch)', () => {
    const circular: Record<string, unknown> = {};
    circular.self = circular; // JSON.stringify throws -> catch -> String(content)
    const out = utils.renderMarkdown(circular);
    expect(out).toEqual(utils.renderMarkdown('[object Object]'));
    expect(out).toContain('[object Object]');
  });

  it('coerces null to empty string (null arm of the final ternary)', () => {
    expect(utils.renderMarkdown(null)).toBe('');
  });

  it('coerces undefined to empty string (undefined arm of the final ternary)', () => {
    expect(utils.renderMarkdown(undefined)).toBe('');
  });

  it('coerces a number primitive via String() (non-null primitive arm)', () => {
    expect(utils.renderMarkdown(42)).toEqual(utils.renderMarkdown('42'));
    expect(utils.renderMarkdown(42)).toContain('42');
  });

  it('coerces a falsy number (0) via String(), not the null branch', () => {
    // 0 is falsy so `content && typeof === 'object'` short-circuits, but
    // 0 != null, so it must still reach String(0) === '0' rather than ''.
    expect(utils.renderMarkdown(0)).toEqual(utils.renderMarkdown('0'));
    expect(utils.renderMarkdown(0)).toContain('0');
  });

  it('coerces a boolean primitive via String() (true and false)', () => {
    expect(utils.renderMarkdown(true)).toEqual(utils.renderMarkdown('true'));
    expect(utils.renderMarkdown(false)).toEqual(utils.renderMarkdown('false'));
    // false is falsy but not null -> must NOT collapse to ''.
    expect(utils.renderMarkdown(false)).not.toBe('');
  });
});

// ---------------------------------------------------------------------------
// configureDOMPurify — the idempotency guard and both sanitize hooks, driven
// through renderMarkdown. vitest isolates modules per file, so the first
// renderMarkdown call in this file runs the configure body; later calls take
// the early-return.
// ---------------------------------------------------------------------------

describe('configureDOMPurify branch coverage', () => {
  it('configures hooks exactly once and keeps them active on later calls (early-return guard)', () => {
    const md = '[link](https://example.com)';
    const first = utils.renderMarkdown(md);
    const second = utils.renderMarkdown(md);
    const third = utils.renderMarkdown(md);
    // Identical output every call: the `if (domPurifyConfigured) return`
    // guard short-circuits calls 2 and 3, yet the afterSanitizeAttributes
    // hook registered on call 1 still fires.
    expect(second).toEqual(first);
    expect(third).toEqual(first);
    expect(first).toContain('href="https://example.com"');
    expect(first).toContain('target="_blank"');
    expect(first).toContain('rel="noopener noreferrer"');
  });

  it('does not add target/rel to non-anchor elements (afterSanitizeAttributes non-A arm)', () => {
    // <div> hits the `element.tagName !== 'A'` early return inside the hook.
    const out = utils.renderMarkdown('<div>box</div>');
    expect(out).toContain('<div>box</div>');
    expect(out).not.toContain('target=');
    expect(out).not.toContain('rel=');
  });

  it('keeps a valid language-x class on <code> (uponSanitizeAttribute code+class+regex-true arm)', () => {
    const out = utils.renderMarkdown(['```ts', 'const x = 1;', '```'].join('\n'));
    expect(out).toContain('<code');
    expect(out).toContain('language-ts');
  });

  it('drops an invalid class on <code> (uponSanitizeAttribute code+class+regex-false arm)', () => {
    // 'highlight' fails ^language-[a-z0-9+#_-]{1,30}$ -> forceKeepAttr stays
    // unset -> DOMPurify strips 'class' (not in ALLOWED_ATTR).
    const out = utils.renderMarkdown('<code class="highlight">x</code>');
    expect(out).toContain('<code>x</code>');
    expect(out).not.toContain('class=');
    expect(out).not.toContain('highlight');
  });

  it('drops a language-x class on a non-code element (uponSanitizeAttribute non-CODE arm)', () => {
    const out = utils.renderMarkdown('<div class="language-bash">z</div>');
    expect(out).toContain('>z<');
    expect(out).not.toContain('class=');
    expect(out).not.toContain('language-bash');
  });

  it('ignores non-class attributes on <code> while still validating class (attrName !== class arm)', () => {
    // The hook processes 'id' (attrName !== 'class' -> early return; the attr
    // is then stripped by DOMPurify) and 'class' (validated + kept).
    const out = utils.renderMarkdown('<code id="ignored" class="language-js">y</code>');
    expect(out).toContain('language-js');
    expect(out).not.toContain('id=');
    expect(out).not.toContain('ignored');
  });
});
