import { describe, expect, it } from 'vitest';
import { getSourceTypeLabel, getSourceTypePresentation } from '@/utils/sourceTypePresentation';

describe('sourceTypePresentation', () => {
  it('returns canonical source-type presentation for known types', () => {
    expect(getSourceTypePresentation('agent')).toMatchObject({ label: 'Agent' });
    expect(getSourceTypePresentation('api')).toMatchObject({ label: 'API' });
    expect(getSourceTypePresentation('hybrid')).toMatchObject({ label: 'Hybrid' });
  });

  it('returns fallback labels for unknown types and null for empty values', () => {
    expect(getSourceTypePresentation('unknown-source')).toBeNull();
    expect(getSourceTypeLabel('unknown-source')).toBe('unknown-source');
    expect(getSourceTypeLabel(undefined)).toBeNull();
  });
});
