import { describe, expect, it } from 'vitest';

import recoveryActivitySectionSource from '@/components/Recovery/RecoveryActivitySection.tsx?raw';
import recoveryPageSource from '@/components/Recovery/Recovery.tsx?raw';
import recoveryPointDetailsSource from '@/components/Recovery/RecoveryPointDetails.tsx?raw';
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

  it('keeps recovery activity focus labels item-first', () => {
    expect(recoveryPageSource).toContain('const selectedHistoryItemLabel = createMemo(() => {');
    expect(recoveryPageSource).not.toContain('const selectedHistorySubjectLabel = createMemo(() => {');
    expect(recoveryActivitySectionSource).toContain('selectedHistoryItemLabel: Accessor<string | null>;');
    expect(recoveryActivitySectionSource).not.toContain(
      'selectedHistorySubjectLabel: Accessor<string | null>;',
    );
  });

  it('keeps recovery detail platform helpers platform-first', () => {
    expect(recoveryPointDetailsSource).toContain(
      "const isPbsPlatform = createMemo(() => platformKey() === 'proxmox-pbs');",
    );
    expect(recoveryPointDetailsSource).not.toContain(
      "const isPbsProvider = createMemo(() => platformKey() === 'proxmox-pbs');",
    );
  });
});
