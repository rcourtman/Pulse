import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const installNotificationMock = (permission: NotificationPermission) => {
  const instances: Array<{ title: string; options?: NotificationOptions }> = [];
  class NotificationMock {
    static permission: NotificationPermission = permission;
    static requestPermission = vi.fn(async () => NotificationMock.permission);
    onclick: (() => void) | null = null;
    close = vi.fn();
    constructor(title: string, options?: NotificationOptions) {
      instances.push({ title, options });
    }
  }
  vi.stubGlobal('Notification', NotificationMock);
  return { NotificationMock, instances };
};

const setDocumentHidden = (hidden: boolean) => {
  Object.defineProperty(document, 'hidden', { value: hidden, configurable: true });
};

const loadModule = async () => {
  vi.resetModules();
  return import('../assistantNotifications');
};

describe('assistantNotifications', () => {
  beforeEach(() => {
    localStorage.clear();
    setDocumentHidden(true);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    setDocumentHidden(false);
  });

  it('stays silent until the user opts in', async () => {
    const { instances } = installNotificationMock('granted');
    const mod = await loadModule();

    mod.notifyAssistantAttention('done');
    expect(instances).toHaveLength(0);
  });

  it('requests permission on enable and notifies while the tab is hidden', async () => {
    const { NotificationMock, instances } = installNotificationMock('default');
    NotificationMock.requestPermission = vi.fn(async () => {
      NotificationMock.permission = 'granted';
      return 'granted' as NotificationPermission;
    });
    const mod = await loadModule();

    await expect(mod.setAssistantNotificationsEnabled(true)).resolves.toBe(true);
    expect(NotificationMock.requestPermission).toHaveBeenCalledOnce();

    mod.notifyAssistantAttention('question');
    expect(instances).toHaveLength(1);
    expect(instances[0].title).toBe('Pulse Assistant');
    expect(instances[0].options?.body).toBe('The Assistant has a question for you.');
    // Generic copy only: no answer text, commands, or resource names.
    expect(instances[0].options?.tag).toBe('pulse-assistant-question');
  });

  it('reports a denied permission as not enabled', async () => {
    const { NotificationMock } = installNotificationMock('default');
    NotificationMock.requestPermission = vi.fn(async () => 'denied' as NotificationPermission);
    const mod = await loadModule();

    await expect(mod.setAssistantNotificationsEnabled(true)).resolves.toBe(false);
    expect(mod.assistantNotificationsEnabled()).toBe(false);
  });

  it('does not notify while the tab is visible and focused', async () => {
    const { instances } = installNotificationMock('granted');
    const mod = await loadModule();
    await mod.setAssistantNotificationsEnabled(true);

    setDocumentHidden(false);
    const hasFocus = vi.spyOn(document, 'hasFocus').mockReturnValue(true);
    mod.notifyAssistantAttention('done');
    expect(instances).toHaveLength(0);
    hasFocus.mockRestore();
  });

  it('persists the preference across module reloads', async () => {
    const { NotificationMock } = installNotificationMock('granted');
    NotificationMock.requestPermission = vi.fn(async () => 'granted' as NotificationPermission);
    let mod = await loadModule();
    await mod.setAssistantNotificationsEnabled(true);

    mod = await loadModule();
    expect(mod.assistantNotificationsEnabled()).toBe(true);

    await mod.setAssistantNotificationsEnabled(false);
    mod = await loadModule();
    expect(mod.assistantNotificationsEnabled()).toBe(false);
  });
});
