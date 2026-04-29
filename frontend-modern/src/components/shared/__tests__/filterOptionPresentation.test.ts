import { describe, expect, it } from 'vitest';
import {
  FILTER_OPTION_ALL_LABEL,
  getAllFilterOptionLabel,
} from '@/components/shared/filterOptionPresentation';

describe('filterOptionPresentation', () => {
  it('builds canonical all-option labels without changing domain casing', () => {
    expect(FILTER_OPTION_ALL_LABEL).toBe('All');
    expect(getAllFilterOptionLabel('')).toBe('All');
    expect(getAllFilterOptionLabel(' roles ')).toBe('All roles');
    expect(getAllFilterOptionLabel('K8s clusters')).toBe('All K8s clusters');
  });
});
