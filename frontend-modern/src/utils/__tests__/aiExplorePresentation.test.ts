import { describe, expect, it } from 'vitest';
import { getAIExploreStatusPresentation } from '@/utils/aiExplorePresentation';

describe('aiExplorePresentation', () => {
  it('returns started presentation', () => {
    expect(getAIExploreStatusPresentation('started')).toMatchObject({
      label: 'Explore Started',
    });
    expect(getAIExploreStatusPresentation('started').classes).toContain('bg-sky-50');
  });

  it('returns completed presentation', () => {
    expect(getAIExploreStatusPresentation('completed')).toMatchObject({
      label: 'Explore Completed',
    });
    expect(getAIExploreStatusPresentation('completed').classes).toContain('bg-emerald-50');
  });

  it('returns default presentation for unknown phases', () => {
    expect(getAIExploreStatusPresentation('something-new')).toMatchObject({
      label: 'Explore Status',
    });
    expect(getAIExploreStatusPresentation('something-new').classes).toContain('bg-sky-50');
  });
});
