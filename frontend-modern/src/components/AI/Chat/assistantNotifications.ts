import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';

export type AssistantAttentionKind = 'done' | 'error' | 'question' | 'approval';

const STORAGE_KEY = 'pulse:ai_chat_attention_notifications';

// Generic, fixed copy only. OS notification centers persist and sync text, so
// answers, commands, and resource names must never travel through them.
const ATTENTION_BODY: Record<AssistantAttentionKind, string> = {
  done: 'The Assistant finished its response.',
  error: 'The Assistant ran into an error.',
  question: 'The Assistant has a question for you.',
  approval: 'An action is waiting for your approval.',
};

const readStoredPreference = (): boolean => {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'true';
  } catch {
    return false;
  }
};

const [enabled, setEnabled] = createSignal(readStoredPreference());

export const assistantNotificationsSupported = (): boolean =>
  typeof window !== 'undefined' && 'Notification' in window;

export const assistantNotificationsEnabled = enabled;

const persistPreference = (value: boolean) => {
  try {
    localStorage.setItem(STORAGE_KEY, value ? 'true' : 'false');
  } catch {
    // Preference simply won't survive a reload.
  }
};

// Enabling asks the browser for permission (must run in a user gesture).
// Returns the resulting enabled state so callers can reflect a denial.
export const setAssistantNotificationsEnabled = async (value: boolean): Promise<boolean> => {
  if (!value) {
    setEnabled(false);
    persistPreference(false);
    return false;
  }
  if (!assistantNotificationsSupported()) return false;

  let permission = Notification.permission;
  if (permission === 'default') {
    try {
      permission = await Notification.requestPermission();
    } catch (error) {
      logger.debug('[AssistantNotifications] Permission request failed', { error });
      permission = 'denied';
    }
  }
  const granted = permission === 'granted';
  setEnabled(granted);
  persistPreference(granted);
  return granted;
};

// Notify only when the user is away: tab hidden or window unfocused. A
// notification for a visible chat is noise.
const userIsAway = (): boolean => {
  if (typeof document === 'undefined') return false;
  if (document.hidden) return true;
  return typeof document.hasFocus === 'function' ? !document.hasFocus() : false;
};

export const notifyAssistantAttention = (kind: AssistantAttentionKind): void => {
  if (!enabled()) return;
  if (!assistantNotificationsSupported() || Notification.permission !== 'granted') return;
  if (!userIsAway()) return;

  try {
    const notification = new Notification('Pulse Assistant', {
      body: ATTENTION_BODY[kind],
      // One slot per kind: a burst of events replaces instead of stacking.
      tag: `pulse-assistant-${kind}`,
    });
    notification.onclick = () => {
      window.focus();
      notification.close();
    };
  } catch (error) {
    logger.debug('[AssistantNotifications] Failed to show notification', { error });
  }
};
