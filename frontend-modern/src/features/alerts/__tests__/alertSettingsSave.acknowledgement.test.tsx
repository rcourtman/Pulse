import { cleanup, renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { useAlertsConfigurationState } from '../useAlertsConfigurationState';
import type { AlertTab } from '../types';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getConfig: vi.fn(),
    updateConfig: vi.fn(),
    getDeadManConfig: vi.fn(),
    updateDeadManConfig: vi.fn(),
  },
}));
vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getEmailConfig: vi.fn(),
    updateEmailConfig: vi.fn(),
    getAppriseConfig: vi.fn(),
    updateAppriseConfig: vi.fn(),
    getWebhooks: vi.fn(),
  },
}));
vi.mock('@/stores/license', () => ({ hasFeature: () => false }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

async function editor() {
  const hook = renderHook(() => {
    const [activeTab, setActiveTab] = createSignal<AlertTab>('destinations');
    const [dirty, setDirty] = createSignal(false);
    const state = useAlertsConfigurationState({
      activeTab,
      allResources: () => [],
      byType: () => [],
      children: () => [],
      activeAlerts: {},
      removeAlerts: vi.fn(),
      setOverviewOverrides: vi.fn(),
      hasUnsavedChanges: dirty,
      setHasUnsavedChanges: setDirty,
      alertsActivationState: () => 'active',
      alertsActivationConfig: () => ({ enabled: true }),
    });
    return { ...state, dirty, setActiveTab };
  });
  await waitFor(() => {
    expect(AlertsAPI.getConfig).toHaveBeenCalledTimes(1);
    expect(hook.result.isReloadingConfig()).toBe(false);
  });
  vi.mocked(notificationStore.success).mockClear();
  return hook;
}

function editEmail(state: Awaited<ReturnType<typeof editor>>['result'], server: string) {
  state.setEmailConfig({ ...state.emailConfig(), server });
  state.guardedSetHasUnsavedChanges(true);
}

describe('alert settings save acknowledgement', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(AlertsAPI.getConfig).mockResolvedValue({ overrides: {} } as never);
    vi.mocked(AlertsAPI.updateConfig).mockResolvedValue({ success: true });
    vi.mocked(AlertsAPI.getDeadManConfig).mockResolvedValue({ pingUrl: '', configured: false });
    vi.mocked(AlertsAPI.updateDeadManConfig).mockResolvedValue({ success: true, configured: true });
    vi.mocked(NotificationsAPI.getEmailConfig).mockResolvedValue({
      enabled: true,
      server: 'smtp.saved.example.test',
      to: ['ops@example.test'],
    } as never);
    vi.mocked(NotificationsAPI.updateEmailConfig).mockResolvedValue({ success: true });
    vi.mocked(NotificationsAPI.getAppriseConfig).mockResolvedValue({
      enabled: true,
      mode: 'http',
      serverUrl: 'https://apprise.saved.example.test',
      apiKey: '',
      hasApiKey: true,
    } as never);
    vi.mocked(NotificationsAPI.updateAppriseConfig).mockImplementation(async (config) => ({
      ...config,
      apiKey: '',
      hasApiKey: true,
    }));
    vi.mocked(NotificationsAPI.getWebhooks).mockResolvedValue([]);
  });

  it('saves one coherent click-time snapshot, not later destination edits', async () => {
    const { result } = await editor();
    const pending = deferred<{ success: boolean }>();
    vi.mocked(AlertsAPI.updateConfig).mockReturnValueOnce(pending.promise);
    editEmail(result, 'smtp.first.example.test');
    result.setAppriseConfig({
      ...result.appriseConfig(),
      serverUrl: 'https://apprise.first.example.test',
    });
    result.setDeadManPingUrl('https://watchdog.example.test/first');
    const save = result.saveAlertConfiguration();

    editEmail(result, 'smtp.newer.example.test');
    result.setAppriseConfig({
      ...result.appriseConfig(),
      serverUrl: 'https://apprise.newer.example.test',
    });
    result.setDeadManPingUrl('https://watchdog.example.test/newer');
    pending.resolve({ success: true });
    await save;

    expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledWith(
      expect.objectContaining({ server: 'smtp.first.example.test' }),
    );
    expect(NotificationsAPI.updateAppriseConfig).toHaveBeenCalledWith(
      expect.objectContaining({ serverUrl: 'https://apprise.first.example.test' }),
    );
    expect(AlertsAPI.updateDeadManConfig).toHaveBeenCalledWith(
      'https://watchdog.example.test/first',
    );
    expect(result.emailConfig().server).toBe('smtp.newer.example.test');
    expect(result.appriseConfig().serverUrl).toBe('https://apprise.newer.example.test');
    expect(result.deadManPingUrl()).toBe('https://watchdog.example.test/newer');
    expect(result.dirty()).toBe(true);
  });

  it('refuses default writes after a failed policy read until saved settings are loaded', async () => {
    vi.mocked(AlertsAPI.getConfig).mockRejectedValueOnce(
      new Error('Synthetic policy read failure'),
    );
    const { result } = await editor();
    expect(result.isConfigLoaded()).toBe(false);
    expect(result.configLoadError()).toContain('Saved alert settings could not be loaded');
    editEmail(result, 'smtp.not-loaded.example.test');
    await result.saveAlertConfiguration();
    expect(AlertsAPI.updateConfig).not.toHaveBeenCalled();
    expect(NotificationsAPI.updateEmailConfig).not.toHaveBeenCalled();
    expect(NotificationsAPI.updateAppriseConfig).not.toHaveBeenCalled();
    expect(AlertsAPI.updateDeadManConfig).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();

    await result.loadAlertConfiguration();
    expect(result.isConfigLoaded()).toBe(true);
    expect(result.configLoadError()).toBeNull();
    editEmail(result, 'smtp.loaded.example.test');
    await result.saveAlertConfiguration();
    expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledWith(
      expect.objectContaining({ server: 'smtp.loaded.example.test' }),
    );
    expect(result.dirty()).toBe(false);
  });

  it('withdraws save admission while replacing a previously loaded context', async () => {
    const { result } = await editor();
    const pending = deferred<Awaited<ReturnType<typeof AlertsAPI.getConfig>>>();
    vi.mocked(AlertsAPI.getConfig).mockReturnValueOnce(pending.promise);
    const reload = result.loadAlertConfiguration();
    expect(result.isConfigLoaded()).toBe(false);
    await result.saveAlertConfiguration();
    expect(AlertsAPI.updateConfig).not.toHaveBeenCalled();
    pending.resolve({ overrides: {} } as never);
    await reload;
    expect(result.isConfigLoaded()).toBe(true);
  });

  it("does not reuse the old context's write admission when its replacement read fails", async () => {
    const { result } = await editor();
    vi.mocked(AlertsAPI.getConfig).mockRejectedValueOnce(
      new Error('Synthetic replacement failure'),
    );
    eventBus.emit('org_switched', 'synthetic-unavailable-org');
    await waitFor(() => {
      expect(AlertsAPI.getConfig).toHaveBeenCalledTimes(2);
      expect(result.isReloadingConfig()).toBe(false);
    });
    expect(result.isConfigLoaded()).toBe(false);
    editEmail(result, 'smtp.stale-context.example.test');
    await result.saveAlertConfiguration();
    expect(AlertsAPI.updateConfig).not.toHaveBeenCalled();
    expect(NotificationsAPI.updateEmailConfig).not.toHaveBeenCalled();
  });

  it('does not let an older failed read withdraw a newer loaded context', async () => {
    const { result } = await editor();
    const pending = deferred<Awaited<ReturnType<typeof AlertsAPI.getConfig>>>();
    vi.mocked(AlertsAPI.getConfig).mockReturnValueOnce(pending.promise);
    const superseded = result.loadAlertConfiguration();
    await result.loadAlertConfiguration();
    expect(result.isConfigLoaded()).toBe(true);
    pending.reject(new Error('Synthetic superseded read failure'));
    await superseded;
    expect(result.isConfigLoaded()).toBe(true);
    expect(result.configLoadError()).toBeNull();
  });

  it('keeps edits made during a destination write dirty after acknowledgement', async () => {
    const { result } = await editor();
    const pending = deferred<{ success: boolean }>();
    vi.mocked(NotificationsAPI.updateEmailConfig).mockReturnValueOnce(pending.promise);
    editEmail(result, 'smtp.first.example.test');
    const save = result.saveAlertConfiguration();
    await waitFor(() => expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledTimes(1));
    editEmail(result, 'smtp.newer.example.test');
    pending.resolve({ success: true });
    await save;

    expect(result.dirty()).toBe(true);
    expect(result.emailConfig().server).toBe('smtp.newer.example.test');
    expect(notificationStore.success).toHaveBeenLastCalledWith(
      expect.stringContaining('still unsaved'),
    );
    await result.saveAlertConfiguration();
    expect(NotificationsAPI.updateEmailConfig).toHaveBeenLastCalledWith(
      expect.objectContaining({ server: 'smtp.newer.example.test' }),
    );
    expect(result.dirty()).toBe(false);
  });

  it('does not replace a newer Apprise draft with the older saved response', async () => {
    const { result } = await editor();
    const pending = deferred<Awaited<ReturnType<typeof NotificationsAPI.updateAppriseConfig>>>();
    vi.mocked(NotificationsAPI.updateAppriseConfig).mockReturnValueOnce(pending.promise);
    result.setAppriseConfig({ ...result.appriseConfig(), configKey: 'first' });
    result.guardedSetHasUnsavedChanges(true);
    const save = result.saveAlertConfiguration();
    await waitFor(() => expect(NotificationsAPI.updateAppriseConfig).toHaveBeenCalledTimes(1));
    result.setAppriseConfig({
      ...result.appriseConfig(),
      configKey: 'newer',
      apiKey: 'synthetic-new-draft',
    });
    result.guardedSetHasUnsavedChanges(true);
    pending.resolve({
      enabled: true,
      mode: 'http',
      configKey: 'first',
      apiKey: '',
      hasApiKey: true,
    } as never);
    await save;

    expect(result.appriseConfig().configKey).toBe('newer');
    expect(result.appriseConfig().apiKey).toBe('synthetic-new-draft');
    expect(result.dirty()).toBe(true);
  });

  it('admits only one save until the preceding writes complete', async () => {
    const { result } = await editor();
    const pending = deferred<{ success: boolean }>();
    vi.mocked(AlertsAPI.updateConfig).mockReturnValue(pending.promise);
    editEmail(result, 'smtp.first.example.test');
    const first = result.saveAlertConfiguration();
    const duplicate = result.saveAlertConfiguration();
    pending.resolve({ success: true });
    await Promise.all([first, duplicate]);

    expect(AlertsAPI.updateConfig).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.updateAppriseConfig).toHaveBeenCalledTimes(1);
    expect(AlertsAPI.updateDeadManConfig).toHaveBeenCalledTimes(1);
  });

  it('retains unsaved destination drafts when switching settings tabs', async () => {
    const { result } = await editor();
    editEmail(result, 'smtp.draft.example.test');
    result.setActiveTab('schedule');
    result.setActiveTab('destinations');
    await Promise.resolve();
    await Promise.resolve();

    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(1);
    expect(result.emailConfig().server).toBe('smtp.draft.example.test');
    expect(result.dirty()).toBe(true);
  });

  it('retains drafts on a failed write and admits a deliberate retry', async () => {
    const { result } = await editor();
    editEmail(result, 'smtp.retry.example.test');
    vi.mocked(NotificationsAPI.updateAppriseConfig).mockRejectedValueOnce(
      new Error('Synthetic save failure'),
    );
    await expect(result.saveAlertConfiguration()).rejects.toThrow('Synthetic save failure');
    expect(result.dirty()).toBe(true);
    expect(result.emailConfig().server).toBe('smtp.retry.example.test');
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(AlertsAPI.updateDeadManConfig).not.toHaveBeenCalled();

    await result.saveAlertConfiguration();
    expect(result.dirty()).toBe(false);
    expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledTimes(2);
  });

  it('only reports a completed save when every requested write succeeded', async () => {
    const { result } = await editor();
    editEmail(result, 'smtp.accepted.example.test');
    result.setAppriseConfig({ ...result.appriseConfig(), apiKey: 'synthetic-replacement' });
    result.guardedSetHasUnsavedChanges(true);
    await result.saveAlertConfiguration();

    expect(result.dirty()).toBe(false);
    expect(result.appriseConfig().apiKey).toBe('');
    expect(result.appriseConfig().hasApiKey).toBe(true);
    expect(notificationStore.success).toHaveBeenCalledTimes(1);
  });

  it('stops unsent destination writes after the organisation changes', async () => {
    const { result } = await editor();
    const pending = deferred<{ success: boolean }>();
    vi.mocked(AlertsAPI.updateConfig).mockReturnValueOnce(pending.promise);
    editEmail(result, 'smtp.previous-org.example.test');
    const save = result.saveAlertConfiguration();
    eventBus.emit('org_switched', 'synthetic-other-org');
    await waitFor(() => expect(AlertsAPI.getConfig).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.isReloadingConfig()).toBe(false));
    pending.resolve({ success: true });
    await save;

    expect(NotificationsAPI.updateEmailConfig).not.toHaveBeenCalled();
    expect(NotificationsAPI.updateAppriseConfig).not.toHaveBeenCalled();
    expect(AlertsAPI.updateDeadManConfig).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(result.emailConfig().server).toBe('smtp.saved.example.test');
  });

  it('stops later writes when the organisation changes during the email request', async () => {
    const { result } = await editor();
    const pending = deferred<{ success: boolean }>();
    vi.mocked(NotificationsAPI.updateEmailConfig).mockReturnValueOnce(pending.promise);
    editEmail(result, 'smtp.previous-org.example.test');
    const save = result.saveAlertConfiguration();
    await waitFor(() => expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledTimes(1));
    eventBus.emit('org_switched', 'synthetic-other-org');
    await waitFor(() => expect(AlertsAPI.getConfig).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.isReloadingConfig()).toBe(false));
    pending.resolve({ success: true });
    await save;

    expect(NotificationsAPI.updateAppriseConfig).not.toHaveBeenCalled();
    expect(AlertsAPI.updateDeadManConfig).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(result.emailConfig().server).toBe('smtp.saved.example.test');
  });

  it('does not advance an unfinished save after the editor unmounts', async () => {
    const { result } = await editor();
    const pending = deferred<{ success: boolean }>();
    vi.mocked(AlertsAPI.updateConfig).mockReturnValueOnce(pending.promise);
    editEmail(result, 'smtp.unmounted.example.test');
    const save = result.saveAlertConfiguration();
    cleanup();
    pending.resolve({ success: true });
    await save;

    expect(NotificationsAPI.updateEmailConfig).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
  });
});
