import { describe, expect, it } from 'vitest';
import { getStorageHealthPresentation } from '../healthPresentation';

describe('getStorageHealthPresentation', () => {
  it('returns canonical healthy presentation', () => {
    expect(getStorageHealthPresentation('healthy')).toEqual({
      label: 'Healthy',
      variant: 'success',
      dotClass: 'bg-green-500',
      countClass: 'text-muted',
    });
  });

  it('returns canonical warning presentation', () => {
    expect(getStorageHealthPresentation('warning')).toEqual({
      label: 'Warning',
      variant: 'warning',
      dotClass: 'bg-yellow-500',
      countClass: 'text-yellow-600 dark:text-yellow-400',
    });
  });

  it('returns canonical critical/offline/unknown presentation', () => {
    expect(getStorageHealthPresentation('critical')).toMatchObject({
      label: 'Critical',
      variant: 'danger',
      dotClass: 'bg-red-500',
    });
    expect(getStorageHealthPresentation('offline')).toMatchObject({
      label: 'Offline',
      variant: 'muted',
      dotClass: 'bg-slate-400',
    });
    expect(getStorageHealthPresentation('unknown')).toMatchObject({
      label: 'Unknown',
      variant: 'muted',
      dotClass: 'bg-slate-300',
    });
  });
});
