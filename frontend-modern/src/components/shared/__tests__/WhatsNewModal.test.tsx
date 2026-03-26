import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { WhatsNewModal } from '@/components/shared/WhatsNewModal';
import whatsNewModalSource from '@/components/shared/WhatsNewModal.tsx?raw';
import whatsNewModalModelSource from '@/components/shared/whatsNewModalModel.ts?raw';
import whatsNewModalStateSource from '@/components/shared/useWhatsNewModalState.ts?raw';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('WhatsNewModal', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps whats new modal on shell, runtime, and model owners', () => {
    expect(whatsNewModalSource).toContain('useWhatsNewModalState');
    expect(whatsNewModalSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalSource).not.toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalSource).not.toContain('createSignal');
    expect(whatsNewModalSource).not.toContain('WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalSource).not.toContain('Infrastructure');
    expect(whatsNewModalSource).not.toContain('Documentation');
    expect(whatsNewModalSource).toContain('buildRecoveryPath');
    expect(whatsNewModalSource).not.toContain('href="/recovery?view=events&mode=remote"');
    expect(whatsNewModalSource).not.toContain('https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md');

    expect(whatsNewModalStateSource).toContain('export function useWhatsNewModalState');
    expect(whatsNewModalStateSource).toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalStateSource).toContain('createSignal');
    expect(whatsNewModalStateSource).toContain('STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalStateSource).toContain('handleClose');

    expect(whatsNewModalModelSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_TELEMETRY_TITLE');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_PRIVACY_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_LABEL');
    expect(whatsNewModalModelSource).toContain("title: 'Infrastructure'");
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

  it('routes the recovery CTA through the canonical recovery link helper', async () => {
    render(() => <WhatsNewModal />);

    const recoveryLink = await screen.findByRole('link', { name: 'Recovery events' });
    expect(recoveryLink).toHaveAttribute('href', '/recovery?view=events&mode=remote');
  });
});
