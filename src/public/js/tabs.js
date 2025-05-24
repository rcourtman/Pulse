PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.tabs = (() => {
    let tabs = [];
    let tabContents = [];
    let nestedTabsContainer = null;
    let nestedTabContentContainer = null;
    let mainTabsContainer = null;
    let logSessionArea = null;

    function init() {
        tabs = Array.from(document.querySelectorAll('.tab'));
        tabContents = document.querySelectorAll('.tab-content');
        nestedTabsContainer = document.querySelector('.nested-tabs');
        nestedTabContentContainer = document.getElementById('nested-tab-content-container');
        mainTabsContainer = document.getElementById('main-tabs-container');
        logSessionArea = document.getElementById('log-session-area');

        // Initial styling pass for all tabs to ensure consistent look from the start
        tabs.forEach(tab => {
            const isInitiallyActive = tab.classList.contains('active');
            styleMainTab(tab, isInitiallyActive); // Apply the unified styling
        });

        tabs.forEach(tab => {
            tab.addEventListener('click', () => {
                const tabId = tab.getAttribute('data-tab');
                activateMainTab(tab, tabId); // This will re-apply styles via its own calls to styleMainTab
            });
        });

        if (nestedTabsContainer) {
            nestedTabsContainer.addEventListener('click', (event) => {
                const nestedTab = event.target.closest('.nested-tab');
                if (nestedTab) {
                    const nestedTabId = nestedTab.getAttribute('data-nested-tab');
                    if (nestedTabId) {
                        activateNestedTab(nestedTabId);
                    }
                }
            });
        }
    }

    function activateMainTab(clickedTab, tabId) {
        if (clickedTab.classList.contains('pointer-events-none')) return;

        tabs.forEach(t => styleMainTab(t, false)); // Use the helper
        tabContents.forEach(content => content.classList.add('hidden'));

        styleMainTab(clickedTab, true); // Use the helper

        const activeContent = document.getElementById(tabId);
        if (activeContent) {
            activeContent.classList.remove('hidden');

            if (tabId === 'main') {
                activateNestedTab('nested-tab-dashboard');
                if (PulseApp.ui && PulseApp.ui.dashboard) {
                    PulseApp.ui.dashboard.updateDashboardTable();
                } else {
                    console.warn('[Tabs] PulseApp.ui.dashboard not available for updateDashboardTable')
                }
            }

            if (tabId === 'backups') {
                if (PulseApp.ui && PulseApp.ui.backups) {
                    PulseApp.ui.backups.updateBackupsTab();
                } else {
                    console.warn('[Tabs] PulseApp.ui.backups not available for updateBackupsTab')
                }
            }
        }
    }

    const ACTIVE_MAIN_TAB_CLASSES = ['active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700', 'text-blue-600', 'dark:text-blue-500'];
    const INACTIVE_MAIN_TAB_CLASSES = ['text-gray-500', 'dark:text-gray-400', 'hover:text-gray-700', 'dark:hover:text-gray-300', 'hover:bg-gray-200', 'dark:hover:bg-gray-700', 'border-transparent'];
    const _ACTIVE_STRUCTURAL_CLASSES = ['border', 'border-b-0', 'rounded-t', '-mb-px']; // For the "connected" look of the active tab

    function styleMainTab(tabElement, isActive) {
        const isActuallyDisabled = DISABLED_TAB_CLASSES.some(cls => tabElement.classList.contains(cls));

        // Always remove all potentially conflicting classes first for a clean slate
        tabElement.classList.remove(...ACTIVE_MAIN_TAB_CLASSES);
        tabElement.classList.remove(...INACTIVE_MAIN_TAB_CLASSES);
        tabElement.classList.remove(..._ACTIVE_STRUCTURAL_CLASSES);

        if (isActuallyDisabled) {
            // styleTabAvailability will add/remove DISABLED_TAB_CLASSES and DISABLED_TAB_EXTRA_BG.
            // We ensure no 'cursor-pointer' from other states.
            tabElement.classList.remove('cursor-pointer');
        } else {
            // Not disabled: Apply distinct active or inactive styling.
            if (isActive) {
                tabElement.classList.add(...ACTIVE_MAIN_TAB_CLASSES);
                tabElement.classList.add(..._ACTIVE_STRUCTURAL_CLASSES);
            } else { // Is Inactive and Enabled
                tabElement.classList.add(...INACTIVE_MAIN_TAB_CLASSES);
                // Inactive tabs should not have the specific "connected" look of the active tab.
                // _ACTIVE_STRUCTURAL_CLASSES were already removed.
                // INACTIVE_MAIN_TAB_CLASSES includes 'border-transparent'.
            }
            // All enabled tabs (active or inactive) should be clickable.
            tabElement.classList.add('cursor-pointer');
            // Clean up any lingering disabled styles if it's now enabled.
            tabElement.classList.remove(...DISABLED_TAB_CLASSES);
            tabElement.classList.remove(...DISABLED_TAB_EXTRA_BG);
        }
    }

    const ACTIVE_NESTED_TAB_CLASSES = ['active', 'text-blue-600', 'border-blue-600'];
    const INACTIVE_NESTED_TAB_CLASSES = ['text-gray-500', 'hover:text-gray-700', 'dark:text-gray-400', 'dark:hover:text-gray-200', 'border-transparent', 'hover:border-gray-300', 'dark:hover:border-gray-600'];

    function styleNestedTab(tabElement, isActive) {
        if (isActive) {
            tabElement.classList.add(...ACTIVE_NESTED_TAB_CLASSES);
            tabElement.classList.remove(...INACTIVE_NESTED_TAB_CLASSES);
        } else {
            tabElement.classList.remove(...ACTIVE_NESTED_TAB_CLASSES);
            tabElement.classList.add(...INACTIVE_NESTED_TAB_CLASSES);
        }
    }
    
    function activateNestedTab(targetId) {
        if (nestedTabsContainer) {
            nestedTabsContainer.querySelectorAll('.nested-tab').forEach(nt => styleNestedTab(nt, false));
        }

        if (nestedTabContentContainer) {
            nestedTabContentContainer.querySelectorAll('.log-session-panel-container').forEach(panelContainer => {
                panelContainer.classList.add('hidden');
            });
        }

        const targetTab = nestedTabsContainer?.querySelector(`.nested-tab[data-nested-tab="${targetId}"]`);
        if (targetTab) {
            styleNestedTab(targetTab, true);
        }

        const targetPanelContainer = nestedTabContentContainer?.querySelector(`#${targetId}`);
        if (targetPanelContainer) {
            targetPanelContainer.classList.remove('hidden');
            if (logSessionArea) logSessionArea.classList.remove('hidden');
        }
    }

    const DISABLED_TAB_CLASSES = ['opacity-50', 'cursor-not-allowed', 'pointer-events-none'];
    const ENABLED_TAB_CLASSES = ['hover:bg-gray-200', 'dark:hover:bg-gray-700', 'cursor-pointer'];
    const DISABLED_TAB_EXTRA_BG = ['bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent'];
    const ACTIVE_TAB_CLASSES_TO_REMOVE_WHEN_DISABLED = ['active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700'];

    function styleTabAvailability(tabElement, isAvailable) {
        if (!isAvailable) {
            tabElement.classList.add(...DISABLED_TAB_CLASSES);
            tabElement.classList.remove(...ENABLED_TAB_CLASSES);
            tabElement.setAttribute('title', 'Requires PBS integration to be configured and connected.');
            // Ensure inactive styling if disabled
            tabElement.classList.remove(...ACTIVE_TAB_CLASSES_TO_REMOVE_WHEN_DISABLED);
            tabElement.classList.add(...DISABLED_TAB_EXTRA_BG);
        } else {
            tabElement.classList.remove(...DISABLED_TAB_CLASSES);
            tabElement.classList.add(...ENABLED_TAB_CLASSES);
            tabElement.removeAttribute('title');
            // Remove potentially added disabled background if it was previously disabled
            tabElement.classList.remove(...DISABLED_TAB_EXTRA_BG);
        }
    }

    function updateTabAvailability() {
        const pbsTab = document.querySelector('.tab[data-tab="pbs"]');
        const backupsTab = document.querySelector('.tab[data-tab="backups"]');

        if (!pbsTab || !backupsTab) {
            console.warn("PBS or Backups tab element not found for availability update.");
            return;
        }

        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const isPbsAvailable = pbsDataArray.length > 0 && pbsDataArray.some(pbs => pbs.status === 'ok');

        styleTabAvailability(pbsTab, isPbsAvailable);
        styleTabAvailability(backupsTab, isPbsAvailable);

        // Ensure correct active/inactive styling after availability change
        const currentActiveTab = document.querySelector('.tab.active');

        [pbsTab, backupsTab].forEach(tab => {
            if (!tab) return; // Skip if tab element doesn't exist

            if (isPbsAvailable) {
                // Tab is now considered available
                if (tab === currentActiveTab) {
                    // If it's the current active tab, ensure it's styled as active
                    styleMainTab(tab, true);
                } else {
                    // If it's available but not active, ensure it's styled as inactive
                    styleMainTab(tab, false);
                }
            } else {
                // Tab is unavailable. styleTabAvailability already handled its appearance.
                // If it was the active tab, it should no longer appear active.
                // styleTabAvailability removes ACTIVE_TAB_CLASSES_TO_REMOVE_WHEN_DISABLED.
                // We might need to explicitly make another tab active if the current one became disabled.
                // However, the problem description is about appearance when *available*.
                // For now, let's assume disabling an active tab is handled or is a separate concern.
                // The main goal here is to fix the appearance when isPbsAvailable is true.
            }
        });
    }

    // --- Log Tab Helper Functions ---

    function createLogTabElement(nestedTabId, shortTitle, fullCriteriaDesc) {
        const newTab = document.createElement('div');
        newTab.className = 'nested-tab px-3 py-1.5 cursor-pointer text-sm font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 border-b-2 border-transparent hover:border-gray-300 dark:hover:border-gray-600 flex items-center gap-1';
        newTab.dataset.nestedTab = nestedTabId;
        newTab.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block align-middle"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="12" y1="18" x2="12" y2="12"/><line x1="9" y1="15" x2="15" y2="15"/></svg>
            <span class="log-tab-title" title="${fullCriteriaDesc || 'Log started ' + shortTitle}">${shortTitle}</span>
        `;
        return newTab;
    }

    function setupLogTabRename(tabElement) {
        const tabTitleSpan = tabElement.querySelector('.log-tab-title');
        if (!tabTitleSpan) return;

        tabTitleSpan.addEventListener('dblclick', () => {
            tabTitleSpan.classList.add('hidden');
            const input = document.createElement('input');
            input.type = 'text';
            input.value = tabTitleSpan.textContent;
            input.className = 'log-tab-rename-input flex-grow p-0 px-1 h-5 text-xs border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-200 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 outline-none';
            input.style.maxWidth = '150px';

            const finalizeRename = () => {
                const newName = input.value.trim();
                if (newName) tabTitleSpan.textContent = newName;
                input.remove();
                tabTitleSpan.classList.remove('hidden');
            };
            const cancelRename = () => {
                input.remove();
                tabTitleSpan.classList.remove('hidden');
            };

            input.addEventListener('blur', finalizeRename);
            input.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') finalizeRename();
                else if (e.key === 'Escape') cancelRename();
            });

            // Insert input before the timer span if it exists, otherwise append
            const timerSpan = tabElement.querySelector('.log-timer-display');
            if (timerSpan) tabElement.insertBefore(input, timerSpan);
            else tabElement.appendChild(input); // Append if no timer yet
            
            input.focus();
            input.select();
        });
    }

    function createLogTabCloseButton(nestedTabId) {
        const tabCloseButton = document.createElement('button');
        tabCloseButton.className = 'ml-auto pl-2 text-gray-400 hover:text-red-600 dark:hover:text-red-400 opacity-50 hover:opacity-100 transition-opacity';
        tabCloseButton.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>';
        tabCloseButton.title = 'Close & Stop Log';
        tabCloseButton.onclick = (event) => {
            event.stopPropagation();
            // Extract sessionId from nestedTabId before calling remove
            const sessionId = nestedTabId.replace('log-session-', ''); 
            removeLogTabAndContent(sessionId, true); // Pass true to indicate manual close
        };
        return tabCloseButton;
    }

    function createLogTabContentPanel(nestedTabId) {
        const newContent = document.createElement('div');
        newContent.id = nestedTabId;
        newContent.className = 'log-session-panel-container hidden'; // Start hidden
        return newContent;
    }

    // --- Main Add Log Tab Function ---

    function addLogTab(sessionId, sessionTitle, fullCriteriaDesc) {
        if (!nestedTabsContainer || !nestedTabContentContainer || !logSessionArea) {
            console.error("Log tab/content container or session area not found!");
            return null;
        }
        const nestedTabId = `log-session-${sessionId}`;
        const startTime = new Date(sessionId);
        const shortTitle = startTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

        // 1. Create elements using helpers
        const newTabElement = createLogTabElement(nestedTabId, shortTitle, fullCriteriaDesc);
        const closeButton = createLogTabCloseButton(nestedTabId); // Pass nestedTabId
        const newContentPanel = createLogTabContentPanel(nestedTabId);

        // 2. Assemble tab element
        newTabElement.appendChild(closeButton); // Append close button

        // 3. Setup interactions
        setupLogTabRename(newTabElement);

        // 4. Append to DOM
        nestedTabsContainer.appendChild(newTabElement);
        nestedTabContentContainer.appendChild(newContentPanel);
        logSessionArea.classList.remove('hidden'); // Ensure log area is visible

        // 5. Activate the new tab
        activateNestedTab(nestedTabId);

        return newContentPanel; // Return the content panel for adding log entries
    }

    function removeLogTabAndContent(sessionId, isManualClose = false) {
        const tabId = `log-session-${sessionId}`;
        const tabToRemove = nestedTabsContainer?.querySelector(`.nested-tab[data-nested-tab="${tabId}"]`);
        const contentToRemove = nestedTabContentContainer?.querySelector(`#${tabId}`);

        if (!tabToRemove) return; // Nothing to remove

        const wasActive = tabToRemove.classList.contains('active');

        // Stop logging session if closed manually
        if (isManualClose && PulseApp.state.getActiveLogSession(sessionId)) {
            PulseApp.thresholds.logging.stopThresholdLogging(sessionId, 'manual');
        }
        // Note: PulseApp.state.removeActiveLogSession(sessionId) should be called by stopThresholdLogging

        // Remove elements from DOM
        tabToRemove.remove();
        if (contentToRemove) contentToRemove.remove();

        // Update UI elements
        if (PulseApp.thresholds.logging) {
            PulseApp.thresholds.logging.updateClearAllButtonVisibility();
        }

        // Activate dashboard if the removed tab was active
        if (wasActive) {
            activateNestedTab('nested-tab-dashboard');
        }

        // Hide log area if no log tabs remain
        if (nestedTabsContainer && logSessionArea && nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').length === 0) {
            logSessionArea.classList.add('hidden');
        }
    }

    return {
        init,
        activateNestedTab,
        updateTabAvailability
    };
})();
