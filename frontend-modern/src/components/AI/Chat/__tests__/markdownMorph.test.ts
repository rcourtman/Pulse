import { describe, expect, it } from 'vitest';
import { morphMarkdownInto } from '../markdownMorph';

const make = (html: string): HTMLDivElement => {
  const el = document.createElement('div');
  morphMarkdownInto(el, html);
  return el;
};

describe('morphMarkdownInto', () => {
  it('renders the html into the container', () => {
    const el = make('<p>hello</p>');
    expect(el.innerHTML).toBe('<p>hello</p>');
  });

  it('preserves the DOM identity of unchanged leading blocks as the answer grows', () => {
    const el = make('<p>first</p><p>second</p>');
    const firstBefore = el.children[0];
    const secondBefore = el.children[1];

    // A streaming tick: a third paragraph appears, the earlier two are unchanged.
    morphMarkdownInto(el, '<p>first</p><p>second</p><p>third</p>');

    expect(el.children).toHaveLength(3);
    // Earlier blocks are the SAME nodes — not rebuilt — so they don't repaint.
    expect(el.children[0]).toBe(firstBefore);
    expect(el.children[1]).toBe(secondBefore);
    expect(el.children[2].textContent).toBe('third');
  });

  it('morphs a growing block in place, keeping its node identity and the prefix', () => {
    const el = make('<p>intro</p><p>par</p>');
    const introBefore = el.children[0];
    const tailBefore = el.children[1];

    // The last block grows; the intro is untouched and the growing <p> keeps its
    // node (only its text node updates), so nothing earlier repaints.
    morphMarkdownInto(el, '<p>intro</p><p>paragraph</p>');

    expect(el.children[0]).toBe(introBefore); // prefix kept
    expect(el.children[1]).toBe(tailBefore); // same <p>, text updated in place
    expect(el.children[1].textContent).toBe('paragraph');
  });

  it('morphs a changed earlier container in place and leaves following blocks alone', () => {
    const el = make('<ul><li>a</li></ul><p>after</p>');
    const listBefore = el.children[0];
    const afterBefore = el.children[1];

    // The list gained an item: the <ul> is morphed in place (new <li> appended),
    // and the unchanged <p> after it keeps its node.
    morphMarkdownInto(el, '<ul><li>a</li><li>b</li></ul><p>after</p>');

    expect(el.children[0]).toBe(listBefore); // <ul> morphed in place
    expect(el.querySelectorAll('li')).toHaveLength(2);
    expect(el.children[1]).toBe(afterBefore); // following block untouched
  });

  it('empties the container when the html is empty', () => {
    const el = make('<p>something</p>');
    morphMarkdownInto(el, '');
    expect(el.childNodes).toHaveLength(0);
  });

  it('preserves earlier list items as a streaming list grows (recurses into <ol>)', () => {
    const el = make('<ol><li>one</li><li>two</li></ol>');
    const olBefore = el.children[0];
    const li1Before = olBefore.children[0];
    const li2Before = olBefore.children[1];

    // A new list item streams in. The <ol> is the same node, and the first two
    // <li> are untouched — only the third is added.
    morphMarkdownInto(el, '<ol><li>one</li><li>two</li><li>three</li></ol>');

    expect(el.children[0]).toBe(olBefore); // same <ol>, morphed in place
    expect(olBefore.children).toHaveLength(3);
    expect(olBefore.children[0]).toBe(li1Before);
    expect(olBefore.children[1]).toBe(li2Before);
    expect(olBefore.children[2].textContent).toBe('three');
  });

  it('morphs the growing last list item in place, keeping earlier ones', () => {
    const el = make('<ul><li>alpha</li><li>be</li></ul>');
    const li1Before = el.querySelectorAll('li')[0];
    const li2Before = el.querySelectorAll('li')[1];

    morphMarkdownInto(el, '<ul><li>alpha</li><li>beta</li></ul>');

    const lis = el.querySelectorAll('li');
    expect(lis[0]).toBe(li1Before); // earlier item untouched
    expect(lis[1]).toBe(li2Before); // same <li>, text updated in place
    expect(lis[1].textContent).toBe('beta');
  });

  it('updates attributes in place without replacing the element', () => {
    const el = make('<a href="http://a.example">link</a>');
    const anchorBefore = el.children[0];

    morphMarkdownInto(el, '<a href="http://b.example" rel="noopener">link</a>');

    expect(el.children[0]).toBe(anchorBefore); // same node, attributes synced
    expect(anchorBefore.getAttribute('href')).toBe('http://b.example');
    expect(anchorBefore.getAttribute('rel')).toBe('noopener');
  });

  it('replaces a node when its tag changes (structure shifted)', () => {
    const el = make('<p>maybe a heading</p>');
    const pBefore = el.children[0];

    morphMarkdownInto(el, '<h2>maybe a heading</h2>');

    expect(el.children[0]).not.toBe(pBefore);
    expect(el.children[0].tagName).toBe('H2');
  });
});
