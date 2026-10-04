import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadMetadataId } from '@/utils/workloads';
import {
  RESOURCE_METADATA_CHANGED_EVENT,
  type ResourceMetadataChangedDetail,
} from '@/utils/resourceMetadataEvents';
import { WorkloadWebLinksAction } from '../WorkloadWebLinksDialog';
import type { WorkloadGuestMetadataMap } from '../workloadWebLinksModel';

const { updateMetadataMock, successMock, readOnlyMock } = vi.hoisted(() => ({
  updateMetadataMock: vi.fn(),
  successMock: vi.fn(),
  readOnlyMock: vi.fn(() => false),
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: { updateMetadata: updateMetadataMock },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: successMock, error: vi.fn(), info: vi.fn() },
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: readOnlyMock,
}));

const makeGuest = (overrides: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    node: 'pve1',
    instance: 'cluster',
    status: 'running',
    type: 'lxc',
    ...overrides,
  }) as WorkloadGuest;

const auth = makeGuest({ id: 'auth', vmid: 107, name: 'auth-service-01' });
const billing = makeGuest({ id: 'billing', vmid: 108, name: 'billing-worker-01' });
const cache = makeGuest({ id: 'cache', vmid: 117, name: 'artifact-cache-01' });
const authId = getWorkloadMetadataId(auth);
const billingId = getWorkloadMetadataId(billing);
const cacheId = getWorkloadMetadataId(cache);

const renderAction = (metadata: WorkloadGuestMetadataMap = {}) => {
  const [guests] = createSignal([auth, billing, cache]);
  const [guestMetadata, setGuestMetadata] = createSignal(metadata);
  const events: ResourceMetadataChangedDetail[] = [];
  const onChanged = (event: Event) => {
    const detail = (event as CustomEvent<ResourceMetadataChangedDetail>).detail;
    events.push(detail);
    setGuestMetadata((current) => ({
      ...current,
      [detail.metadataId!]: { id: detail.metadataId!, customUrl: detail.customUrl },
    }));
  };
  window.addEventListener(RESOURCE_METADATA_CHANGED_EVENT, onChanged);
  render(() => <WorkloadWebLinksAction guests={guests} guestMetadata={guestMetadata} />);
  return {
    events,
    dispose: () => window.removeEventListener(RESOURCE_METADATA_CHANGED_EVENT, onChanged),
  };
};

const openEditor = () => {
  fireEvent.click(screen.getByRole('button', { name: /Edit links/ }));
  return within(screen.getByRole('dialog', { name: 'Web links' }));
};

