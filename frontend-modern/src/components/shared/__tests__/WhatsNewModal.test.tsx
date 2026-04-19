import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { WhatsNewModal } from '@/components/shared/WhatsNewModal';
import whatsNewModalSource from '@/components/shared/WhatsNewModal.tsx?raw';
import whatsNewModalModelSource from '@/components/shared/whatsNewModalModel.ts?raw';
import whatsNewModalStateSource from '@/components/shared/useWhatsNewModalState.ts?raw';
import { STORAGE_KEYS } from '@/utils/localStorage';

const presentationPolicyIsDemoModeMock = vi.hoisted(() => vi.fn(() => false));
const sessionPresentationPolicyResolvedMock = vi.hoisted(() => vi.fn(() => true));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsDemoMode: presentationPolicyIsDemoModeMock,
  sessionPresentationPolicyResolved: sessionPresentationPolicyResolvedMock,
}));

describe('WhatsNewModal', () => {
  beforeEach(() => {
    localStorage.clear();
    presentationPolicyIsDemoModeMock.mockReturnValue(false);
    sessionPresentationPolicyResolvedMock.mockReturnValue(true);
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps whats new modal on shell, runtime, and model owners', () => {
    expect(whatsNewModalSource).toContain('useWhatsNewModalState');
    expect(whatsNewModalSource).toContain('useDialogState');
    expect(whatsNewModalSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalSource).toContain('Portal');
    expect(whatsNewModalSource).not.toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalSource).not.toContain('createSignal');
    expect(whatsNewModalSource).not.toContain('WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalSource).not.toContain('Migration guide');
    expect(whatsNewModalSource).not.toContain('https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md');
    expect(whatsNewModalSource).not.toContain('bg-gradient');
    expect(whatsNewModalSource).not.toContain('backdrop-blur-sm');

    expect(whatsNewModalStateSource).toContain('export function useWhatsNewModalState');
    expect(whatsNewModalStateSource).toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalStateSource).toContain('createSignal');
    expect(whatsNewModalStateSource).toContain('createMemo');
    expect(whatsNewModalStateSource).toContain('handleNext');
    expect(whatsNewModalStateSource).toContain('handlePrevious');
    expect(whatsNewModalStateSource).toContain('spotlightStyle');
    expect(whatsNewModalStateSource).toContain('STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalStateSource).toContain('sessionPresentationPolicyResolved');
    expect(whatsNewModalStateSource).toContain('presentationPolicyIsDemoMode');
    expect(whatsNewModalStateSource).toContain('handleClose');

    expect(whatsNewModalModelSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_TELEMETRY_TITLE');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_PRIVACY_URL');
    expect(whatsNewModalModelSource).toContain('MIGRATION_GUIDE_DOC_URL');
    expect(whatsNewModalModelSource).toContain('PRIVACY_DOC_URL');
    expect(whatsNewModalModelSource).toContain('anonymous daily ping');
    expect(whatsNewModalModelSource).toContain('nothing is gone');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_KICKER_LABEL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_STEP_MAP_LABEL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_TELEMETRY_LABEL');
    expect(whatsNewModalModelSource).not.toContain('https://github.com/rcourtman/Pulse/blob/main/docs/README.md');
    expect(whatsNewModalModelSource).not.toContain('https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_LABEL');
    expect(whatsNewModalModelSource).toContain("title: 'Dashboard'");
  });

  it('renders when the navigation modal has not been seen yet', async () => {
    render(() => <WhatsNewModal />);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText('Welcome to Pulse v6')).toBeInTheDocument();
    expect(within(dialog).getByText(/nothing is gone/i)).toBeInTheDocument();
    expect(within(dialog).getByText('Step 1 of 5')).toBeInTheDocument();
    expect(within(dialog).getByText('Where Things Moved')).toBeInTheDocument();
    expect(within(dialog).queryByText(/Stop 1/i)).not.toBeInTheDocument();
    expect(within(dialog).getAllByText('Dashboard')).toHaveLength(2);
  });

  it('stays hidden for public demo sessions', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    render(() => <WhatsNewModal />);

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
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

  it('advances through the guided tour and finishes on the last step', async () => {
    render(() => <WhatsNewModal />);

    expect(await screen.findByText(/overall picture: health, alerts, capacity/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    expect(await screen.findByText(/systems themselves: Proxmox nodes, Docker hosts/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    expect(await screen.findByText(/guests or Docker workloads in v5/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    expect(await screen.findByText(/Datastores, pools, disks, and capacity moved here/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    expect(await screen.findByText(/Backups, snapshots, and replication moved here/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: "Let's go" })).toBeInTheDocument();
  });

  it('lets the user jump to a tour stop directly from the stop map', async () => {
    render(() => <WhatsNewModal />);

    expect(await screen.findByText(/overall picture: health, alerts, capacity/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Workloads/i }));

    expect(await screen.findByText(/guests or Docker workloads in v5/i)).toBeInTheDocument();
    expect(screen.getByText('Step 3 of 5')).toBeInTheDocument();
  });

  it('routes the docs CTA through the migration guide', async () => {
    render(() => <WhatsNewModal />);

    const docsLink = await screen.findByRole('link', { name: 'Migration guide' });
    expect(docsLink).toHaveAttribute('href', '/docs/MIGRATION_UNIFIED_NAV.md');
  });
});
