import { describe, expect, it } from 'vitest';
import {
  matchesStorageNodeTerms,
  parseStorageSearchQuery,
} from '@/features/storageBackups/storageSearchQuery';

describe('storageSearchQuery', () => {
  it('returns empty term lists for blank input', () => {
    expect(parseStorageSearchQuery('')).toEqual({ freeTerms: [], nodeTerms: [] });
    expect(parseStorageSearchQuery('   ')).toEqual({ freeTerms: [], nodeTerms: [] });
    expect(parseStorageSearchQuery('\t\n')).toEqual({ freeTerms: [], nodeTerms: [] });
  });

  it('collects free terms and lowercases them', () => {
    expect(parseStorageSearchQuery('foo').freeTerms).toEqual(['foo']);
    expect(parseStorageSearchQuery('FOO Bar').freeTerms).toEqual(['foo', 'bar']);
  });

  it('routes node: tokens into nodeTerms after stripping the prefix', () => {
    expect(parseStorageSearchQuery('node:foo')).toEqual({ freeTerms: [], nodeTerms: ['foo'] });
    expect(parseStorageSearchQuery('node:foo node:bar').nodeTerms).toEqual(['foo', 'bar']);
    // keeps everything after the first colon as the term, including extra colons
    expect(parseStorageSearchQuery('node:foo:bar').nodeTerms).toEqual(['foo:bar']);
  });

  it('keeps free and node terms separate regardless of token order', () => {
    expect(parseStorageSearchQuery('foo node:bar baz')).toEqual({
      freeTerms: ['foo', 'baz'],
      nodeTerms: ['bar'],
    });
    expect(parseStorageSearchQuery('node:alpha beta node:gamma')).toEqual({
      freeTerms: ['beta'],
      nodeTerms: ['alpha', 'gamma'],
    });
  });

  it('normalizes case for both free and node terms', () => {
    expect(parseStorageSearchQuery('FOO NODE:Bar BaZ')).toEqual({
      freeTerms: ['foo', 'baz'],
      nodeTerms: ['bar'],
    });
  });

  it('collapses leading, trailing, and repeated internal whitespace', () => {
    expect(parseStorageSearchQuery('  foo  ')).toEqual({ freeTerms: ['foo'], nodeTerms: [] });
    expect(parseStorageSearchQuery('foo\tbar   baz')).toEqual({
      freeTerms: ['foo', 'bar', 'baz'],
      nodeTerms: [],
    });
  });

  it('drops a bare node: prefix and does not bind a following whitespace-separated word', () => {
    // 'node:' with no attached value yields no node term...
    expect(parseStorageSearchQuery('node:')).toEqual({ freeTerms: [], nodeTerms: [] });
    // ...and a space-separated successor becomes a free term, not node:'s argument.
    expect(parseStorageSearchQuery('node: foo')).toEqual({ freeTerms: ['foo'], nodeTerms: [] });
    expect(parseStorageSearchQuery('node: foo node:bar')).toEqual({
      freeTerms: ['foo'],
      nodeTerms: ['bar'],
    });
  });

  it('matches when every node term is a substring of at least one hint', () => {
    expect(matchesStorageNodeTerms(['node-foo'], ['foo'])).toBe(true);
    expect(matchesStorageNodeTerms(['node-alpha', 'node-beta'], ['alpha', 'beta'])).toBe(true);
    // a term may match against a different hint than the others
    expect(matchesStorageNodeTerms(['aaa', 'bbb'], ['a', 'b'])).toBe(true);
  });

  it('rejects when any node term is absent from every hint', () => {
    expect(matchesStorageNodeTerms(['node-foo'], ['foo', 'bar'])).toBe(false);
    expect(matchesStorageNodeTerms(['node-a'], ['b'])).toBe(false);
    expect(matchesStorageNodeTerms([], ['foo'])).toBe(false);
  });

  it('treats an empty node term set as a universal match', () => {
    expect(matchesStorageNodeTerms(['node-foo'], [])).toBe(true);
    expect(matchesStorageNodeTerms([], [])).toBe(true);
  });

  it('normalizes hints case-insensitively and trims surrounding whitespace', () => {
    expect(matchesStorageNodeTerms(['NODE-FOO'], ['foo'])).toBe(true);
    expect(matchesStorageNodeTerms(['  Node-Foo  '], ['foo'])).toBe(true);
  });

  it('matches via substring (partial) as well as exact equality', () => {
    expect(matchesStorageNodeTerms(['foo'], ['foo'])).toBe(true);
    expect(matchesStorageNodeTerms(['node-foo-cluster'], ['foo', 'cluster'])).toBe(true);
  });

  it('ignores blank hints when searching for a match', () => {
    expect(matchesStorageNodeTerms(['', '  '], ['foo'])).toBe(false);
    expect(matchesStorageNodeTerms(['', 'node-foo'], ['foo'])).toBe(true);
  });
});
