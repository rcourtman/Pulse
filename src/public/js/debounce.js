// Debounce utility function
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Export for use in other modules
if (typeof PulseApp !== 'undefined') {
    PulseApp.utils = PulseApp.utils || {};
    PulseApp.utils.debounce = debounce;
}