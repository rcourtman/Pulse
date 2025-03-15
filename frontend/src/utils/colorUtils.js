/**
 * Generates a consistent color based on a string input (node name)
 * Returns a color with reduced opacity for use in table row backgrounds
 * 
 * @param {string} str - The input string (node name)
 * @param {number} opacity - Opacity value between 0 and 1
 * @param {string} mode - 'dark' or 'light' theme mode
 * @returns {string} - RGBA color string
 */
export const getNodeColor = (str, opacity = 0.1, mode = 'light') => {
  if (!str) return mode === 'dark' ? 'rgba(255, 255, 255, 0.02)' : 'rgba(0, 0, 0, 0.02)';
  
  // Generate a numeric hash from the string
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  
  // Different color sets for dark and light modes
  const baseColors = mode === 'dark' ? [
    [130, 177, 255], // Blue
    [255, 145, 143], // Red
    [126, 211, 150], // Green
    [241, 186, 252], // Pink
    [255, 213, 128], // Yellow
    [177, 156, 255], // Purple
    [158, 230, 240], // Cyan
    [255, 169, 119], // Orange
  ] : [
    [25, 118, 210],  // Blue
    [211, 47, 47],   // Red
    [46, 125, 50],   // Green
    [194, 24, 91],   // Pink
    [255, 145, 0],   // Orange
    [123, 31, 162],  // Purple
    [0, 131, 143],   // Teal
    [109, 76, 65]    // Brown
  ];
  
  // Use the hash to pick a color from the baseColors
  const colorIndex = Math.abs(hash) % baseColors.length;
  const [r, g, b] = baseColors[colorIndex];
  
  // Return rgba color string with appropriate opacity
  return `rgba(${r}, ${g}, ${b}, ${opacity})`;
};

/**
 * Generates a consistent text color based on a string input (node name)
 * 
 * @param {string} str - The input string (node name)
 * @param {string} mode - 'dark' or 'light' theme mode
 * @returns {string} - RGB color string
 */
export const getNodeTextColor = (str, mode = 'light') => {
  if (!str) return mode === 'dark' ? 'rgba(255, 255, 255, 0.7)' : 'rgba(0, 0, 0, 0.7)';
  
  // Generate a numeric hash from the string
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  
  // Different color sets for dark and light modes
  const baseColors = mode === 'dark' ? [
    [130, 177, 255], // Blue
    [255, 145, 143], // Red
    [126, 211, 150], // Green
    [241, 186, 252], // Pink
    [255, 213, 128], // Yellow
    [177, 156, 255], // Purple
    [158, 230, 240], // Cyan
    [255, 169, 119], // Orange
  ] : [
    [25, 118, 210],  // Blue
    [211, 47, 47],   // Red
    [46, 125, 50],   // Green
    [194, 24, 91],   // Pink
    [255, 145, 0],   // Orange
    [123, 31, 162],  // Purple
    [0, 131, 143],   // Teal
    [109, 76, 65]    // Brown
  ];
  
  // Use the hash to pick a color from the baseColors
  const colorIndex = Math.abs(hash) % baseColors.length;
  const [r, g, b] = baseColors[colorIndex];
  
  // Return rgb color string
  return `rgb(${r}, ${g}, ${b})`;
}; 