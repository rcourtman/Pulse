import { describe, it, expect } from 'vitest';
import { orderFactsByActionability } from '../factOrdering';
import type { DiscoveryFact } from '@/types/discovery';

const fact = (category: string, key: string): DiscoveryFact => ({
  category: category as DiscoveryFact['category'],
  key,
  value: key,
  source: 'test',
  confidence: 0.9,
  discovered_at: '2026-01-01T00:00:00Z',
});

describe('orderFactsByActionability', () => {
  it('orders the most actionable categories first', () => {
    const ordered = orderFactsByActionability([
      fact('version', 'v'),
      fact('storage', 's'),
      fact('service', 'restart'),
      fact('hardware', 'gpu'),
    ]);
    expect(ordered.map((f) => f.category)).toEqual(['service', 'hardware', 'version', 'storage']);
  });

  it('keeps a trailing service-control fact within the first 8 of many', () => {
    const facts: DiscoveryFact[] = [];
    for (let i = 0; i < 10; i += 1) facts.push(fact('version', `v${i}`));
    facts.push(fact('service', 'restart')); // 11th — dropped by a naive slice(0, 8)

    const top8 = orderFactsByActionability(facts).slice(0, 8);
    expect(top8.some((f) => f.key === 'restart')).toBe(true);
  });

  it('is stable within a category (preserves analyzer order)', () => {
    const ordered = orderFactsByActionability([
      fact('version', 'a'),
      fact('version', 'b'),
      fact('version', 'c'),
    ]);
    expect(ordered.map((f) => f.key)).toEqual(['a', 'b', 'c']);
  });

  it('does not mutate the input', () => {
    const input = [fact('version', 'v'), fact('service', 's')];
    const snapshot = input.map((f) => f.key);
    orderFactsByActionability(input);
    expect(input.map((f) => f.key)).toEqual(snapshot);
  });
});
