import { describe, expect, it } from 'vitest';
import { morphMarkdownInto } from '../markdownMorph';

// Branch-coverage companion to markdownMorph.test.ts. morphMarkdownInto is the
// only exported symbol; morphChildren / canMorphInPlace / morphNode /
// syncAttributes are all module-private, so every branch below is driven
// through morphMarkdownInto and observed via concrete DOM identity / innerHTML
// / attributes / nodeValue assertions.
//
// The existing sibling test covers: plain render, append-only growth (the
// `!oldNode` arm), in-place text growth inside a kept <p>/<li>, in-place list
// growth, full empty, in-place <ol> recursion, attribute add/update, and
// tag-change replacement (canMorphInPlace false via differing tagNames).
//
// This file targets the branches that sibling does NOT hit:
//   - canMorphInPlace `nodeType !== nodeType` arm (text<->element swap)
//   - canMorphInPlace COMMENT_NODE arm of the trailing return
//   - morphNode non-element (comment) update arm
//   - syncAttributes attribute-removal arm
//   - syncAttributes equal-value skip arm
//   - morphChildren trailing-drop while-loop with a kept prefix
//   - top-level bare-text morph (canMorphInPlace TEXT arm at top level)

const make = (html: string): HTMLDivElement => {
  const el = document.createElement('div');
  morphMarkdownInto(el, html);
  return el;
};

// ---------------------------------------------------------------------------
// canMorphInPlace — `oldNode.nodeType !== newNode.nodeType` arm (returns false)
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: nodeType mismatch forces replacement', () => {
  it('replaces a bare text node when the new render is an element (text -> element)', () => {
    const el = make('draft');
    expect(el.childNodes).toHaveLength(1);
    expect(el.childNodes[0].nodeType).toBe(Node.TEXT_NODE);
    const textBefore = el.childNodes[0];

    // The streaming tick turns the loose text into a wrapped <p>.
    morphMarkdownInto(el, '<p>draft</p>');

    // canMorphInPlace(text, p): TEXT !== ELEMENT -> false -> replaceChild.
    expect(el.childNodes[0]).not.toBe(textBefore);
    expect(el.childNodes[0].nodeType).toBe(Node.ELEMENT_NODE);
    expect((el.childNodes[0] as Element).tagName).toBe('P');
    expect(el.innerHTML).toBe('<p>draft</p>');
  });

  it('replaces an element when the new render is bare text (element -> text)', () => {
    const el = make('<p>x</p>');
    const elBefore = el.childNodes[0];

    morphMarkdownInto(el, 'x');

    // canMorphInPlace(p, text): ELEMENT !== TEXT -> false -> replaceChild.
    expect(el.childNodes[0]).not.toBe(elBefore);
    expect(el.childNodes[0].nodeType).toBe(Node.TEXT_NODE);
    expect(el.textContent).toBe('x');
  });
});

// ---------------------------------------------------------------------------
// canMorphInPlace — COMMENT_NODE arm of `TEXT_NODE || COMMENT_NODE` + morphNode
// non-element update arm for comments.
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: comment nodes', () => {
  it('morphs a changed comment in place (COMMENT arm + comment nodeValue update)', () => {
    const el = make('<!-- note -->');
    expect(el.childNodes).toHaveLength(1);
    expect(el.childNodes[0].nodeType).toBe(Node.COMMENT_NODE);
    const commentBefore = el.childNodes[0] as Comment;

    morphMarkdownInto(el, '<!-- note2 -->');

    // isEqualNode false (nodeValue differs) -> canMorphInPlace true (both
    // COMMENT) -> morphNode non-element arm updates nodeValue, identity kept.
    // A comment's data is the literal text between `<!--` and `-->`, so the
    // surrounding spaces are part of nodeValue.
    expect(el.childNodes[0]).toBe(commentBefore);
    expect(commentBefore.nodeValue).toBe(' note2 ');
    expect(el.innerHTML).toBe('<!-- note2 -->');
  });

  it('keeps an identical comment untouched (isEqualNode true for comments)', () => {
    const el = make('<!-- keep -->');
    const commentBefore = el.childNodes[0];

    morphMarkdownInto(el, '<!-- keep -->');

    expect(el.childNodes[0]).toBe(commentBefore);
  });
});

// ---------------------------------------------------------------------------
// syncAttributes — attribute-removal arm (old has an attr new lacks).
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: attribute removal', () => {
  it('removes an attribute that disappears from the new render, keeping node identity', () => {
    const el = make('<a href="http://a.example" title="t">l</a>');
    const anchorBefore = el.children[0];

    morphMarkdownInto(el, '<a href="http://a.example">l</a>');

    // syncAttributes: old has `title`, new lacks it -> removeAttribute arm.
    expect(el.children[0]).toBe(anchorBefore);
    expect(anchorBefore.getAttribute('href')).toBe('http://a.example');
    expect(anchorBefore.getAttribute('title')).toBeNull();
    expect(anchorBefore.textContent).toBe('l');
  });
});

