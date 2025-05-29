// WCAG-compliant color replacements
PulseApp.wcagColors = (() => {
    // Map of old colors to WCAG-compliant replacements
    // These meet WCAG AA standards (4.5:1 for normal text, 3:1 for large text)
    const colorMap = {
        // Gray scale improvements for better contrast
        'text-gray-400': 'text-gray-600', // Light mode
        'dark:text-gray-400': 'dark:text-gray-300', // Dark mode
        'text-gray-500': 'text-gray-700',
        'dark:text-gray-500': 'dark:text-gray-300',
        'text-gray-600': 'text-gray-700',
        'dark:text-gray-600': 'dark:text-gray-300',
        
        // Status colors with better contrast
        'text-green-500': 'text-green-700',
        'dark:text-green-500': 'dark:text-green-400',
        'text-red-500': 'text-red-700',
        'dark:text-red-500': 'dark:text-red-400',
        'text-yellow-500': 'text-yellow-700',
        'dark:text-yellow-500': 'dark:text-yellow-400',
        'text-blue-500': 'text-blue-700',
        'dark:text-blue-500': 'dark:text-blue-400',
        
        // Background adjustments for better contrast
        'bg-gray-100': 'bg-gray-50',
        'dark:bg-gray-800': 'dark:bg-gray-900',
        
        // Specific problematic combinations
        'text-gray-400 dark:text-gray-500': 'text-gray-600 dark:text-gray-300',
        'text-gray-500 dark:text-gray-400': 'text-gray-700 dark:text-gray-300',
        'text-gray-600 dark:text-gray-400': 'text-gray-700 dark:text-gray-300'
    };
    
    function applyWCAGColors() {
        // This would be called during initialization to update CSS classes
    }
    
    function getWCAGColor(originalClass) {
        return colorMap[originalClass] || originalClass;
    }
    
    return {
        applyWCAGColors,
        getWCAGColor
    };
})();