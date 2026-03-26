import { describe, expect, it } from 'vitest';

import recoverySummaryPresentationSource from '@/utils/recoverySummaryPresentation.ts?raw';
import recoveryHistorySectionSource from '@/components/Recovery/RecoveryHistorySection.tsx?raw';
import recoveryProtectedInventorySectionSource from '@/components/Recovery/RecoveryProtectedInventorySection.tsx?raw';

describe('recovery canonical vocabulary', () => {
  it('keeps shared recovery summary helpers item-first', () => {
    expect(recoverySummaryPresentationSource).toContain(
      'const getRecoverySummaryItemTypePresentation = (',
    );
    expect(recoverySummaryPresentationSource).not.toContain(
      'const getRecoverySummarySubjectTypePresentation = (',
    );
  });

  it('keeps recovery platform filter iterators platform-first', () => {
    expect(recoveryProtectedInventorySectionSource).toContain('{(platform) => (');
    expect(recoveryHistorySectionSource).toContain('{(platform) => (');
    expect(recoveryProtectedInventorySectionSource).not.toContain('{(provider) => (');
    expect(recoveryHistorySectionSource).not.toContain('{(provider) => (');
  });
});
