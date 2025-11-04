// Generate consistent colors for tags based on their text
// This replicates Proxmox's tag color generation logic

/**
 * Simple hash function to generate a number from a string
 * This ensures the same tag always gets the same color
 */
function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash; // Convert to 32bit integer
  }
  return Math.abs(hash);
}

/**
 * Generate a color for a tag based on its text
 * Uses HSL to ensure good visibility and consistent saturation/lightness
 * (Internal helper - use getTagColorWithSpecial instead)
 */
function getTagColor(tag: string): { bg: string; text: string; border: string } {
  // Get a hash of the tag
  const hash = hashString(tag.toLowerCase());

  // Generate hue from hash (0-360 degrees)
  const hue = hash % 360;

  // Use moderate saturation for subtle but visible colors
  // These values are tuned to be noticeable without being distracting
  const saturation = 65; // Moderate saturation
  const lightnessBg = 60; // Slightly muted background
  const lightnessText = 25; // Dark text for contrast
  const lightnessBorder = 50; // Medium border

  // For dark mode, we'll adjust these in the component
  return {
    bg: `hsl(${hue}, ${saturation}%, ${lightnessBg}%)`,
    text: `hsl(${hue}, ${saturation}%, ${lightnessText}%)`,
    border: `hsl(${hue}, ${saturation}%, ${lightnessBorder}%)`,
  };
}

/**
 * Get tag colors adjusted for dark mode
 * (Internal helper - use getTagColorWithSpecial instead)
 */
function getTagColorDark(tag: string): { bg: string; text: string; border: string } {
  const hash = hashString(tag.toLowerCase());
  const hue = hash % 360;
  const saturation = 55; // Moderate saturation in dark mode

  return {
    bg: `hsl(${hue}, ${saturation}%, 35%)`, // Subtler background
    text: `hsl(${hue}, ${saturation}%, 85%)`, // Light text
    border: `hsl(${hue}, ${saturation}%, 45%)`, // Subtle border
  };
}

interface TagColorStyle {
  bg: string;
  text: string;
  border: string;
}

interface TagColorTheme {
  light: TagColorStyle;
  dark: TagColorStyle;
}

/**
 * Proxmox's default tag colors for special tags
 * These override the hash-based colors for specific tags
 */
const specialTagColors: Record<string, TagColorTheme> = {
  production: {
    light: { bg: 'rgb(254, 226, 226)', text: 'rgb(153, 27, 27)', border: 'rgb(239, 68, 68)' },
    dark: { bg: 'rgb(127, 29, 29)', text: 'rgb(254, 202, 202)', border: 'rgb(185, 28, 28)' },
  },
  staging: {
    light: { bg: 'rgb(254, 243, 199)', text: 'rgb(146, 64, 14)', border: 'rgb(245, 158, 11)' },
    dark: { bg: 'rgb(120, 53, 15)', text: 'rgb(253, 230, 138)', border: 'rgb(217, 119, 6)' },
  },
  development: {
    light: { bg: 'rgb(220, 252, 231)', text: 'rgb(22, 101, 52)', border: 'rgb(34, 197, 94)' },
    dark: { bg: 'rgb(20, 83, 45)', text: 'rgb(187, 247, 208)', border: 'rgb(34, 197, 94)' },
  },
  backup: {
    light: { bg: 'rgb(219, 234, 254)', text: 'rgb(30, 58, 138)', border: 'rgb(59, 130, 246)' },
    dark: { bg: 'rgb(30, 58, 138)', text: 'rgb(191, 219, 254)', border: 'rgb(59, 130, 246)' },
  },
};

/**
 * Get color for a tag, checking special colors first
 */
export function getTagColorWithSpecial(
  tag: string,
  isDarkMode: boolean,
): { bg: string; text: string; border: string } {
  const lowerTag = tag.toLowerCase();

  // Check if it's a special tag
  if (specialTagColors[lowerTag]) {
    return isDarkMode ? specialTagColors[lowerTag].dark : specialTagColors[lowerTag].light;
  }

  // Otherwise use hash-based color
  return isDarkMode ? getTagColorDark(tag) : getTagColor(tag);
}
