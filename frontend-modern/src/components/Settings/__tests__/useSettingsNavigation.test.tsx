import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { useSettingsNavigation } from '../useSettingsNavigation';

const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

function renderHarness(pathname = '/settings', search = '', hash = '') {
  return render(() => {
    const navigation = useSettingsNavigation({
      navigate: navigateSpy,
      location: {
        pathname,
        search,
        hash,
      },
    });

    return (
      <button type="button" onClick={() => navigation.setActiveTab('infrastructure-operations')}>
        open infrastructure settings
      </button>
    );
  });
}

describe('useSettingsNavigation', () => {
  afterEach(() => {
    cleanup();
    navigateSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
  });

  it('lands /settings on reporting inventory when the session is read-only', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderHarness('/settings');

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/operations', {
        replace: true,
        scroll: false,
      });
    });
  });

  it('routes infrastructure tab clicks to reporting inventory when the session is read-only', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderHarness('/settings/system-general');

    fireEvent.click(screen.getByRole('button', { name: 'open infrastructure settings' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/operations', {
      scroll: false,
    });
  });
});
