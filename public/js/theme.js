PulseApp.theme = (() => {
    const htmlElement = document.documentElement;
    const prefersDarkScheme = window.matchMedia('(prefers-color-scheme: dark)');
    let themeToggleButton = null;

    function applyTheme(theme) {
        if (theme === 'dark') {
            htmlElement.classList.add('dark');
            localStorage.setItem('theme', 'dark');
        } else {
            htmlElement.classList.remove('dark');
            localStorage.setItem('theme', 'light');
        }
    }

    function init() {
        themeToggleButton = document.getElementById('theme-toggle-button');
        const savedTheme = localStorage.getItem('theme');
        const initialTheme = savedTheme || (prefersDarkScheme.matches ? 'dark' : 'light');
        applyTheme(initialTheme);

        if (themeToggleButton) {
            themeToggleButton.addEventListener('click', function() {
                const currentIsDark = htmlElement.classList.contains('dark');
                applyTheme(currentIsDark ? 'light' : 'dark');
            });
        }
    }

    return {
        init
    };
})(); 