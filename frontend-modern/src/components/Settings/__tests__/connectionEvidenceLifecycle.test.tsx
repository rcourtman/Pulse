import { render, renderHook, waitFor } from '@solidjs/testing-library';
import { Show, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TrueNASAPI } from '@/api/truenas';
import { VMwareAPI } from '@/api/vmware';
import { notificationStore } from '@/stores/notifications';
import { TrueNASCredentialSlot } from '../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { VMwareCredentialSlot } from '../ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import { useTrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';
import { useVMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

vi.mock('@/api/truenas');
vi.mock('@/api/vmware');
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const deferred = () => {
  let resolve!: (value: never) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<never>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const preview = {
  current_count: 1,
  projected_count: 2,
  additional_count: 1,
  effect: 'adds_new',
  current_systems: [],
  projected_systems: [],
  current_system: null,
  projected_system: null,
};
const saved = {
  id: 'saved-alpha',
  name: 'Alpha',
  host: 'alpha.example.invalid',
  port: 443,
  username: 'fixture',
  password: '********',
  apiKey: '********',
  useHttps: true,
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: true,
  monitorDatastores: true,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
};
const failure = {
  status: 400,
  code: 'vmware_connection_failed',
  message: 'Obsolete endpoint failure',
  details: { category: 'tls', error: 'Obsolete TLS error' },
};

for (const platform of [
  { name: 'TrueNAS', api: TrueNASAPI, useState: useTrueNASSettingsPanelState },
  { name: 'VMware', api: VMwareAPI, useState: useVMwareSettingsPanelState },
]) {
  describe(`${platform.name} connection evidence lifecycle`, () => {
    beforeEach(() => {
      vi.resetAllMocks();
      vi.mocked(platform.api.listConnections).mockResolvedValue([]);
    });
    const setup = async () => {
      const hook = renderHook(() => platform.useState());
      await waitFor(() => expect(hook.result.loading()).toBe(false));
      hook.result.openCreateDialog();
      hook.result.updateForm({
        host: saved.host,
        username: 'fixture',
        password: 'fixture-secret',
      });
      return hook;
    };
    const feedbackCleared = (state: ReturnType<typeof platform.useState>) => {
      expect(state.monitoredSystemPreview()).toBeNull();
      expect(state.monitoredSystemPreviewError()).toBeNull();
      expect(state.monitoredSystemPreviewErrorTitle()).toBeNull();
      if ('connectionFailure' in state) {
        expect(state.connectionFailure()).toBeNull();
        expect(state.connectionTestWarning()).toBeNull();
      }
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(notificationStore.warning).not.toHaveBeenCalled();
    };

    it('rejects obsolete test success after an A-B-A endpoint edit', async () => {
      const { result } = await setup();
      const read = deferred();
      vi.mocked(platform.api.testConnection).mockReturnValueOnce(read.promise);
      const pending = result.testCurrentForm();
      result.updateForm({ host: 'beta.example.invalid' });
      result.updateForm({ host: saved.host });
      expect(result.testing()).toBe(true);
      read.resolve({ success: true } as never);
      expect(await pending).toBe(false);
      feedbackCleared(result);
      expect(result.testing()).toBe(false);
    });

    it('rejects obsolete impact after changing credentials', async () => {
      const { result } = await setup();
      const read = deferred();
      vi.mocked(platform.api.previewConnection).mockReturnValueOnce(read.promise);
      const pending = result.previewCurrentForm();
      result.updateForm({ password: 'different-fixture-secret' });
      read.resolve(preview as never);
      expect(await pending).toBeNull();
      feedbackCleared(result);
      expect(result.previewing()).toBe(false);
    });

    for (const patch of [
      { host: 'beta.example.invalid' },
      { port: '8443' },
      { username: 'other-fixture' },
      { password: 'other-fixture-secret' },
      { insecureSkipVerify: true },
      { enabled: false },
      ...(platform.name === 'TrueNAS'
        ? [
            { useHttps: false },
            { fingerprint: 'ab'.repeat(32) },
            { authMode: 'userpass' },
            { apiKey: 'different-fixture-key' },
            { monitorDatasets: false },
          ]
        : [{ monitorVms: false }]),
    ]) {
      it(`retires pending errors on ${Object.keys(patch)[0]} edits without overlapping reads`, async () => {
        const { result } = await setup();
        const read = deferred();
        vi.mocked(platform.api.testConnection).mockReturnValueOnce(read.promise);
        const pending = result.testCurrentForm();
        result.updateForm(patch as never);
        await result.testCurrentForm();
        await result.testSavedConnection(saved);
        await result.previewCurrentForm();
        await result.saveCurrentForm();
        expect(platform.api.testConnection).toHaveBeenCalledTimes(1);
        expect(platform.api.testSavedConnection).not.toHaveBeenCalled();
        expect(platform.api.previewConnection).not.toHaveBeenCalled();
        expect(platform.api.createConnection).not.toHaveBeenCalled();
        expect(result.testing()).toBe(true);
        read.reject(failure);
        expect(await pending).toBe(false);
        feedbackCleared(result);
      });
    }

    for (const change of ['close-reopen', 'edit-target', 'create-reset', 'dispose'] as const) {
      for (const outcome of ['success', 'generic-error', 'structured-error'] as const) {
        it(`retires preview ${outcome} on ${change} and keeps the pending owner busy`, async () => {
          const { result, cleanup } = await setup();
          const read = deferred();
          vi.mocked(platform.api.previewConnection).mockReturnValueOnce(read.promise);
          const pending = result.previewCurrentForm();
          if (change === 'dispose') cleanup();
          if (change === 'create-reset') result.openCreateDialog();
          if (change === 'edit-target') result.openEditDialog(saved);
          if (change === 'close-reopen') {
            result.closeDialog();
            expect(result.dialogOpen()).toBe(false);
            result.openCreateDialog();
          }
          expect(result.previewing()).toBe(true);
          await result.previewCurrentForm();
          await result.testCurrentForm();
          expect(platform.api.previewConnection).toHaveBeenCalledTimes(1);
          expect(platform.api.previewSavedConnection).not.toHaveBeenCalled();
          expect(platform.api.testConnection).not.toHaveBeenCalled();
          if (outcome === 'success') read.resolve(preview as never);
          else
            read.reject(
              outcome === 'generic-error'
                ? failure
                : {
                    code: 'monitored_system_usage_unavailable',
                    details: { reason: 'supplemental_inventory_unsettled' },
                  },
            );
          expect(await pending).toBeNull();
          feedbackCleared(result);
          expect(result.previewing()).toBe(false);
        });
      }
    }

    it('retires form tests on editor departure, including degraded VMware results', async () => {
      const { result } = await setup();
      const read = deferred();
      vi.mocked(platform.api.testConnection).mockReturnValueOnce(read.promise);
      const pending = result.testCurrentForm();
      result.retireFormEvidence();
      read.resolve({
        success: true,
        degraded: true,
        issueCount: 1,
        issues: [{ message: 'Old optional read' }],
      } as never);
      expect(await pending).toBe(false);
      feedbackCleared(result);
    });

    it('does not revive saved-test success on a new target or reload from that obsolete test', async () => {
      const { result } = await setup();
      const read = deferred();
      vi.mocked(platform.api.testSavedConnection).mockReturnValueOnce(read.promise);
      const pending = result.testSavedConnection(saved);
      result.openEditDialog({ ...saved, id: 'saved-beta', host: 'beta.example.invalid' });
      read.resolve({ success: true } as never);
      await pending;
      feedbackCleared(result);
      expect(platform.api.listConnections).toHaveBeenCalledTimes(1);
      expect(result.testing()).toBe(false);
    });

    it('accepts fresh results after obsolete work settles and clears completed impact on edits', async () => {
      const { result } = await setup();
      vi.mocked(platform.api.previewConnection).mockResolvedValue(preview as never);
      expect(await result.previewCurrentForm()).toEqual(preview);
      expect(result.monitoredSystemPreview()).toEqual(preview);
      result.updateForm({ host: 'beta.example.invalid' });
      expect(result.monitoredSystemPreview()).toBeNull();
      vi.mocked(platform.api.testConnection).mockResolvedValue({ success: true } as never);
      expect(await result.testCurrentForm()).toBe(true);
      expect(notificationStore.success).toHaveBeenCalledWith(
        `${platform.name} connection successful`,
      );
      expect(platform.api.testConnection).toHaveBeenCalledWith(
        expect.objectContaining({
          host: 'beta.example.invalid',
          insecureSkipVerify: false,
        }),
      );
    });

    it('retires a departing credential slot even while Settings keeps its hook alive', async () => {
      const read = deferred();
      vi.mocked(platform.api.previewConnection).mockReturnValueOnce(read.promise);
      const [show, setShow] = createSignal(true);
      let state!: ReturnType<typeof platform.useState>;
      const view = render(() => {
        if (platform.name === 'TrueNAS') {
          const nas = useTrueNASSettingsPanelState();
          state = nas;
          return (
            <Show when={show()}>
              <TrueNASCredentialSlot state={nas} onCancel={() => {}} onSaved={() => {}} />
            </Show>
          );
        }
        const vmware = useVMwareSettingsPanelState();
        state = vmware;
        return (
          <Show when={show()}>
            <VMwareCredentialSlot state={vmware} onCancel={() => {}} onSaved={() => {}} />
          </Show>
        );
      });
      await waitFor(() => expect(state.loading()).toBe(false));
      state.updateForm({ host: saved.host, username: 'fixture' });
      const pending = state.previewCurrentForm();
      setShow(false);
      expect(state.previewing()).toBe(true);
      read.resolve(preview as never);
      expect(await pending).toBeNull();
      feedbackCleared(state);
      view.unmount();
    });
  });
}
