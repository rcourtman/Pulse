import { describe, expect, it } from 'vitest';

import {
  ASSISTANT_PANEL_DEFAULT_WIDTH,
  ASSISTANT_PANEL_MAX_WIDTH,
  ASSISTANT_PANEL_MIN_WIDTH,
  ASSISTANT_PANEL_WIDTH_STORAGE_KEY,
  clampAssistantPanelWidth,
  loadStoredAssistantPanelWidth,
  persistAssistantPanelWidth,
} from '../assistantPanelWidth';

describe('clampAssistantPanelWidth', () => {
  it('passes through an in-range width', () => {
    expect(clampAssistantPanelWidth(600, 1920)).toBe(600);
  });

  it('rounds fractional widths', () => {
    expect(clampAssistantPanelWidth(600.6, 1920)).toBe(601);
  });

  it('clamps below the minimum', () => {
    expect(clampAssistantPanelWidth(100, 1920)).toBe(ASSISTANT_PANEL_MIN_WIDTH);
  });

  it('clamps above the maximum on wide viewports', () => {
    expect(clampAssistantPanelWidth(5000, 3000)).toBe(ASSISTANT_PANEL_MAX_WIDTH);
  });

  it('keeps room for the rest of the app on narrow viewports', () => {
    expect(clampAssistantPanelWidth(900, 1000)).toBe(680);
  });

  it('never clamps below the minimum even when the viewport is tiny', () => {
    expect(clampAssistantPanelWidth(900, 600)).toBe(ASSISTANT_PANEL_MIN_WIDTH);
  });

  it('falls back to the default for non-finite input', () => {
    expect(clampAssistantPanelWidth(Number.NaN, 1920)).toBe(ASSISTANT_PANEL_DEFAULT_WIDTH);
    expect(clampAssistantPanelWidth(Number.POSITIVE_INFINITY, 1920)).toBe(
      ASSISTANT_PANEL_DEFAULT_WIDTH,
    );
  });
});

describe('loadStoredAssistantPanelWidth', () => {
  it('reads a stored integer', () => {
    expect(loadStoredAssistantPanelWidth({ getItem: () => '640' })).toBe(640);
  });

  it('returns null for missing or blank values', () => {
    expect(loadStoredAssistantPanelWidth({ getItem: () => null })).toBeNull();
    expect(loadStoredAssistantPanelWidth({ getItem: () => '  ' })).toBeNull();
  });

  it('returns null for garbage', () => {
    expect(loadStoredAssistantPanelWidth({ getItem: () => 'wide' })).toBeNull();
  });

  it('returns null when storage throws', () => {
    expect(
      loadStoredAssistantPanelWidth({
        getItem: () => {
          throw new Error('denied');
        },
      }),
    ).toBeNull();
  });
});

describe('persistAssistantPanelWidth', () => {
  it('writes the rounded width under the storage key', () => {
    const writes: Array<[string, string]> = [];
    persistAssistantPanelWidth({ setItem: (key, value) => writes.push([key, value]) }, 640.4);
    expect(writes).toEqual([[ASSISTANT_PANEL_WIDTH_STORAGE_KEY, '640']]);
  });

  it('swallows storage failures', () => {
    expect(() =>
      persistAssistantPanelWidth(
        {
          setItem: () => {
            throw new Error('quota');
          },
        },
        640,
      ),
    ).not.toThrow();
  });
});