// ---------------------------------------------------------------------------
// syncAttributes — equal-value skip arm (getAttribute === attr.value).
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: attribute value unchanged', () => {
  it('skips setAttribute when the value is already correct, while still adding/removing others', () => {
    const el = make('<a href="http://keep.example" class="c">l</a>');
    const anchorBefore = el.children[0];

    morphMarkdownInto(el, '<a href="http://keep.example" rel="noopener">l</a>');

    // href value is unchanged -> inner guard skips setAttribute (equal arm).
    // `class` is removed (removal arm) and `rel` is added (add arm).
    expect(el.children[0]).toBe(anchorBefore);
    expect(anchorBefore.getAttribute('href')).toBe('http://keep.example');
    expect(anchorBefore.getAttribute('class')).toBeNull();
    expect(anchorBefore.getAttribute('rel')).toBe('noopener');
  });
});

// ---------------------------------------------------------------------------
// morphChildren — trailing-drop while-loop with a kept prefix (drops run after
// the for-loop's continue path).
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: partial trailing drop', () => {
  it('drops one trailing block while keeping an equal leading block', () => {
    const el = make('<p>a</p><p>b</p>');
    const aBefore = el.children[0];
    const bBefore = el.children[1];

    morphMarkdownInto(el, '<p>a</p>');

    // for-loop: i=0, <p>a</p> === <p>a</p> -> isEqualNode continue. Then the
    // while-loop drops the trailing <p>b</p> (length 2 > 1 -> remove).
    expect(el.children).toHaveLength(1);
    expect(el.children[0]).toBe(aBefore);
    expect(bBefore.parentNode).toBeNull();
  });

  it('drops several trailing blocks at once while keeping the equal prefix', () => {
    const el = make('<p>1</p><p>2</p><p>3</p>');
    const firstBefore = el.children[0];

    morphMarkdownInto(el, '<p>1</p>');

    // The while-loop iterates twice: 3>1 remove, 2>1 remove, 1>1 exit.
    expect(el.children).toHaveLength(1);
    expect(el.children[0]).toBe(firstBefore);
    expect(el.children[0].textContent).toBe('1');
  });
});

// ---------------------------------------------------------------------------
// Top-level bare text: canMorphInPlace TEXT arm + morphNode non-element update
// at the container top level (existing sibling only exercises text inside
// kept elements like <p>/<li>).
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: top-level bare text', () => {
  it('morphs a growing bare text node in place at the container top level', () => {
    const el = make('abc');
    const textBefore = el.childNodes[0];
    expect(textBefore.nodeType).toBe(Node.TEXT_NODE);

    morphMarkdownInto(el, 'abcd');

    // isEqualNode false -> canMorphInPlace(text, text) true (TEXT arm) ->
    // morphNode non-element arm updates nodeValue, identity kept.
    expect(el.childNodes[0]).toBe(textBefore);
    expect(el.textContent).toBe('abcd');
  });

  it('keeps an identical bare text node untouched', () => {
    const el = make('same');
    const textBefore = el.childNodes[0];

    morphMarkdownInto(el, 'same');

    expect(el.childNodes[0]).toBe(textBefore);
    expect(el.textContent).toBe('same');
  });

  it('appends a second top-level text node when one is already present', () => {
    const el = make('one');
    const firstBefore = el.childNodes[0];

    morphMarkdownInto(el, 'onetwo');

    expect(el.childNodes).toHaveLength(1);
    expect(el.childNodes[0]).toBe(firstBefore);
    expect(el.textContent).toBe('onetwo');
  });
});

// ---------------------------------------------------------------------------
// Multi-node initial render: the `!oldNode` arm fires repeatedly into an empty
// container, covering repeated appendChild across mixed element/text children.
// ---------------------------------------------------------------------------

describe('morphMarkdownInto branch coverage: multi-node append into empty container', () => {
  it('appends several top-level nodes (elements + trailing text) in one tick', () => {
    const el = document.createElement('div');

    morphMarkdownInto(el, '<p>first</p><p>second</p>tail');

    expect(el.childNodes).toHaveLength(3);
    expect((el.childNodes[0] as Element).tagName).toBe('P');
    expect((el.childNodes[0] as Element).textContent).toBe('first');
    expect((el.childNodes[1] as Element).tagName).toBe('P');
    expect((el.childNodes[1] as Element).textContent).toBe('second');
    expect(el.childNodes[2].nodeType).toBe(Node.TEXT_NODE);
    expect(el.childNodes[2].nodeValue).toBe('tail');
  });
});
