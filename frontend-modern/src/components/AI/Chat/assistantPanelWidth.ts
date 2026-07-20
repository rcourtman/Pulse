// Width state for the Assistant side panel's drag-to-resize handle.
//
// The panel historically rendered at a fixed 560px, which both wasted space
// on wide screens and squeezed the composer chrome into overflow glitches.
// The width is user-adjustable within bounds and persists across sessions.

export const ASSISTANT_PANEL_DEFAULT_WIDTH = 560;
export const ASSISTANT_PANEL_MIN_WIDTH = 420;
export const ASSISTANT_PANEL_MAX_WIDTH = 960;
export const ASSISTANT_PANEL_WIDTH_STORAGE_KEY = 'pulse.assistant.panelWidth';

// Keep the rest of the app usable while the panel is docked: never let the
// panel eat the viewport down to less than this remainder.
const MIN_REMAINING_VIEWPORT = 320;

export function clampAssistantPanelWidth(value: number, viewportWidth: number): number {
  if (!Number.isFinite(value)) return ASSISTANT_PANEL_DEFAULT_WIDTH;
  const viewportCap = Math.max(ASSISTANT_PANEL_MIN_WIDTH, viewportWidth - MIN_REMAINING_VIEWPORT);
  const upperBound = Math.min(ASSISTANT_PANEL_MAX_WIDTH, viewportCap);
  return Math.min(upperBound, Math.max(ASSISTANT_PANEL_MIN_WIDTH, Math.round(value)));
}

export function loadStoredAssistantPanelWidth(storage: Pick<Storage, 'getItem'>): number | null {
  let raw: string | null = null;
  try {
    raw = storage.getItem(ASSISTANT_PANEL_WIDTH_STORAGE_KEY);
  } catch {
    return null;
  }
  if (raw === null || raw.trim() === '') return null;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? parsed : null;
}

export function persistAssistantPanelWidth(storage: Pick<Storage, 'setItem'>, width: number): void {
  try {
    storage.setItem(ASSISTANT_PANEL_WIDTH_STORAGE_KEY, String(Math.round(width)));
  } catch {
    // Storage may be unavailable (private mode, quota); resizing still works
    // for the session.
  }
}
