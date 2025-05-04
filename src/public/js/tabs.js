PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.tabs = (() => {
    let tabs = [];
    let tabContents = [];
    let nestedTabsContainer = null;
    let nestedTabContentContainer = null;
    let logSessionArea = null;

    function init() {
        tabs = document.querySelectorAll('.tab');
        tabContents = document.querySelectorAll('.tab-content');
        nestedTabsContainer = document.querySelector('.nested-tabs');
        nestedTabContentContainer = document.querySelector('#log-content-area');
        logSessionArea = document.getElementById('log-session-area');

        tabs.forEach(tab => {
            tab.addEventListener('click', () => {
                const tabId = tab.getAttribute('data-tab');
                activateMainTab(tab, tabId);
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
        if (clickedTab.classList.contains('pointer-events-none')) return; // Don't activate disabled tabs

        tabs.forEach(t => {
            t.classList.remove('active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700', 'text-gray-900', 'dark:text-white', '-mb-px');
            t.classList.add('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent', 'text-gray-600', 'dark:text-gray-400', 'hover:bg-gray-200', 'dark:hover:bg-gray-700');
        });
        tabContents.forEach(content => content.classList.add('hidden'));

        clickedTab.classList.add('active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700', 'text-gray-900', 'dark:text-white', '-mb-px');
        clickedTab.classList.remove('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent', 'text-gray-600', 'dark:text-gray-400', 'hover:bg-gray-200', 'dark:hover:bg-gray-700');

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

    function activateNestedTab(targetId) {
        if (nestedTabsContainer) {
            nestedTabsContainer.querySelectorAll('.nested-tab').forEach(nt => {
                nt.classList.remove('active', 'text-blue-600', 'border-blue-600');
                nt.classList.add('text-gray-500', 'hover:text-gray-700', 'dark:text-gray-400', 'dark:hover:text-gray-200', 'border-transparent', 'hover:border-gray-300', 'dark:hover:border-gray-600');
            });
        }

        if (nestedTabContentContainer) {
            nestedTabContentContainer.querySelectorAll('.log-session-panel-container').forEach(panelContainer => {
                panelContainer.classList.add('hidden');
            });
        }

        const targetTab = nestedTabsContainer?.querySelector(`.nested-tab[data-nested-tab="${targetId}"]`);
        if (targetTab) {
            targetTab.classList.add('active', 'text-blue-600', 'border-blue-600');
            targetTab.classList.remove('text-gray-500', 'hover:text-gray-700', 'dark:text-gray-400', 'dark:hover:text-gray-200', 'border-transparent', 'hover:border-gray-300', 'dark:hover:border-gray-600');
        }

        const targetPanelContainer = nestedTabContentContainer?.querySelector(`#${targetId}`);
        if (targetPanelContainer) {
            targetPanelContainer.classList.remove('hidden');
            if (logSessionArea) logSessionArea.classList.remove('hidden');
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

        const disabledClasses = ['opacity-50', 'cursor-not-allowed', 'pointer-events-none'];
        const enabledClasses = ['hover:bg-gray-200', 'dark:hover:bg-gray-700', 'cursor-pointer'];

        [pbsTab, backupsTab].forEach(tab => {
            if (!isPbsAvailable) {
                tab.classList.add(...disabledClasses);
                tab.classList.remove(...enabledClasses);
                tab.setAttribute('title', 'Requires PBS integration to be configured and connected.');
                tab.classList.remove('active', 'bg-white', 'dark:bg-gray-800', 'border-gray-300', 'dark:border-gray-700');
                tab.classList.add('bg-gray-100', 'dark:bg-gray-700/50', 'border-transparent');

            } else {
                tab.classList.remove(...disabledClasses);
                tab.classList.add(...enabledClasses);
                tab.removeAttribute('title');
            }
        });
    }

    function addLogTab(sessionId, sessionTitle, fullCriteriaDesc) {
        if (!nestedTabsContainer || !nestedTabContentContainer || !logSessionArea) {
            console.error("Log tab/content container or session area not found!");
            return null; // Indicate failure
        }
        const nestedTabId = `log-session-${sessionId}`;
        const startTime = new Date(sessionId);
        const shortTitle = startTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

        const newTab = document.createElement('div');
        newTab.className = 'nested-tab px-3 py-1.5 cursor-pointer text-sm font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 border-b-2 border-transparent hover:border-gray-300 dark:hover:border-gray-600 flex items-center gap-1';
        newTab.dataset.nestedTab = nestedTabId;
        newTab.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block align-middle"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="12" y1="18" x2="12" y2="12"/><line x1="9" y1="15" x2="15" y2="15"/></svg>
            <span class="log-tab-title" title="${fullCriteriaDesc || 'Log started ' + shortTitle}">${shortTitle}</span>
        `;

        const tabTitleSpan = newTab.querySelector('.log-tab-title');
        if (tabTitleSpan) {
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
                const timerSpan = newTab.querySelector('.log-timer-display');
                if (timerSpan) newTab.insertBefore(input, timerSpan);
                else newTab.appendChild(input);
                input.focus();
                input.select();
            });
        }

        const tabCloseButton = document.createElement('button');
        tabCloseButton.className = 'ml-auto pl-2 text-gray-400 hover:text-red-600 dark:hover:text-red-400 opacity-50 hover:opacity-100 transition-opacity';
        tabCloseButton.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>';
        tabCloseButton.title = 'Close & Stop Log';
        tabCloseButton.onclick = (event) => {
            event.stopPropagation();
            const tabElement = event.currentTarget.closest('.nested-tab');
            if (!tabElement) return;
            const tabSessionId = tabElement.dataset.nestedTab;
            const tabSessionIdStr = tabSessionId.replace('log-session-', '');

            if (PulseApp.state.getActiveLogSession(tabSessionIdStr)) {
                PulseApp.thresholds.logging.stopThresholdLogging(tabSessionIdStr, 'manual');
            }

            const contentToRemove = nestedTabContentContainer.querySelector(`#${tabSessionId}`);
            tabElement.remove();
            if (contentToRemove) contentToRemove.remove();

            if (PulseApp.thresholds.logging) PulseApp.thresholds.logging.updateClearAllButtonVisibility();

            if (tabElement.classList.contains('active')) {
                activateNestedTab('nested-tab-dashboard');
            }

            if (nestedTabsContainer && logSessionArea && nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').length === 0) {
                logSessionArea.classList.add('hidden');
            }
        };
        newTab.appendChild(tabCloseButton);

        const newContent = document.createElement('div');
        newContent.id = nestedTabId;
        newContent.className = 'log-session-panel-container hidden';

        nestedTabsContainer.appendChild(newTab);
        nestedTabContentContainer.appendChild(newContent);
        logSessionArea.classList.remove('hidden');

        activateNestedTab(nestedTabId);

        return newContent; // Return the content container div
    }

    function removeLogTabAndContent(sessionId) {
         const tabToRemove = nestedTabsContainer?.querySelector(`.nested-tab[data-nested-tab="log-session-${sessionId}"]`);
         const contentToRemove = nestedTabContentContainer?.querySelector(`#log-session-${sessionId}`);

         if (tabToRemove) {
             const wasActive = tabToRemove.classList.contains('active');
             tabToRemove.remove();
             if (contentToRemove) contentToRemove.remove();

             if (PulseApp.thresholds.logging) PulseApp.thresholds.logging.updateClearAllButtonVisibility();

             if (wasActive) {
                 activateNestedTab('nested-tab-dashboard');
             }

             if (nestedTabsContainer && logSessionArea && nestedTabsContainer.querySelectorAll('.nested-tab[data-nested-tab^="log-session-"]').length === 0) {
                 logSessionArea.classList.add('hidden');
             }
         }
    }


    return {
        init,
        activateNestedTab,
        updateTabAvailability,
        addLogTab,
        removeLogTabAndContent
    };
})(); 