import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Show, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { dialogStackHasBlockingDialog } from '@/components/shared/useDialogState';
import { aiChatStore } from '@/stores/aiChat';
import { MobileAlertHistoryInvestigationDialog } from '../MobileAlertHistoryInvestigationDialog';

const incident = {
  id: 'incident-1',
  alertIdentifier: 'alert-1',
  alertType: 'backup',
  level: 'warning',
  resourceId: 'resource-1',
  resourceName: 'Backup',
  resourceType: 'storage',
  status: 'resolved',
  openedAt: '2026-09-07T12:00:00Z',
  closedAt: '2026-09-07T12:10:00Z',
  events: [],
};

describe('mobile incident Assistant transition', () => {
  afterEach(() => {
    cleanup();
    aiChatStore.close();
    aiChatStore.clearAllContext();
    aiChatStore.setEnabled(false);
    vi.restoreAllMocks();
  });
  for (const kind of ['timeline', 'resource'] as const) {
    it(`dismisses the ${kind} drawer after preserving its incident context`, async () => {
      aiChatStore.setEnabled(true);
      const nativeOpen = aiChatStore.open;
      const openAssistant = vi.spyOn(aiChatStore, 'open').mockImplementation((context) => {
        expect(dialogStackHasBlockingDialog()).toBe(false);
        nativeOpen(context);
      });
      const [open, setOpen] = createSignal(true);
      const state = {
        incidentLoading: () => ({}),
        incidentErrors: () => ({}),
        incidentTimelines: () => ({ row: incident }),
        historyIncidentEventFilters: () => new Set(),
        incidentNoteDrafts: () => ({}),
        incidentNoteSaving: () => new Set(),
        resourceIncidentPanel: () => ({
          resourceId: 'resource-1',
          resourceName: 'Backup',
          rowKey: 'row',
        }),
        resourceIncidents: () => ({ 'resource-1': [incident] }),
        resourceIncidentLoading: () => ({}),
        resourceIncidentError: () => ({}),
        expandedResourceIncidentIds: () => new Set(),
        resourceIncidentEventFilters: () => new Set(),
      } as any;
      render(() => (
        <Show when={open()}>
          <MobileAlertHistoryInvestigationDialog
            investigation={{ kind, rowKey: 'row', alert: { resourceName: 'Backup' } as any }}
            state={state}
            onClose={() => setOpen(false)}
          />
        </Show>
      ));
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole('button', { name: 'Discuss incident incident-1 with Pulse Assistant' }),
      );
      expect(screen.queryByRole('dialog')).toBeNull();
      await waitFor(() => expect(openAssistant).toHaveBeenCalledTimes(1));
      expect(aiChatStore.isOpen).toBe(true);
      expect(openAssistant).toHaveBeenCalledWith(
        expect.objectContaining({
          targetId: 'resource-1',
          autonomousMode: false,
          context: expect.objectContaining({
            alertIncidentId: 'incident-1',
            alertStatus: 'resolved',
          }),
        }),
      );
    });
  }
});
