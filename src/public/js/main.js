// Global authentication handler
(function() {
    let authCheckInProgress = false;
    let hasRedirectedToLogin = false;
    
    // Don't redirect if we're already on the login page
    const isLoginPage = window.location.pathname === '/login.html' || window.location.pathname.endsWith('/login.html');
    
    // Override fetch to handle 401 responses globally
    const originalFetch = window.fetch;
    window.fetch = function(...args) {
        return originalFetch.apply(this, args).then(response => {
            if (response.status === 401 && !authCheckInProgress && !hasRedirectedToLogin && !isLoginPage) {
                authCheckInProgress = true;
                console.log('Authentication required - redirecting to login');
                hasRedirectedToLogin = true;
                
                // Small delay to prevent redirect loops
                setTimeout(() => {
                    window.location.href = '/login.html?redirect=' + encodeURIComponent(window.location.pathname + window.location.search);
                }, 100);
            }
            return response;
        });
    };
})();

document.addEventListener('DOMContentLoaded', function() {
    const PulseApp = window.PulseApp || {};

    // Check if we should show the security enhancement banner for v3.43+
    function checkSecurityEnhancementBanner(currentVersion) {
        const BANNER_KEY = 'pulse-security-banner-shown';
        const TARGET_VERSION = '3.43.0';
        
        // Check if we've already shown the banner
        if (localStorage.getItem(BANNER_KEY) === 'true') {
            return;
        }
        
        // Parse version (handle -rc and -dev suffixes)
        const versionMatch = currentVersion.match(/^v?(\d+)\.(\d+)\.(\d+)/);
        if (!versionMatch) return;
        
        const major = parseInt(versionMatch[1]);
        const minor = parseInt(versionMatch[2]);
        const patch = parseInt(versionMatch[3]);
        
        // Check if version is 3.43.0 or higher
        if (major > 3 || (major === 3 && minor > 43) || (major === 3 && minor === 43 && patch >= 0)) {
            // Show the banner using a custom notification
            setTimeout(() => {
                const banner = document.createElement('div');
                banner.className = 'fixed top-4 left-1/2 transform -translate-x-1/2 z-50 max-w-2xl w-full mx-4';
                banner.innerHTML = `
                    <div class="bg-blue-50 dark:bg-blue-900/50 border border-blue-200 dark:border-blue-800 rounded-lg shadow-lg p-4">
                        <div class="flex items-start">
                            <div class="flex-shrink-0">
                                <svg class="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                                </svg>
                            </div>
                            <div class="ml-3 flex-1">
                                <h3 class="text-sm font-medium text-blue-800 dark:text-blue-200">
                                    🔒 Security Enhancement in v${currentVersion}
                                </h3>
                                <div class="mt-2 text-sm text-blue-700 dark:text-blue-300">
                                    <p>Pulse now supports minimal permissions mode for improved security. 
                                    Run diagnostics to check your current permission level.</p>
                                </div>
                                <div class="mt-3 flex space-x-3">
                                    <button onclick="window.location.href='/diagnostics.html'; this.closest('.fixed').remove();" 
                                            class="text-sm font-medium text-blue-600 hover:text-blue-500 dark:text-blue-400 dark:hover:text-blue-300 underline">
                                        Check Diagnostics
                                    </button>
                                    <button onclick="localStorage.setItem('${BANNER_KEY}', 'true'); this.closest('.fixed').remove();" 
                                            class="text-sm font-medium text-gray-600 hover:text-gray-500 dark:text-gray-400 dark:hover:text-gray-300">
                                        Dismiss
                                    </button>
                                </div>
                            </div>
                            <div class="ml-3 flex-shrink-0">
                                <button onclick="localStorage.setItem('${BANNER_KEY}', 'true'); this.closest('.fixed').remove();" 
                                        class="text-gray-400 hover:text-gray-500 dark:text-gray-500 dark:hover:text-gray-400">
                                    <svg class="h-5 w-5" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                `;
                document.body.appendChild(banner);
                
                // Auto-remove after 30 seconds if not dismissed
                setTimeout(() => {
                    if (banner.parentNode) {
                        banner.style.transition = 'opacity 0.5s';
                        banner.style.opacity = '0';
                        setTimeout(() => banner.remove(), 500);
                    }
                }, 30000);
            }, 2000); // Show after 2 seconds to not interfere with initial load
        }
    }

    function updateAllUITables() {
        if (!PulseApp.state || !PulseApp.state.get('initialDataReceived')) {
            return;
        }
        
        // Global scroll preservation for all main scrollable containers
        const allContainers = [
            document.querySelector('.table-container'), // Main dashboard table
            document.querySelector('#node-summary-cards-container'), // Node cards
            document.querySelector('#storage-info-content'), // Storage tab
            document.querySelector('#unified-table'), // Unified backups tab
            // Also find any overflow-x-auto or overflow-y-auto containers that might be scrolled
            ...Array.from(document.querySelectorAll('.overflow-x-auto, .overflow-y-auto, [style*="overflow"]'))
        ];
        
        const scrollableContainers = allContainers.filter((container, index, array) => {
            // Remove duplicates and null containers
            return container && array.indexOf(container) === index;
        });
        
        // Capture all scroll positions before any updates
        const scrollPositions = scrollableContainers.map(container => {
            const position = {
                element: container,
                scrollLeft: container.scrollLeft,
                scrollTop: container.scrollTop
            };
            if (position.scrollLeft > 0 || position.scrollTop > 0) {
            }
            return position;
        });
        
        const nodesData = PulseApp.state.get('nodesData');
        const pbsDataArray = PulseApp.state.get('pbsDataArray');

        // Disable individual scroll preservation to avoid conflicts
        const originalPreserveScrollPosition = PulseApp.utils.preserveScrollPosition;
        PulseApp.utils.preserveScrollPosition = (element, updateFn) => {
            // Just run the update function without scroll preservation
            updateFn();
        };

        PulseApp.ui.nodes?.updateNodeSummaryCards(nodesData);
        PulseApp.ui.dashboard?.updateDashboardTable();
        PulseApp.ui.storage?.updateStorageInfo();
        
        // Restore the original function
        PulseApp.utils.preserveScrollPosition = originalPreserveScrollPosition;

        // Update tab availability based on PBS data
        PulseApp.ui.tabs?.updateTabAvailability();

        updateLoadingOverlayVisibility(); // Call the helper function

        PulseApp.thresholds?.logging?.checkThresholdViolations();
        
        // Update alerts when state changes
        const state = PulseApp.state.getFullState();
        PulseApp.alerts?.updateAlertsFromState?.(state);
        
        // Check and show configuration banner if needed
        PulseApp.ui.configBanner?.checkAndShowBanner();
        
        // Optimized scroll preservation
        const mainTableContainer = document.querySelector('.table-container');
        if (mainTableContainer) {
            const savedScrollTop = mainTableContainer.scrollTop;
            const savedScrollLeft = mainTableContainer.scrollLeft;
            
            if (savedScrollTop > 0 || savedScrollLeft > 0) {
                // Single efficient restoration strategy
                const restoreScroll = () => {
                    mainTableContainer.scrollTo({
                        top: savedScrollTop,
                        left: savedScrollLeft,
                        behavior: 'instant'
                    });
                };
                
                // Primary restoration - immediate
                requestAnimationFrame(restoreScroll);
                
                // Fallback restoration - after layout
                setTimeout(() => {
                    if (Math.abs(mainTableContainer.scrollTop - savedScrollTop) > 5) {
                        restoreScroll();
                    }
                }, 100);
            }
        }
    }

    function updateLoadingOverlayVisibility() {
        const loadingOverlay = document.getElementById('loading-overlay');
        if (!loadingOverlay) return;

        const isConnected = PulseApp.socketHandler?.isConnected();
        const initialDataReceived = PulseApp.state?.get('initialDataReceived');
        
        const hasAnyData = PulseApp.state && (
            (PulseApp.state.get('nodesData') || []).length > 0 ||
            (PulseApp.state.get('vmsData') || []).length > 0 ||
            (PulseApp.state.get('containersData') || []).length > 0 ||
            (PulseApp.state.get('pbsDataArray') || []).length > 0
        );

        if (loadingOverlay.style.display !== 'none') { // Only act if currently visible
            if (isConnected && (initialDataReceived || hasAnyData)) {
                loadingOverlay.style.display = 'none';
            } else if (!isConnected) {
            }
            // If initialDataReceived is false, or socket is connected but no data yet, overlay remains.
        } else if (!isConnected && loadingOverlay.style.display === 'none') {
            // If overlay is hidden but socket disconnects, re-show it.
            loadingOverlay.style.display = 'flex'; // Or 'block', or its original display type
        }
    }

    function initializeModules() {
        PulseApp.state?.init?.();
        PulseApp.config?.init?.();
        PulseApp.utils?.init?.();
        PulseApp.theme?.init?.();
        // Ensure socketHandler.init receives both callbacks if it's designed to accept them
        // If socketHandler.init only expects one, this might need adjustment in socketHandler.js
        PulseApp.socketHandler?.init?.(updateAllUITables, updateLoadingOverlayVisibility); 
        PulseApp.tooltips?.init?.();
        PulseApp.alerts?.init?.();

        PulseApp.ui = PulseApp.ui || {};
        PulseApp.ui.tabs?.init?.();
        PulseApp.ui.nodes?.init?.();
        PulseApp.ui.dashboard?.init?.();
        PulseApp.ui.storage?.init?.();
        PulseApp.ui.unifiedBackups?.init?.();
        PulseApp.ui.settings?.init?.();
        PulseApp.ui.thresholds?.init?.();
        PulseApp.ui.alerts?.init?.();
        PulseApp.ui.chartsControls?.init?.();
        PulseApp.ui.common?.init?.();

        PulseApp.thresholds = PulseApp.thresholds || {};
    }

    function validateCriticalElements() {
        const criticalElements = [
            'connection-status',
            'main-table',
            'dashboard-search',
            'dashboard-status-text',
            'app-version'
        ];
        let allFound = true;
        criticalElements.forEach(id => {
            if (!document.getElementById(id)) {
                console.error(`Critical element #${id} not found!`);
            }
        });
        if (!document.querySelector('#main-table tbody')) {
             console.error('Critical element #main-table tbody not found!');
        }
        return allFound;
    }

    function fetchVersion() {
        const versionSpan = document.getElementById('app-version');
        if (!versionSpan) {
            console.error('Version span element not found');
            return;
        }
        
        fetch('/api/version')
            .then(response => {
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                return response.json();
            })
            .then(data => {
                if (data.version) {
                    versionSpan.textContent = data.version;
                    
                    // Check if this is a release candidate or development version
                    const versionBadge = document.getElementById('version-badge');
                    if (versionBadge && data.version) {
                        const isVersionRC = data.version.includes('-rc');
                        const isVersionDev = data.version.includes('-dev');
                        
                        if (isVersionDev) {
                            versionBadge.textContent = 'DEV';
                            versionBadge.classList.remove('hidden');
                        } else if (isVersionRC) {
                            versionBadge.textContent = 'RC';
                            versionBadge.classList.remove('hidden');
                        }
                    }
                    
                    // Also update the page title
                    const isVersionRC = data.version && data.version.includes('-rc');
                    const isVersionDev = data.version && data.version.includes('-dev');
                    document.title = isVersionDev ? 'Pulse DEV' : (isVersionRC ? 'Pulse RC' : 'Pulse');
                    
                    // Security enhancement banner removed per user request
                    // checkSecurityEnhancementBanner(data.version);
                    
                    // Check if update is available
                    if (data.updateAvailable && data.latestVersion) {
                        // Check if update indicator already exists
                        const existingIndicator = document.getElementById('update-indicator');
                        if (!existingIndicator) {
                            // Create update indicator
                            const updateIndicator = document.createElement('span');
                            updateIndicator.id = 'update-indicator';
                            updateIndicator.className = 'ml-2 inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
                            updateIndicator.innerHTML = `
                                <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-8.293l-3-3a1 1 0 00-1.414 0l-3 3a1 1 0 001.414 1.414L9 9.414V13a1 1 0 102 0V9.414l1.293 1.293a1 1 0 001.414-1.414z" clip-rule="evenodd"/>
                                </svg>
                                v${data.latestVersion} available
                            `;
                            updateIndicator.title = 'Click to view the latest release';
                            updateIndicator.style.cursor = 'pointer';
                            updateIndicator.addEventListener('click', (e) => {
                                e.preventDefault();
                                e.stopPropagation();
                                window.open('https://github.com/rcourtman/Pulse/releases/latest', '_blank');
                            });
                            
                            // Insert after version link
                            versionSpan.parentNode.insertBefore(updateIndicator, versionSpan.nextSibling);
                        }
                    } else {
                        // Remove update indicator if no update available
                        const existingIndicator = document.getElementById('update-indicator');
                        if (existingIndicator) {
                            existingIndicator.remove();
                        }
                    }
                } else {
                    versionSpan.textContent = 'unknown';
                }
            })
            .catch(error => {
                console.error('Error fetching version:', error);
                versionSpan.textContent = 'error';
            });
    }

    if (!validateCriticalElements()) {
        console.error("Stopping JS execution due to missing critical elements.");
        return;
    }

    initializeModules();
    
    // Check if configuration is missing or contains placeholders and automatically open settings modal
    checkAndOpenSettingsIfNeeded();
    
    // Fetch version immediately and retry after a short delay if needed
    fetchVersion();
    
    // Also try again after DOM is fully ready and socket might be connected
    setTimeout(() => {
        const versionSpan = document.getElementById('app-version');
        if (versionSpan && (versionSpan.textContent === 'loading...' || versionSpan.textContent === 'error')) {
            fetchVersion();
        }
    }, 2000);
    
    setInterval(() => {
        fetchVersion();
    }, 6 * 60 * 60 * 1000);
    
    /**
     * Check if configuration is missing or contains placeholder values and open settings modal
     */
    async function checkAndOpenSettingsIfNeeded() {
        try {
            // Wait a moment for the socket connection to establish and initial data to arrive
            setTimeout(async () => {
                try {
                    const response = await fetch('/api/health');
                    if (response.ok) {
                        const health = await response.json();
                        
                        // Check if configuration has placeholder values or no data is available
                        if (health.system && health.system.configPlaceholder) {
                            // Wait for settings module to be fully initialized
                            setTimeout(() => {
                                if (PulseApp.ui.settings && typeof PulseApp.ui.settings.openModal === 'function') {
                                    PulseApp.ui.settings.openModal();
                                }
                            }, 500);
                        }
                    }
                } catch (error) {
                    console.error('[Main] Error checking configuration status:', error);
                }
            }, 2000); // Wait 2 seconds for everything to settle
        } catch (error) {
            console.error('[Main] Error in checkAndOpenSettingsIfNeeded:', error);
        }
    }
});


