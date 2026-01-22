/**
 * Estimate text width based on character count.
 * Uses an approximation for 10px font size (~5.5-6px per character).
 *
 * @param text - The text to estimate width for
 * @param charWidth - Average character width (default: 5.5)
 * @param padding - Additional padding to add (default: 8)
 * @returns Estimated width in pixels
 */
export function estimateTextWidth(text: string, charWidth = 5.5, padding = 8): number {
  return text.length * charWidth + padding;
}
