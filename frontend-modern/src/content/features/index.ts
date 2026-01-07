/**
 * Feature Discovery Registry
 *
 * Central export point for feature tips and What's New content.
 */

import type { FeatureTip } from './types';

// Feature tips for discoverable features
const featureTips: FeatureTip[] = [
  {
    id: 'alert-delay-thresholds',
    title: 'Alert Delay Thresholds',
    description:
      'Click the clock icon to set how long a threshold must be exceeded before an alert fires. Prevents alerts from brief spikes.',
    location: 'alerts',
    addedInVersion: 'v5.0.0',
    action: {
      label: 'Go to Alerts',
      path: '/alerts',
    },
    priority: 10,
  },
  {
    id: 'custom-ai-endpoints',
    title: 'Custom AI Endpoints',
    description:
      'Use OpenRouter, vLLM, or any OpenAI-compatible API. Expand the OpenAI section in AI Settings and enter your provider\'s base URL.',
    location: 'settings',
    addedInVersion: 'v4.5.0',
    action: {
      label: 'Configure AI',
      path: '/settings',
    },
    priority: 8,
  },
  {
    id: 'container-update-detection',
    title: 'Container Update Detection',
    description:
      'Pulse checks for container image updates automatically. Look for the blue badge on containers with available updates.',
    location: 'docker',
    addedInVersion: 'v5.0.11',
    action: {
      label: 'View Containers',
      path: '/docker',
    },
    priority: 7,
  },
  {
    id: 'sparkline-metrics',
    title: 'Sparkline Metrics',
    description:
      'Toggle between bar charts and sparklines for CPU/Memory metrics. Click the chart icon in the column header.',
    location: 'dashboard',
    addedInVersion: 'v4.0.0',
    priority: 5,
  },
  {
    id: 'column-customization',
    title: 'Customize Columns',
    description:
      'Click "Columns" to show/hide table columns. Your preferences are saved automatically.',
    location: 'dashboard',
    addedInVersion: 'v4.0.0',
    priority: 4,
  },
];

/**
 * Get all feature tips
 */
export function getAllFeatureTips(): FeatureTip[] {
  return [...featureTips].sort((a, b) => (b.priority || 0) - (a.priority || 0));
}

/**
 * Get feature tips for a specific location
 */
export function getFeatureTipsForLocation(location: FeatureTip['location']): FeatureTip[] {
  return featureTips
    .filter((tip) => tip.location === location || tip.location === 'global')
    .sort((a, b) => (b.priority || 0) - (a.priority || 0));
}

/**
 * Get a specific feature tip by ID
 */
export function getFeatureTip(id: string): FeatureTip | undefined {
  return featureTips.find((tip) => tip.id === id);
}

// Re-export types
export type { FeatureTip } from './types';
