/**
 * Help Content Registry
 *
 * Central export point for all help content.
 * Use getHelpContent(id) to retrieve content by ID.
 */

import type { HelpContent, HelpContentId, HelpContentRegistry } from './types';
import { alertsHelpContent } from './alerts';
import { aiHelpContent } from './ai';
import { updatesHelpContent } from './updates';

// Combine all help content sources
const allContent: HelpContent[] = [
  ...alertsHelpContent,
  ...aiHelpContent,
  ...updatesHelpContent,
];

// Build registry for O(1) lookups
const registry: HelpContentRegistry = {};
for (const item of allContent) {
  if (registry[item.id]) {
    console.warn(`[HelpContent] Duplicate ID detected: ${item.id}`);
  }
  registry[item.id] = item;
}

/**
 * Get help content by ID
 * @param id - The help content ID (e.g., "alerts.thresholds.delay")
 * @returns The help content or undefined if not found
 */
export function getHelpContent(id: HelpContentId): HelpContent | undefined {
  return registry[id];
}

/**
 * Get all help content for a category
 * @param category - The category prefix (e.g., "alerts", "ai")
 * @returns Array of matching help content
 */
export function getHelpContentByCategory(category: string): HelpContent[] {
  const prefix = category.endsWith('.') ? category : `${category}.`;
  return allContent.filter((item) => item.id.startsWith(prefix));
}

/**
 * Get all help content
 * @returns Array of all registered help content
 */
export function getAllHelpContent(): HelpContent[] {
  return [...allContent];
}

/**
 * Check if help content exists for an ID
 * @param id - The help content ID
 * @returns true if content exists
 */
export function hasHelpContent(id: HelpContentId): boolean {
  return id in registry;
}

// Re-export types
export type { HelpContent, HelpContentId, HelpContentRegistry } from './types';
