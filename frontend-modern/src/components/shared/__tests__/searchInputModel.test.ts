import { describe, expect, it } from 'vitest';
import {
  rankSearchInputSuggestions,
  resolveSearchInputInlineCompletion,
  scoreSearchInputSuggestion,
} from '@/components/shared/searchInputModel';

describe('searchInputModel', () => {
  it('matches the inline completion text as well as visible labels and infrastructure keywords', () => {
    const suggestion = {
      id: 'filter:node:pve2',
      label: 'Node: West Production B',
      completion: 'pve2',
      keywords: ['production-west'],
    };

    expect(scoreSearchInputSuggestion(suggestion, 'pve')).toBe(1);
    expect(scoreSearchInputSuggestion(suggestion, 'West')).toBe(2);
    expect(scoreSearchInputSuggestion(suggestion, 'production-west')).toBe(0);
  });

  it('keeps the source group order while ranking stronger matches within each group', () => {
    const ranked = rankSearchInputSuggestions(
      [
        { id: 'filter', label: 'Node: pve2', completion: 'pve2', group: 'Filters' },
        { id: 'weak-filter', label: 'Cluster: production-pve', group: 'Filters' },
        { id: 'resource', label: 'pve2', group: 'Infrastructure' },
      ],
      'pve',
    );

    expect(ranked.map((suggestion) => suggestion.id)).toEqual([
      'filter',
      'weak-filter',
      'resource',
    ]);
  });

  it('completes only the unambiguous common prefix when several objects match', () => {
    const suggestions = [
      { id: 'deployment:checkout-api', label: 'checkout-api' },
      { id: 'deployment:checkout-web', label: 'checkout-web' },
      { id: 'pod:checkout-api-a', label: 'checkout-api-6c746d5bcf-c7z2p' },
    ];

    expect(resolveSearchInputInlineCompletion(suggestions, 'chec')?.text).toBe('checkout-');
    expect(resolveSearchInputInlineCompletion(suggestions, 'checkout-api-6c746d5bcf-c')?.text).toBe(
      'checkout-api-6c746d5bcf-c7z2p',
    );
  });
});
