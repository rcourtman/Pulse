import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import { useSettingsShellState } from '../useSettingsShellState';

const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

describe('useSettingsShellState', () => {
  afterEach(() => {
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
  });

  it('uses reporting-focused infrastructure copy in read-only sessions', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);

    createRoot((dispose) => {
      const state = useSettingsShellState({
        activeTab: () => 'infrastructure-operations',
      });

      expect(state.headerMeta().title).toBe('Infrastructure Operations');
      expect(state.headerMeta().description).toBe(
        'Review the current monitored-system inventory, reporting posture, and connected platform coverage. Setup changes stay unavailable in this read-only session.',
      );

      dispose();
    });
  });
});