describe('WorkloadWebLinksAction', () => {
  let harness: ReturnType<typeof renderAction> | undefined;

  beforeEach(() => {
    updateMetadataMock.mockReset();
    updateMetadataMock.mockResolvedValue({});
    successMock.mockReset();
    readOnlyMock.mockReturnValue(false);
  });

  afterEach(() => {
    harness?.dispose();
    harness = undefined;
    cleanup();
  });

  it('is absent from read-only sessions', () => {
    readOnlyMock.mockReturnValue(true);
    harness = renderAction();

    expect(screen.queryByRole('button', { name: /Edit links/ })).not.toBeInTheDocument();
  });

  it('lists every guest in the view with its saved link and a labelled input', () => {
    harness = renderAction({ [authId]: { id: authId, customUrl: 'https://auth.example' } });
    const dialog = openEditor();

    expect(dialog.getByTestId('workload-web-links-summary')).toHaveTextContent(
      '1 of 3 in this view have a link',
    );
    expect(dialog.getByLabelText(/auth-service-01/)).toHaveValue('https://auth.example');
    expect(dialog.getByLabelText(/artifact-cache-01/)).toHaveValue('');

    fireEvent.click(dialog.getByRole('button', { name: 'Without a link' }));
    expect(dialog.queryByLabelText(/auth-service-01/)).not.toBeInTheDocument();
    expect(dialog.getByLabelText(/billing-worker-01/)).toBeInTheDocument();
  });

  it('blocks the whole save on an invalid link and explains it on that row', async () => {
    harness = renderAction();
    const dialog = openEditor();

    fireEvent.input(dialog.getByLabelText(/artifact-cache-01/), {
      target: { value: 'https://cache.example' },
    });
    const billingInput = dialog.getByLabelText(/billing-worker-01/);
    fireEvent.input(billingInput, { target: { value: 'billing admin' } });
    fireEvent.click(dialog.getByRole('button', { name: 'Save 2 links' }));

    await waitFor(() => expect(billingInput).toHaveAttribute('aria-invalid', 'true'));
    expect(billingInput).toHaveAccessibleDescription(
      'Enter a valid URL (for example: https://198.51.100.100:8080).',
    );
    expect(updateMetadataMock).not.toHaveBeenCalled();
  });

  it('saves additions and removals, notifies the table, and closes', async () => {
    harness = renderAction({ [authId]: { id: authId, customUrl: 'https://auth.example' } });
    const dialog = openEditor();

    fireEvent.input(dialog.getByLabelText(/auth-service-01/), { target: { value: '' } });
    fireEvent.input(dialog.getByLabelText(/artifact-cache-01/), {
      target: { value: ' https://cache.example ' },
    });
    expect(dialog.getByRole('status')).toHaveTextContent('2 unsaved changes');
    fireEvent.click(dialog.getByRole('button', { name: 'Save 2 links' }));

    await waitFor(() => expect(successMock).toHaveBeenCalledWith('Saved 2 links.'));
    expect(updateMetadataMock.mock.calls).toEqual([
      [cacheId, { customUrl: 'https://cache.example' }],
      [authId, { customUrl: '' }],
    ]);
    expect(harness.events).toEqual([
      { metadataKind: 'guest', metadataId: cacheId, customUrl: 'https://cache.example' },
      { metadataKind: 'guest', metadataId: authId, customUrl: '' },
    ]);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit links' })).toBeInTheDocument();
  });

  it('keeps the editor open with the failure on the row that did not save', async () => {
    updateMetadataMock.mockImplementation(async (id: string) => {
      if (id === billingId) throw new Error('Permission denied');
      return {};
    });
    harness = renderAction();
    const dialog = openEditor();

    fireEvent.input(dialog.getByLabelText(/artifact-cache-01/), {
      target: { value: 'https://cache.example' },
    });
    fireEvent.input(dialog.getByLabelText(/billing-worker-01/), {
      target: { value: 'https://billing.example' },
    });
    fireEvent.click(dialog.getByRole('button', { name: 'Save 2 links' }));

    await waitFor(() =>
      expect(dialog.getByLabelText(/billing-worker-01/)).toHaveAccessibleDescription(
        'Permission denied',
      ),
    );
    expect(successMock).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog', { name: 'Web links' })).toBeInTheDocument();
    expect(dialog.getByRole('status')).toHaveTextContent('1 unsaved change');
    expect(dialog.getByRole('button', { name: 'Save' })).toBeEnabled();
  });

  it('keeps unsaved drafts when the editor is closed and flags them on the trigger', () => {
    harness = renderAction();
    const dialog = openEditor();

    fireEvent.input(dialog.getByLabelText(/artifact-cache-01/), {
      target: { value: 'https://cache.example' },
    });
    fireEvent.click(dialog.getByRole('button', { name: 'Close web links' }));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Edit links/ })).toHaveTextContent('(unsaved)');

    const reopened = openEditor();
    expect(reopened.getByLabelText(/artifact-cache-01/)).toHaveValue('https://cache.example');
    fireEvent.click(reopened.getByRole('button', { name: 'Discard' }));
    expect(reopened.getByLabelText(/artifact-cache-01/)).toHaveValue('');
    expect(updateMetadataMock).not.toHaveBeenCalled();
  });
});
