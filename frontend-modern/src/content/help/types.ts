/**
 * Help Content System Types
 *
 * Centralized help content for feature discoverability.
 * All help text lives in this directory for easy maintenance.
 */

/**
 * Unique identifier for help content items
 * Format: category.subcategory.item (e.g., "alerts.thresholds.delay")
 */
export type HelpContentId = string;

/**
 * Help content item - the core unit of help text
 */
export interface HelpContent {
  /** Unique identifier for this help item */
  id: HelpContentId;

  /** Short title displayed in popover header */
  title: string;

  /** Main help text - supports newlines for formatting */
  description: string;

  /** Optional: Example values or use cases */
  examples?: string[];

  /** Optional: Link to documentation */
  docUrl?: string;

  /** Optional: Related help content IDs for cross-referencing */
  related?: HelpContentId[];

  /** Version when this feature was added (for What's New tracking) */
  addedInVersion?: string;
}

/**
 * Help content registry - centralized lookup by ID
 */
export type HelpContentRegistry = Record<HelpContentId, HelpContent>;
