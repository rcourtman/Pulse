import { describe, expect, it } from 'vitest';
import {
  getRecoveryBreadcrumbLinkClass,
  getRecoveryDrawerCloseButtonClass,
  getRecoveryEmptyStateActionClass,
  getRecoveryFilterPanelClearClass,
} from '@/utils/recoveryActionPresentation';

describe('recoveryActionPresentation', () => {
  it('derives breadcrumb and filter clear link classes', () => {
    expect(getRecoveryBreadcrumbLinkClass()).toContain('text-blue-600');
    expect(getRecoveryFilterPanelClearClass()).toContain('text-blue-600');
  });

  it('derives empty-state action and drawer-close button classes', () => {
    expect(getRecoveryEmptyStateActionClass()).toContain('rounded-md border');
    expect(getRecoveryDrawerCloseButtonClass()).toContain('hover:bg-surface-hover');
  });
});
