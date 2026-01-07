/**
 * Feature Tips Store
 *
 * Manages feature discovery state:
 * - Which feature tips have been dismissed
 * - Provides functions to check and update discovery state
 */

import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  getAllFeatureTips,
  getFeatureTipsForLocation,
  type FeatureTip,
} from '@/content/features';

// Read dismissed tips from localStorage
const getInitialDismissedTips = (): Set<string> => {
  if (typeof window === 'undefined') return new Set();

  try {
    const stored = localStorage.getItem(STORAGE_KEYS.DISMISSED_FEATURE_TIPS);
    if (stored) {
      const parsed = JSON.parse(stored);
      if (Array.isArray(parsed)) {
        return new Set(parsed);
      }
    }
  } catch (_err) {
    // Ignore localStorage errors
  }

  return new Set();
};

// Create signals
const [dismissedTips, setDismissedTips] = createSignal<Set<string>>(getInitialDismissedTips());

/**
 * Check if a feature tip has been dismissed
 */
export function isTipDismissed(tipId: string): boolean {
  return dismissedTips().has(tipId);
}

/**
 * Dismiss a feature tip and persist to localStorage
 */
export function dismissTip(tipId: string): void {
  const current = dismissedTips();
  if (current.has(tipId)) return;

  const updated = new Set(current);
  updated.add(tipId);
  setDismissedTips(updated);

  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(STORAGE_KEYS.DISMISSED_FEATURE_TIPS, JSON.stringify([...updated]));
    } catch (err) {
      logger.warn('Failed to save dismissed tips', err);
    }
  }
}

/**
 * Reset all dismissed tips (useful for testing)
 */
export function resetDismissedTips(): void {
  setDismissedTips(new Set<string>());

  if (typeof window !== 'undefined') {
    try {
      localStorage.removeItem(STORAGE_KEYS.DISMISSED_FEATURE_TIPS);
    } catch (err) {
      logger.warn('Failed to reset dismissed tips', err);
    }
  }
}

/**
 * Get undismissed feature tips for a location
 */
export function getUndismissedTipsForLocation(location: FeatureTip['location']): FeatureTip[] {
  const tips = getFeatureTipsForLocation(location);
  const dismissed = dismissedTips();
  return tips.filter((tip) => !dismissed.has(tip.id));
}

/**
 * Get all undismissed feature tips
 */
export function getAllUndismissedTips(): FeatureTip[] {
  const tips = getAllFeatureTips();
  const dismissed = dismissedTips();
  return tips.filter((tip) => !dismissed.has(tip.id));
}

/**
 * Hook for components to use feature tips state
 */
export function useFeatureTips() {
  return {
    dismissedTips,
    isTipDismissed,
    dismissTip,
    resetDismissedTips,
    getUndismissedTipsForLocation,
    getAllUndismissedTips,
  };
}
