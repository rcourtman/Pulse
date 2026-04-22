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
    vi.unstubAllGlobals();
  });

  it('uses reporting-focused infrastructure copy in read-only sessions', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);

    createRoot((dispose) => {
      const state = useSettingsShellState({
        activeTab: () => 'infrastructure-systems',
      });

      expect(state.headerMeta().title).toBe('Infrastructure');
      expect(state.headerMeta().description).toBe(
        'Review the current top-level monitored systems and reporting posture. Setup changes stay unavailable in this read-only session.',
      );

      dispose();
    });
  });

  it('keeps the content pane visible on mobile-sized viewports by default', () => {
    vi.stubGlobal('window', { innerWidth: 390 });

    createRoot((dispose) => {
      const state = useSettingsShellState({
        activeTab: () => 'infrastructure-systems',
      });

      expect(state.isMobileMenuOpen()).toBe(false);

      dispose();
    });
  });
});
