const SNOOZE_MS = 7 * 24 * 60 * 60 * 1000;

export function isUpsellSnoozed(key: string): boolean {
  if (typeof window === 'undefined') return false;
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return false;
    const expiresAt = Number(raw);
    if (!Number.isFinite(expiresAt)) return false;
    return Date.now() < expiresAt;
  } catch {
    return false;
  }
}

export function snoozeUpsell(key: string): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(key, String(Date.now() + SNOOZE_MS));
  } catch {
    // Ignore storage quota / privacy-mode failures.
  }
}
