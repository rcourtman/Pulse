/**
 * Generate a consistent color for a node based on its name
 * Returns a color that's visually distinct but not too bright
 */

// Predefined palette of colors that work well in light and dark mode
const colorPalette = [
  '#3B82F6', // blue
  '#10B981', // emerald
  '#F59E0B', // amber
  '#EF4444', // red
  '#8B5CF6', // violet
  '#EC4899', // pink
  '#14B8A6', // teal
  '#F97316', // orange
  '#06B6D4', // cyan
  '#84CC16', // lime
  '#A855F7', // purple
  '#F43F5E', // rose
  '#22D3EE', // sky
  '#FCD34D', // yellow
  '#6366F1', // indigo
];

/**
 * Simple hash function to convert string to number
 */
function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash; // Convert to 32-bit integer
  }
  return Math.abs(hash);
}

/**
 * Get a consistent color for a node based on its name
 */
export function getNodeColor(nodeName: string): string {
  const hash = hashString(nodeName);
  const index = hash % colorPalette.length;
  return colorPalette[index];
}

/**
 * Get a lighter version of the color for backgrounds
 */
export function getNodeColorLight(nodeName: string): string {
  const color = getNodeColor(nodeName);
  // Return color with 20% opacity for subtle backgrounds
  return `${color}20`;
}