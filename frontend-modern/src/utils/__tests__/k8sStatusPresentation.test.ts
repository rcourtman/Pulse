import { describe, expect, it } from 'vitest';
import { getNamespaceCountsIndicator } from '@/utils/k8sStatusPresentation';

describe('k8sStatusPresentation', () => {
  it('treats offline counts as danger', () => {
    expect(
      getNamespaceCountsIndicator({ total: 4, online: 2, warning: 1, offline: 1, unknown: 0 }),
    ).toEqual({ variant: 'danger', label: 'Offline' });
  });

  it('treats warning counts as warning when nothing is offline', () => {
    expect(
      getNamespaceCountsIndicator({ total: 4, online: 3, warning: 1, offline: 0, unknown: 0 }),
    ).toEqual({ variant: 'warning', label: 'Warning' });
  });

  it('treats online counts as success when healthy', () => {
    expect(
      getNamespaceCountsIndicator({ total: 4, online: 4, warning: 0, offline: 0, unknown: 0 }),
    ).toEqual({ variant: 'success', label: 'Online' });
  });

  it('falls back to muted when no signal is available', () => {
    expect(
      getNamespaceCountsIndicator({ total: 0, online: 0, warning: 0, offline: 0, unknown: 0 }),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });
});
