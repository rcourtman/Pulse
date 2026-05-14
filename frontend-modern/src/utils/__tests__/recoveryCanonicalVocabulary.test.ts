import { describe, expect, it } from 'vitest';

import recoveryPointDetailsSource from '@/components/Recovery/RecoveryPointDetails.tsx?raw';
import recoverySurfaceStateSource from '@/features/recovery/useRecoverySurfaceState.ts?raw';
import recoveryHistorySectionSource from '@/components/Recovery/RecoveryHistorySection.tsx?raw';
import recoveryProtectedInventorySectionSource from '@/components/Recovery/RecoveryProtectedInventorySection.tsx?raw';

describe('recovery canonical vocabulary', () => {
  it('keeps recovery platform filter iterators platform-first', () => {
    // The intent of this guardrail is the canonical "platform" naming over
    // legacy "provider", not a specific callback shape. Both files now use
    // block-body callbacks (`(platform) => { const badge = ...; return ... }`)
    // because they need a local for the badge resolution, so the original
    // expression-body pattern `(platform) => (` no longer matches.
    expect(recoveryProtectedInventorySectionSource).toContain('(platform) =>');
    expect(recoveryHistorySectionSource).toContain('(platform) =>');
    expect(recoveryProtectedInventorySectionSource).not.toContain('(provider) =>');
    expect(recoveryHistorySectionSource).not.toContain('(provider) =>');
  });

  it('keeps recovery activity focus labels item-first', () => {
    expect(recoverySurfaceStateSource).toContain(
      'const selectedHistoryItemLabel = createMemo(() => {',
    );
    expect(recoverySurfaceStateSource).not.toContain(
      'const selectedHistorySubjectLabel = createMemo(() => {',
    );
    expect(recoveryHistorySectionSource).toContain(
      'selectedHistoryItemLabel: Accessor<string | null>;',
    );
    expect(recoveryHistorySectionSource).not.toContain(
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
