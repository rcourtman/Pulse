// Configuration banner component
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.configBanner = (() => {
    let bannerElement = null;
    let isShowing = false;

    function createBanner() {
        const banner = document.createElement('div');
        banner.id = 'config-banner';
        banner.className = 'fixed top-0 left-0 right-0 bg-yellow-500 dark:bg-yellow-600 text-white px-4 py-3 z-50 shadow-lg transform -translate-y-full transition-transform duration-300 ease-in-out';
        
        banner.innerHTML = `
            <div class="container w-[95%] max-w-screen-xl mx-auto flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <svg class="w-6 h-6 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
                    </svg>
                    <div>
                        <p class="font-medium">Configuration Required</p>
                        <p class="text-sm opacity-90">Pulse needs to be configured with your Proxmox credentials to start monitoring.</p>
                    </div>
                </div>
                <div class="flex items-center gap-2">
                    <a href="/setup.html" 
                       class="bg-white dark:bg-gray-800 text-yellow-600 dark:text-yellow-500 px-4 py-2 rounded-md text-sm font-medium hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                        Configure Now
                    </a>
                    <button onclick="PulseApp.ui.configBanner.hide()" 
                            class="text-white hover:text-gray-200 p-1 rounded-md hover:bg-yellow-600 dark:hover:bg-yellow-700 transition-colors">
                        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L13.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 13.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
                        </svg>
                    </button>
                </div>
            </div>
        `;
        
        document.body.appendChild(banner);
        return banner;
    }

    function show() {
        if (isShowing) return;
        
        if (!bannerElement) {
            bannerElement = createBanner();
        }
        
        // Add space to the body to prevent content overlap
        document.body.style.paddingTop = '80px';
        
        // Trigger the slide-down animation
        setTimeout(() => {
            bannerElement.classList.remove('-translate-y-full');
        }, 100);
        
        isShowing = true;
    }

    function hide() {
        if (!isShowing || !bannerElement) return;
        
        // Slide up animation
        bannerElement.classList.add('-translate-y-full');
        
        // Remove padding after animation
        setTimeout(() => {
            document.body.style.paddingTop = '';
            isShowing = false;
        }, 300);
    }

    function checkAndShowBanner() {
        const isConfigPlaceholder = PulseApp.state?.get('isConfigPlaceholder');
        
        if (isConfigPlaceholder) {
            show();
        } else {
            hide();
        }
    }

    return {
        show,
        hide,
        checkAndShowBanner
    };
})();