import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { WhatsNewModal } from '@/components/shared/WhatsNewModal';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('WhatsNewModal', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('renders when the navigation modal has not been seen yet', async () => {
    render(() => <WhatsNewModal />);

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });
    expect(screen.getByText('Welcome to the New Navigation!')).toBeInTheDocument();
  });

  it('closes on backdrop click and records the modal as seen by default', async () => {
    render(() => <WhatsNewModal />);

    const backdrop = await waitFor(() => {
      const element = document.querySelector('[data-dialog-backdrop]') as HTMLElement | null;
      expect(element).not.toBeNull();
      return element!;
    });

    fireEvent.click(backdrop);

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN)).toBe('true');
  });

  it('supports a session-only dismissal when "Don\'t show again" is unchecked', async () => {
    render(() => <WhatsNewModal />);

    const checkbox = await screen.findByRole('checkbox', { name: "Don't show again" });
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(localStorage.getItem(STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN)).toBe('false');

    cleanup();
    render(() => <WhatsNewModal />);

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });
  });
});
