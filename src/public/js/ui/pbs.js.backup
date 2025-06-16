PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {

    // Global state tracker for expanded PBS tasks
    let expandedTaskState = new Set();
    let expandedShowMoreState = new Set();
    
    // Simple persistent state for expanded detail cards
    let persistentExpandedDetails = new Set();
    let lastKnownMobileState = null; // Track viewport state
    
    // Helper function to find task data by UPID
    function findTaskByUpid(pbsArray, upid) {
        for (const pbsInstance of pbsArray) {
            const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
            for (const taskType of taskTypes) {
                if (pbsInstance[taskType] && pbsInstance[taskType].recentTasks) {
                    const found = pbsInstance[taskType].recentTasks.find(task => task.upid === upid);
                    if (found) return found;
                }
            }
        }
        return null;
    }
    let selectedPbsTabIndex = 0; // Track selected PBS tab index globally
    
    // Namespace filtering
    let pbsNamespaceFilter = null;
    


    const CSS_CLASSES = {
        TEXT_GREEN_500_DARK_GREEN_400: 'text-green-500 dark:text-green-400',
        TEXT_RED_500_DARK_RED_400: 'text-red-500 dark:text-red-400',
        TEXT_GRAY_400: 'text-gray-400',
        TEXT_GRAY_600_DARK_GRAY_400: 'text-gray-600 dark:text-gray-400',
        TEXT_XS: 'text-xs',
        FONT_BOLD: 'font-bold',
        BORDER_B_GRAY_200_DARK_GRAY_700: 'border-b border-gray-200 dark:border-gray-700',
        HOVER_BG_GRAY_50_DARK_HOVER_BG_GRAY_700_50: 'hover:bg-gray-50 dark:hover:bg-gray-700/50',
        TRANSITION_COLORS: 'transition-colors',
        DURATION_150: 'duration-150',
        EASE_IN_OUT: 'ease-in-out',
        P1_PX2: 'p-1 px-2',
        TEXT_SM: 'text-sm',
        TEXT_GRAY_700_DARK_GRAY_300: 'text-gray-700 dark:text-gray-300',
        TEXT_GRAY_500_DARK_GRAY_400: 'text-gray-500 dark:text-gray-400',
        WHITESPACE_NOWRAP: 'whitespace-nowrap',
        FONT_MONO: 'font-mono',
        TRUNCATE: 'truncate',
        PBS_INSTANCE_SECTION: 'pbs-instance-section',
        PBS_INSTANCE_DETAILS: 'pbs-instance-details',
        PBS_TASK_SECTION: 'pbs-task-section',
        PBS_SHOW_MORE: 'pbs-show-more',
        PBS_NO_TASKS: 'pbs-no-tasks',
        HIDDEN: 'hidden',
        BORDER_GRAY_200_DARK_BORDER_GRAY_700: 'border border-gray-200 dark:border-gray-700',
        ROUNDED: 'rounded',
        P3: 'p-3',
        BG_GRAY_100_50_DARK_BG_GRAY_700_50: 'bg-gray-100/50 dark:bg-gray-700/50',
        BORDER_L_4_RED_500_DARK_RED_400: 'border-l-4 border-red-500 dark:border-red-400',
        BORDER_L_4_TRANSPARENT: 'border-l-4 border-transparent',
        TEXT_MD: 'text-md',
        FONT_SEMIBOLD: 'font-semibold',
        MB2: 'mb-2',
        SPACE_Y_1: 'space-y-1',
        FONT_MEDIUM: 'font-medium',
        TEXT_GRAY_800_DARK_GRAY_200: 'text-gray-800 dark:text-gray-200',
        ML1: 'ml-1',
        TEXT_GREEN_600_DARK_GREEN_400: 'text-green-600 dark:text-green-400',
        TEXT_RED_600_DARK_RED_400: 'text-red-600 dark:text-red-400',
        OVERFLOW_X_AUTO: 'overflow-x-auto',
        MIN_W_FULL: 'min-w-full',
        DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700: 'divide-y divide-gray-200 dark:divide-gray-700',
        STICKY: 'sticky',
        TOP_0: 'top-0',
        Z_10: 'z-10',
        BG_GRAY_100_DARK_BG_GRAY_800: 'bg-gray-100 dark:bg-gray-800',
        TRACKING_WIDER: 'tracking-wider',
        TEXT_LEFT: 'text-left',
        TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300: 'text-gray-600 uppercase dark:text-gray-300',
        BORDER_B_GRAY_300_DARK_BORDER_GRAY_600: 'border-b border-gray-300 dark:border-gray-600',
        PBS_TASK_TBODY: 'pbs-task-tbody',
        PT1: 'pt-1',
        TEXT_RIGHT: 'text-right',
        TEXT_BLUE_600_HOVER_BLUE_800_DARK_BLUE_400_DARK_HOVER_BLUE_300: 'text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300',
        ITALIC: 'italic',
        TEXT_GRAY_500_DARK_TEXT_GRAY_400_P4_TEXT_CENTER_TEXT_SM: 'text-gray-500 dark:text-gray-400 p-4 text-center text-sm',
        FLEX: 'flex',
        JUSTIFY_BETWEEN: 'justify-between',
        ITEMS_CENTER: 'items-center',
        MB3: 'mb-3',
        TEXT_LG: 'text-lg',
        SPACE_Y_4: 'space-y-4',
        GRID: 'grid',
        GRID_COLS_1: 'grid-cols-1',
        MD_GRID_COLS_2: 'md:grid-cols-2',
        LG_GRID_COLS_4: 'lg:grid-cols-4',
        GAP_4: 'gap-4',
        BG_GRAY_400_DARK_BG_GRAY_500: 'bg-gray-400 dark:bg-gray-500',
        BG_GREEN_500: 'bg-green-500',
        BG_YELLOW_500: 'bg-yellow-500',
        BG_RED_500: 'bg-red-500',
        INLINE_BLOCK: 'inline-block',
        W_3: 'w-3',
        H_3: 'h-3',
        ROUNDED_FULL: 'rounded-full',
        MR2: 'mr-2',
        FLEX_SHRINK_0: 'flex-shrink-0',
        ANIMATE_SPIN: 'animate-spin',
        BORDER_T_2: 'border-t-2',
        BORDER_B_2: 'border-b-2',
        BORDER_BLUE_500: 'border-blue-500',
        BG_GRAY_50_DARK_BG_GRAY_800_30: 'bg-gray-50 dark:bg-gray-800/30',
        PBS_TAB_CONTAINER: 'flex border-b border-gray-300 dark:border-gray-600 mb-4',
        PBS_TAB_BUTTON: 'px-4 py-2 -mb-px border-b-2 border-transparent font-medium text-sm text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:border-gray-500 focus:outline-none',
        PBS_TAB_BUTTON_ACTIVE: 'border-blue-500 text-blue-600 dark:text-blue-400 dark:border-blue-400',
        PBS_TAB_CONTENT_AREA: 'pbs-tab-content-area'
    };

    const ID_PREFIXES = {
        PBS_INSTANCE: 'pbs-instance-',
        PBS_DETAILS: 'pbs-details-',
        PBS_DS_SECTION: 'pbs-ds-section-',
        PBS_DS_TABLE: 'pbs-ds-table-',
        PBS_DS_TBODY: 'pbs-ds-tbody-',
        PBS_SUMMARIES_SECTION: 'pbs-summaries-section-',
        PBS_RECENT_BACKUP_TASKS_TABLE: 'pbs-recent-backup-tasks-table-',
        PBS_RECENT_VERIFY_TASKS_TABLE: 'pbs-recent-verify-tasks-table-',
        PBS_RECENT_SYNC_TASKS_TABLE: 'pbs-recent-sync-tasks-table-',
        PBS_RECENT_PRUNEGC_TASKS_TABLE: 'pbs-recent-prunegc-tasks-table-',
        PBS_RECENT_BACKUP_TASKS_TBODY: 'pbs-recent-backup-tasks-tbody-',
        PBS_RECENT_VERIFY_TASKS_TBODY: 'pbs-recent-verify-tasks-tbody-',
        PBS_RECENT_SYNC_TASKS_TBODY: 'pbs-recent-sync-tasks-tbody-',
        PBS_RECENT_PRUNEGC_TASKS_TBODY: 'pbs-recent-prunegc-tasks-tbody-',
        PBS_INSTANCES_CONTAINER: 'pbs-instances-container',
        PBS_TAB_BUTTON_PREFIX: 'pbs-tab-button-',
        PBS_TAB_CONTENT_PREFIX: 'pbs-tab-content-'
    };

    const DATA_ATTRIBUTES = {
        TASK_TYPE: 'data-task-type',
        HANDLER_ATTACHED: 'data-handler-attached',
    };

    // Mobile detection and responsive utilities
    function isMobileView() {
        return window.innerWidth < 768;
    }

    function isTabletView() {
        return window.innerWidth >= 768 && window.innerWidth < 1024;
    }

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

    function _initMobileScrollIndicators() {
        const tableContainers = document.querySelectorAll('.pbs-table-container');
        const scrollHints = document.querySelectorAll('.pbs-scroll-hint');
        
        if (!tableContainers.length || !scrollHints.length) return;
        
        tableContainers.forEach((container, index) => {
            const scrollHint = scrollHints[index];
            if (!scrollHint) return;
            
            let scrollHintTimer;
            
            // Hide scroll hint after 5 seconds or on first scroll
            const hideScrollHint = () => {
                if (scrollHint) {
                    scrollHint.style.display = 'none';
                }
            };
            
            scrollHintTimer = setTimeout(hideScrollHint, 5000);
            
            // Handle scroll events
            container.addEventListener('scroll', () => {
                hideScrollHint();
                clearTimeout(scrollHintTimer);
            }, { passive: true });
            
            // Also hide on table container click/touch
            container.addEventListener('touchstart', () => {
                hideScrollHint();
                clearTimeout(scrollHintTimer);
            }, { passive: true });
        });
    }

    const getPbsStatusIcon = (status) => {
        if (status === 'OK') {
            return `<span class="${CSS_CLASSES.TEXT_GREEN_500_DARK_GREEN_400}" title="OK">OK</span>`;
        } else if (status === 'running') {
            return `<span class="${CSS_CLASSES.INLINE_BLOCK} ${CSS_CLASSES.ANIMATE_SPIN} ${CSS_CLASSES.ROUNDED_FULL} ${CSS_CLASSES.H_3} ${CSS_CLASSES.W_3} ${CSS_CLASSES.BORDER_T_2} ${CSS_CLASSES.BORDER_B_2} ${CSS_CLASSES.BORDER_BLUE_500}" title="Running"></span>`;
        } else if (status) {
            return `<span class="${CSS_CLASSES.TEXT_RED_500_DARK_RED_400} ${CSS_CLASSES.FONT_BOLD}" title="${status}">ERROR</span>`;
        } else {
            return `<span class="${CSS_CLASSES.TEXT_GRAY_400}" title="Unknown">?</span>`;
        }
    };

    const getPbsStatusDisplay = (status) => {
        if (status === 'OK') {
            return `<span class="${CSS_CLASSES.TEXT_GREEN_500_DARK_GREEN_400}">OK</span>`;
        } else if (status === 'running') {
            return `<span class="${CSS_CLASSES.INLINE_BLOCK} ${CSS_CLASSES.ANIMATE_SPIN} ${CSS_CLASSES.ROUNDED_FULL} ${CSS_CLASSES.H_3} ${CSS_CLASSES.W_3} ${CSS_CLASSES.BORDER_T_2} ${CSS_CLASSES.BORDER_B_2} ${CSS_CLASSES.BORDER_BLUE_500}" title="Running"></span> <span class="${CSS_CLASSES.TEXT_BLUE_600_DARK_BLUE_400}">Running</span>`;
        } else if (status) {
            // For failed tasks, show the full error message
            const shortStatus = status.length > 50 ? `${status.substring(0, 47)}...` : status;
            return `<span class="${CSS_CLASSES.TEXT_RED_500_DARK_RED_400} ${CSS_CLASSES.FONT_BOLD}" title="${status}">ERROR</span> <span class="${CSS_CLASSES.TEXT_RED_600_DARK_RED_400} ${CSS_CLASSES.TEXT_XS}" title="${status}">${shortStatus}</span>`;
        } else {
            return `<span class="${CSS_CLASSES.TEXT_GRAY_400}">? Unknown</span>`;
        }
    };

    const getPbsGcStatusText = (gcStatus) => {
      if (!gcStatus || gcStatus === 'unknown' || gcStatus === 'N/A') {
        return `<span class="${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.TEXT_GRAY_400}">-</span>`;
      }
      let colorClass = CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400;
      if (gcStatus.includes('error') || gcStatus.includes('failed')) {
          colorClass = CSS_CLASSES.TEXT_RED_500_DARK_RED_400;
      } else if (gcStatus === 'OK') {
          colorClass = CSS_CLASSES.TEXT_GREEN_500_DARK_GREEN_400;
      }
      return `<span class="${CSS_CLASSES.TEXT_XS} ${colorClass}">${gcStatus}</span>`;
    };

    const parsePbsTaskTarget = (task) => {
      // For synthetic backup run tasks, use the guest field directly
      if (task.guest && task.pbsBackupRun) {
          return task.guest;
      }
      
      const workerId = task.worker_id || task.id || '';
      const taskType = task.worker_type || task.type || '';

      let displayTarget = workerId;

      if (taskType === 'backup' || taskType === 'verify') {
        const parts = workerId.split(':');
        if (parts.length >= 2) {
          const targetPart = parts[1];
          const targetSubParts = targetPart.split('/');
          if (targetSubParts.length >= 2) {
              const guestType = targetSubParts[0];
              const guestId = targetSubParts[1];
              displayTarget = `${guestType}/${guestId}`;
          }
        }
      } else if (taskType === 'prune' || taskType === 'garbage_collection') {
          const parts = workerId.split('::');
          if (parts.length === 2) {
              displayTarget = `Prune ${parts[0]} (${parts[1]})`;
          } else {
              const singleColonParts = workerId.split(':');
               if (singleColonParts.length === 1 && workerId !== '') {
                   displayTarget = `GC ${workerId}`;
               } else if (singleColonParts.length >= 2) {
                   displayTarget = `Prune ${singleColonParts[0]} (${singleColonParts[1]})`
               }
          }
      } else if (taskType === 'sync') {
          displayTarget = `Sync Job: ${workerId}`;
      }

      return displayTarget;
    };

    // Mobile-friendly task card component
    const _createMobileTaskCard = (task) => {
        const target = parsePbsTaskTarget(task);
        const statusDisplayHTML = getPbsStatusDisplay(task.status);
        const startTime = task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A';
        const duration = task.duration !== null ? PulseApp.utils.formatDuration(task.duration) : 'N/A';
        const upid = task.upid || 'N/A';
        const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

        const card = document.createElement('div');
        card.className = 'mobile-task-card p-3 border border-gray-200 dark:border-gray-700 rounded-lg mb-3 bg-white dark:bg-gray-800 transition-all duration-200';
        
        // Add UPID as data attribute for tracking expanded state
        card.dataset.upid = task.upid || '';
        
        // Add status-based styling for better problem visibility
        const isFailed = task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running');
        
        if (isFailed) {
            card.classList.add('border-red-300', 'dark:border-red-600', 'bg-red-50', 'dark:bg-red-900/10');
        } else if (task.status && task.status.toLowerCase().includes('running')) {
            card.classList.add('border-blue-300', 'dark:border-blue-600', 'bg-blue-50', 'dark:bg-blue-900/10');
        }

        // Create card header
        const cardHeader = document.createElement('div');
        cardHeader.className = 'flex justify-between items-start mb-3';
        
        const targetElement = document.createElement('div');
        targetElement.className = 'font-medium text-sm truncate pr-2 flex-1';
        if (isFailed) {
            targetElement.innerHTML = `${target}`;
        } else {
            targetElement.textContent = target;
        }
        
        const statusElement = document.createElement('div');
        statusElement.className = 'flex-shrink-0 text-sm';
        statusElement.innerHTML = statusDisplayHTML;
        
        cardHeader.appendChild(targetElement);
        cardHeader.appendChild(statusElement);
        card.appendChild(cardHeader);

        // Create card details grid
        const detailsGrid = document.createElement('div');
        detailsGrid.className = 'grid grid-cols-2 gap-3 text-xs text-gray-600 dark:text-gray-400';
        
        detailsGrid.innerHTML = `
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Start Time</div>
                <div class="truncate">${startTime}</div>
            </div>
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Duration</div>
                <div class="truncate">${duration}</div>
            </div>
        `;
        
        card.appendChild(detailsGrid);

        // Add UPID info if space allows
        if (upid !== 'N/A') {
            const upidElement = document.createElement('div');
            upidElement.className = 'mt-2 text-xs text-gray-500 dark:text-gray-500';
            upidElement.innerHTML = `<span class="font-medium">UPID:</span> <span class="font-mono break-all">${shortUpid}</span>`;
            upidElement.title = upid;
            card.appendChild(upidElement);
        }

        // Add expand button for failed tasks
        if (isFailed) {
            const expandButton = document.createElement('button');
            expandButton.className = 'mt-3 w-full py-2 px-3 text-xs text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-600 rounded bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 transition-colors tap-target';
            expandButton.textContent = 'Show Error Details';
            
            // Simple persistent click handler
            expandButton.addEventListener('click', (event) => {
                event.preventDefault();
                event.stopPropagation();
                
                const upid = task.upid;
                
                if (persistentExpandedDetails.has(upid)) {
                    // Collapse - remove from persistent state
                    persistentExpandedDetails.delete(upid);
                    expandButton.textContent = 'Show Error Details';
                    // Remove detail card if it exists
                    const existingDetailCard = card.nextElementSibling;
                    if (existingDetailCard && existingDetailCard.classList.contains('mobile-task-detail-card')) {
                        existingDetailCard.remove();
                    }
                } else {
                    // Expand - add to persistent state
                    persistentExpandedDetails.add(upid);
                    expandButton.textContent = 'Hide Error Details';
                    // Create detail card
                    const detailCard = _createMobileTaskDetailCard(task);
                    card.insertAdjacentElement('afterend', detailCard);
                }
            });
            
            // Set button text based on persistent state
            if (persistentExpandedDetails.has(task.upid)) {
                expandButton.textContent = 'Hide Error Details';
            }
            
            card.appendChild(expandButton);
        }

        return card;
    };

    // Mobile task detail card component
    const _createMobileTaskDetailCard = (task) => {
        const detailCard = document.createElement('div');
        detailCard.className = 'mobile-task-detail-card p-4 border border-red-200 dark:border-red-600 rounded-lg mb-3 bg-red-25 dark:bg-red-950/10';
        
        const detailContent = document.createElement('div');
        detailContent.className = 'space-y-3 text-sm';
        
        // Error details section
        const errorSection = document.createElement('div');
        errorSection.innerHTML = `
            <div class="font-semibold text-red-700 dark:text-red-300 mb-2">Error Details:</div>
            <div class="bg-gray-100 dark:bg-gray-800 p-3 rounded font-mono text-xs break-all overflow-x-auto">
                ${task.status || 'No error message available'}
            </div>
        `;
        detailContent.appendChild(errorSection);
        
        // Task info section
        const infoSection = document.createElement('div');
        infoSection.className = 'space-y-2 text-xs text-gray-600 dark:text-gray-400';
        
        const endTime = task.endTime ? PulseApp.utils.formatPbsTimestamp(task.endTime) : 'N/A';
        const exitCodeDisplay = task.exitCode !== undefined ? task.exitCode : 'N/A';
        const exitCodeClass = task.exitCode !== undefined && task.exitCode !== 0 ? 'text-red-600 dark:text-red-400 font-semibold' : '';
        
        infoSection.innerHTML = `
            <div class="grid grid-cols-1 gap-2">
                <div><strong>Task Type:</strong> ${task.type || 'N/A'}</div>
                <div><strong>Node:</strong> ${task.node || 'N/A'}</div>
                <div><strong>User:</strong> ${task.user || 'N/A'}</div>
                <div><strong>Start Time:</strong> ${task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A'}</div>
                <div><strong>End Time:</strong> ${endTime}</div>
                <div><strong>Exit Code:</strong> <span class="${exitCodeClass}">${exitCodeDisplay}</span></div>
                <div><strong>Full UPID:</strong> <span class="font-mono break-all">${task.upid || 'N/A'}</span></div>
            </div>
        `;
        
        detailContent.appendChild(infoSection);
        detailCard.appendChild(detailContent);
        
        return detailCard;
    };

    const _createTaskTableRow = (task) => {
        const target = parsePbsTaskTarget(task);
        const statusDisplayHTML = getPbsStatusDisplay(task.status);
        const startTime = task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A';
        const duration = task.duration !== null ? PulseApp.utils.formatDuration(task.duration) : 'N/A';
        const upid = task.upid || 'N/A';
        const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

        const row = document.createElement('tr');
        
        // Add UPID as data attribute for tracking expanded state
        row.dataset.upid = task.upid || '';
        
        // Add status-based row styling for better problem visibility
        let rowClasses = `${CSS_CLASSES.BORDER_B_GRAY_200_DARK_GRAY_700} ${CSS_CLASSES.TRANSITION_COLORS} ${CSS_CLASSES.DURATION_150} ${CSS_CLASSES.EASE_IN_OUT}`;
        
        const isFailed = task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running');
        
        if (isFailed) {
            // Failed tasks get red background and cursor pointer if expandable
            rowClasses += ` bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 cursor-pointer`;
        } else if (task.status && task.status.toLowerCase().includes('running')) {
            // Running tasks get blue background
            rowClasses += ` bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30`;
        } else {
            // Normal hover for successful tasks
            rowClasses += ` hover:bg-gray-50 dark:hover:bg-gray-700`;
        }
        
        row.className = rowClasses;

        const targetCell = document.createElement('td');
        // Determine sticky cell background based on task status
        let stickyBg = 'bg-white dark:bg-gray-800';
        if (isFailed) {
            stickyBg = 'bg-red-50 dark:bg-red-900/20';
        } else if (task.status && task.status.toLowerCase().includes('running')) {
            stickyBg = 'bg-blue-50 dark:bg-blue-900/20';
        }
        targetCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300} sticky left-0 ${stickyBg} z-10 border-b border-gray-200 dark:border-gray-700`;
        
        // Add expand indicator for failed tasks
        if (isFailed) {
            targetCell.innerHTML = `${target}`;
        } else {
            targetCell.textContent = target;
        }
        row.appendChild(targetCell);

        const statusCell = document.createElement('td');
        statusCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} min-w-48`;
        statusCell.innerHTML = statusDisplayHTML;
        row.appendChild(statusCell);

        const namespaceCell = document.createElement('td');
        namespaceCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.WHITESPACE_NOWRAP}`;
        const namespaceText = task.namespace === 'root' ? 'Root' : (task.namespace || 'Root');
        namespaceCell.textContent = namespaceText;
        row.appendChild(namespaceCell);

        const startTimeCell = document.createElement('td');
        startTimeCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.WHITESPACE_NOWRAP}`;
        startTimeCell.textContent = startTime;
        row.appendChild(startTimeCell);

        const durationCell = document.createElement('td');
        durationCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.WHITESPACE_NOWRAP}`;
        durationCell.textContent = duration;
        row.appendChild(durationCell);

        const upidCell = document.createElement('td');
        upidCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MONO} ${CSS_CLASSES.TEXT_GRAY_400} dark:text-gray-500 ${CSS_CLASSES.TRUNCATE}`;
        upidCell.title = upid;
        upidCell.textContent = shortUpid;
        row.appendChild(upidCell);

        // Add click handler for failed tasks to show details
        if (isFailed && !row.dataset.clickHandlerAttached) {
            row.addEventListener('click', (event) => {
                event.stopPropagation(); // Prevent event bubbling
                
                const upid = task.upid;
                const existingDetailRow = row.nextElementSibling;
                
                if (existingDetailRow && existingDetailRow.classList.contains('task-detail-row')) {
                    // Toggle existing detail row - collapse
                    existingDetailRow.remove();
                    targetCell.innerHTML = `${target}`;
                    expandedTaskState.delete(upid); // Remove from global state
                } else {
                    // Create and show detail row - expand
                    const detailRow = _createTaskDetailRow(task);
                    row.insertAdjacentElement('afterend', detailRow);
                    targetCell.innerHTML = `${target}`;
                    expandedTaskState.add(upid); // Add to global state
                }
            });
            
            // Mark row as having click handler to prevent duplicates
            row.dataset.clickHandlerAttached = 'true';
        }

        return row;
    };

    const _createTaskDetailRow = (task) => {
        const detailRow = document.createElement('tr');
        detailRow.className = 'task-detail-row bg-red-25 dark:bg-red-950/10 border-b border-gray-200 dark:border-gray-700';
        
        const detailCell = document.createElement('td');
        detailCell.colSpan = 6;
        detailCell.className = 'px-6 py-4 bg-red-25 dark:bg-red-950/10';
        
        const detailContent = document.createElement('div');
        detailContent.className = 'space-y-2 text-sm';
        
        // Error details section
        const errorSection = document.createElement('div');
        errorSection.innerHTML = `
            <div class="font-semibold text-red-700 dark:text-red-300 mb-2">Error Details:</div>
            <div class="bg-gray-100 dark:bg-gray-800 p-3 rounded font-mono text-xs break-all">
                ${task.status || 'No error message available'}
            </div>
        `;
        detailContent.appendChild(errorSection);
        
        // Task info section
        const infoSection = document.createElement('div');
        infoSection.className = 'grid grid-cols-1 md:grid-cols-2 gap-4 text-xs text-gray-600 dark:text-gray-400';
        
        const leftInfo = document.createElement('div');
        leftInfo.innerHTML = `
            <div><strong>Task Type:</strong> ${task.type || 'N/A'}</div>
            <div><strong>Node:</strong> ${task.node || 'N/A'}</div>
            <div><strong>Worker ID:</strong> ${task.id || 'N/A'}</div>
            <div><strong>User:</strong> ${task.user || 'N/A'}</div>
        `;
        
        const rightInfo = document.createElement('div');
        const endTime = task.endTime ? PulseApp.utils.formatPbsTimestamp(task.endTime) : 'N/A';
        const exitCodeDisplay = task.exitCode !== undefined ? task.exitCode : 'N/A';
        const exitCodeClass = task.exitCode !== undefined && task.exitCode !== 0 ? 'text-red-600 dark:text-red-400 font-semibold' : '';
        
        rightInfo.innerHTML = `
            <div><strong>Start Time:</strong> ${task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A'}</div>
            <div><strong>End Time:</strong> ${endTime}</div>
            <div><strong>Exit Code:</strong> <span class="${exitCodeClass}">${exitCodeDisplay}</span></div>
            <div><strong>Full UPID:</strong> <span class="font-mono break-all">${task.upid || 'N/A'}</span></div>
        `;
        
        infoSection.appendChild(leftInfo);
        infoSection.appendChild(rightInfo);
        detailContent.appendChild(infoSection);
        
        detailCell.appendChild(detailContent);
        detailRow.appendChild(detailCell);
        
        return detailRow;
    };

    function populatePbsTaskTable(parentSectionElement, fullTasksArray) {
      if (!parentSectionElement) {
          console.warn('[PBS UI] Parent element not found for task table');
          return;
      }
      const tableBody = parentSectionElement.querySelector('tbody');
      const showMoreButton = parentSectionElement.querySelector(`.${CSS_CLASSES.PBS_SHOW_MORE}`);
      const noTasksMessage = parentSectionElement.querySelector(`.${CSS_CLASSES.PBS_NO_TASKS}`);
      const initialLimit = PulseApp.config.INITIAL_PBS_TASK_LIMIT;

      if (!tableBody) {
          console.warn('[PBS UI] Table body not found within', parentSectionElement);
          return;
      }

      const table = tableBody.closest('table');
      const tableId = table?.id;
      const isShowMoreExpanded = tableId ? expandedShowMoreState.has(tableId) : false;
      
      // Find the scrollable container
      const scrollableContainer = PulseApp.utils.getScrollableParent(tableBody) || 
                                 parentSectionElement.closest('.overflow-x-auto') ||
                                 parentSectionElement;

      // Store current scroll position for both axes
      const currentScrollLeft = scrollableContainer.scrollLeft || 0;
      const currentScrollTop = scrollableContainer.scrollTop || 0;

      // Use global expanded state instead of scanning DOM
      PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
          tableBody.innerHTML = '';

      const tasks = fullTasksArray || [];
      
      // Separate failed and successful tasks
      const failedTasks = tasks.filter(task => task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running'));
      const successfulTasks = tasks.filter(task => !task.status || task.status === 'OK' || task.status.toLowerCase().includes('running'));
      
      // Always show failed tasks first, then successful tasks up to limit
      const prioritizedTasks = [...failedTasks, ...successfulTasks];
      
      // Determine which tasks to display based on "Show More" state
      let displayedTasks;
      if (isShowMoreExpanded) {
          // If "Show More" was previously expanded, show all tasks
          displayedTasks = prioritizedTasks;
      } else {
          // Otherwise, show limited tasks
          displayedTasks = prioritizedTasks.slice(0, initialLimit);
      }

      // Update status indicator in header
      if (tableId) {
          const statusSpan = document.getElementById(`${tableId}-status`);
          const prioritySpan = document.getElementById(`${tableId}-priority`);
          
          if (statusSpan) {
              const runningTasks = tasks.filter(task => task.status && task.status.toLowerCase().includes('running')).length;
              
              let statusText = `(${tasks.length} total`;
              if (failedTasks.length > 0) {
                  statusText += `, <span class="text-red-600 dark:text-red-400 font-semibold">${failedTasks.length} failed</span>`;
              }
              if (runningTasks > 0) {
                  statusText += `, <span class="text-blue-600 dark:text-blue-400">${runningTasks} running</span>`;
              }
              statusText += ')';
              
              statusSpan.innerHTML = statusText;
          }
          
          // Show priority indicator if failed tasks exist and are being prioritized
          if (prioritySpan) {
              if (failedTasks.length > 0) {
                  prioritySpan.classList.remove('hidden');
              } else {
                  prioritySpan.classList.add('hidden');
              }
          }
      }

      if (tasks.length === 0) {
          if (noTasksMessage) noTasksMessage.classList.remove(CSS_CLASSES.HIDDEN);
          if (showMoreButton) showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
      } else {
          if (noTasksMessage) noTasksMessage.classList.add(CSS_CLASSES.HIDDEN);

          displayedTasks.forEach(task => {
              const taskRow = _createTaskTableRow(task);
              tableBody.appendChild(taskRow);
              
              // Restore expanded state if this task was previously expanded
              if (expandedTaskState.has(task.upid)) {
                  const targetCell = taskRow.querySelector('td:first-child');
                  const target = parsePbsTaskTarget(task);
                  const detailRow = _createTaskDetailRow(task);
                  taskRow.insertAdjacentElement('afterend', detailRow);
                  targetCell.innerHTML = `${target}`;
              }
          });

          if (showMoreButton) {
              if (prioritizedTasks.length > initialLimit) {
                  showMoreButton.classList.remove(CSS_CLASSES.HIDDEN);
                  
                  // Update button text and state based on current expansion
                  if (isShowMoreExpanded) {
                      showMoreButton.textContent = 'Show Less';
                  } else {
                      const remainingCount = prioritizedTasks.length - initialLimit;
                      showMoreButton.textContent = `Show More (${remainingCount} older)`;
                  }
                  
                  // Use proper event cleanup instead of DOM replacement
                  if (!showMoreButton.dataset.handlerAttached) {
                      showMoreButton.dataset.handlerAttached = 'true';
                      showMoreButton.addEventListener('click', () => {
                          if (tableId) {
                              if (expandedShowMoreState.has(tableId)) {
                                  // Currently expanded, collapse it
                                  expandedShowMoreState.delete(tableId);
                              } else {
                                  // Currently collapsed, expand it
                                  expandedShowMoreState.add(tableId);
                              }
                              // Re-populate with new state
                              populatePbsTaskTable(parentSectionElement, fullTasksArray);
                          }
                      });
                  }
              } else {
                  showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
                  // Remove from state if there are no more tasks to show
                  if (tableId && expandedShowMoreState.has(tableId)) {
                      expandedShowMoreState.delete(tableId);
                  }
              }
          }
      }
      }); // End of preserveScrollPosition
      
      // Additional scroll position restoration for horizontal scrolling
      if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
          requestAnimationFrame(() => {
              scrollableContainer.scrollLeft = currentScrollLeft;
              scrollableContainer.scrollTop = currentScrollTop;
          });
      }
  }

    // Mobile-friendly datastore card component
    const _createMobileDatastoreCard = (ds) => {
        const totalBytes = ds.total || 0;
        const usedBytes = ds.used || 0;
        const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0);
        const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
        const usageColor = PulseApp.utils.getUsageColor(usagePercent);
        const gcStatusHtml = getPbsGcStatusText(ds.gcStatus);

        const card = document.createElement('div');
        card.className = `mobile-datastore-card p-4 border border-gray-200 dark:border-gray-700 rounded-lg mb-3 bg-white dark:bg-gray-800 transition-all duration-200`;
        
        // Add critical usage highlighting
        if (usagePercent >= 95) {
            card.classList.add('border-red-300', 'dark:border-red-600', 'bg-red-50', 'dark:bg-red-900/10');
        } else if (usagePercent >= 85) {
            card.classList.add('border-yellow-300', 'dark:border-yellow-600', 'bg-yellow-50', 'dark:bg-yellow-900/10');
        }

        // Create card header with name and usage
        const cardHeader = document.createElement('div');
        cardHeader.className = 'flex justify-between items-start mb-3';
        
        const nameElement = document.createElement('div');
        nameElement.className = 'font-medium text-sm flex-1 pr-2';
        
        let nameContent = ds.name || 'N/A';
        if (usagePercent >= 95) {
            nameElement.innerHTML = `<span class="text-red-700 dark:text-red-300">${nameContent}</span><div class="text-xs text-red-600 dark:text-red-400 font-normal mt-1">CRITICAL: ${usagePercent}% full</div>`;
        } else if (usagePercent >= 85) {
            nameElement.innerHTML = `<span class="text-yellow-700 dark:text-yellow-300">${nameContent}</span><div class="text-xs text-yellow-600 dark:text-yellow-400 font-normal mt-1">WARNING: ${usagePercent}% full</div>`;
        } else {
            nameElement.textContent = nameContent;
        }
        
        const usageElement = document.createElement('div');
        usageElement.className = 'text-right flex-shrink-0';
        usageElement.innerHTML = `
            <div class="text-lg font-semibold ${usageColor.replace('bg-', 'text-').replace('-500', '-600').replace('-400', '-500')}">${usagePercent}%</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">${PulseApp.utils.formatBytes(usedBytes)}</div>
        `;
        
        cardHeader.appendChild(nameElement);
        cardHeader.appendChild(usageElement);
        card.appendChild(cardHeader);

        // Progress bar
        const progressContainer = document.createElement('div');
        progressContainer.className = 'w-full bg-gray-200 dark:bg-gray-700 rounded-full h-3 mb-3';
        
        const progressBar = document.createElement('div');
        progressBar.className = `h-3 rounded-full transition-all duration-300 ${usageColor}`;
        progressBar.style.width = `${Math.min(usagePercent, 100)}%`;
        
        progressContainer.appendChild(progressBar);
        card.appendChild(progressContainer);

        // Storage details grid
        const detailsGrid = document.createElement('div');
        detailsGrid.className = 'grid grid-cols-2 gap-3 text-xs text-gray-600 dark:text-gray-400 mb-3';
        
        detailsGrid.innerHTML = `
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Used</div>
                <div class="truncate">${ds.used !== null ? PulseApp.utils.formatBytes(ds.used) : 'N/A'}</div>
            </div>
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Available</div>
                <div class="truncate">${ds.available !== null ? PulseApp.utils.formatBytes(ds.available) : 'N/A'}</div>
            </div>
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Total</div>
                <div class="truncate">${ds.total !== null ? PulseApp.utils.formatBytes(ds.total) : 'N/A'}</div>
            </div>
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Deduplication</div>
                <div class="truncate font-semibold">${ds.deduplicationFactor ? `${ds.deduplicationFactor}x` : 'N/A'}</div>
            </div>
        `;
        
        card.appendChild(detailsGrid);

        // Path and GC status
        const metaInfo = document.createElement('div');
        metaInfo.className = 'space-y-2 text-xs text-gray-500 dark:text-gray-500';
        
        metaInfo.innerHTML = `
            <div class="truncate"><span class="font-medium">Path:</span> ${ds.path || 'N/A'}</div>
            <div><span class="font-medium">GC Status:</span> ${gcStatusHtml}</div>
        `;
        
        card.appendChild(metaInfo);

        return card;
    };

    // Mobile container for datastore cards
    const _createMobileDatastoreContainer = (datastores, statusText, showDetails) => {
        const container = document.createElement('div');
        container.className = 'mobile-datastore-container space-y-3';
        
        if (showDetails && datastores) {
            if (datastores.length === 0) {
                const emptyCard = document.createElement('div');
                emptyCard.className = 'p-4 text-center text-gray-400 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-700';
                emptyCard.textContent = 'No PBS datastores found or accessible.';
                container.appendChild(emptyCard);
            } else {
                datastores.forEach(ds => {
                    const card = _createMobileDatastoreCard(ds);
                    container.appendChild(card);
                });
            }
        } else {
            const statusCard = document.createElement('div');
            statusCard.className = 'p-4 text-center text-gray-400 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-700';
            statusCard.textContent = statusText;
            container.appendChild(statusCard);
        }
        
        return container;
    };

    const _populateDsTableBody = (dsTableBody, datastores, statusText, showDetails) => {
        if (!dsTableBody) return;
        
        // Find the scrollable container
        const scrollableContainer = PulseApp.utils.getScrollableParent(dsTableBody) || 
                                   dsTableBody.closest('.overflow-x-auto') ||
                                   dsTableBody.parentElement;
        
        // Store current scroll position for both axes
        const currentScrollLeft = scrollableContainer.scrollLeft || 0;
        const currentScrollTop = scrollableContainer.scrollTop || 0;
        
        // Calculate dynamic column widths for responsive display
        if (showDetails && datastores && datastores.length > 0) {
            let maxNameLength = 0;
            let maxPathLength = 0;
            
            datastores.forEach(ds => {
                const nameLength = (ds.name || 'N/A').length;
                const pathLength = (ds.path || 'N/A').length;
                if (nameLength > maxNameLength) maxNameLength = nameLength;
                if (pathLength > maxPathLength) maxPathLength = pathLength;
            });
            
            // Set CSS variables for column widths with responsive limits
            const nameColWidth = Math.min(Math.max(maxNameLength * 7 + 12, 80), 200);
            const pathColWidth = Math.min(Math.max(maxPathLength * 7 + 12, 100), 250);
            const htmlElement = document.documentElement;
            if (htmlElement) {
                htmlElement.style.setProperty('--pbs-name-col-width', `${nameColWidth}px`);
                htmlElement.style.setProperty('--pbs-path-col-width', `${pathColWidth}px`);
            }
        }
        
        PulseApp.utils.preserveScrollPosition(scrollableContainer, () => {
            dsTableBody.innerHTML = '';

        if (showDetails && datastores) {
            if (datastores.length === 0) {
                const row = dsTableBody.insertRow();
                const cell = row.insertCell();
                cell.colSpan = 8;
                cell.className = `px-4 py-4 text-sm text-gray-400 text-center`;
                cell.textContent = 'No PBS datastores found or accessible.';
            } else {
                datastores.forEach(ds => {
                    const totalBytes = ds.total || 0;
                    const usedBytes = ds.used || 0;
                    const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0);
                    const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
                    const usageColor = PulseApp.utils.getUsageColor(usagePercent);
                    const usageText = totalBytes > 0 ? `${usagePercent}% (${PulseApp.utils.formatBytes(usedBytes)} of ${PulseApp.utils.formatBytes(totalBytes)})` : 'N/A';
                    const gcStatusHtml = getPbsGcStatusText(ds.gcStatus);

                    const row = dsTableBody.insertRow();
                    
                    // Add critical usage highlighting
                    let rowClass = `${CSS_CLASSES.HOVER_BG_GRAY_50_DARK_HOVER_BG_GRAY_700_50}`;
                    if (usagePercent >= 95) {
                        rowClass = 'bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30';
                    } else if (usagePercent >= 85) {
                        rowClass = 'bg-yellow-50 dark:bg-yellow-900/20 hover:bg-yellow-100 dark:hover:bg-yellow-900/30';
                    }
                    row.className = rowClass;

                    const createCell = (content, classNames = [], isHtml = false) => {
                        const cell = row.insertCell();
                        cell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.WHITESPACE_NOWRAP} ${classNames.join(' ')}`;
                        if (isHtml) {
                            cell.innerHTML = content;
                        } else {
                            cell.textContent = content;
                        }
                        return cell;
                    };

                    // Add usage alert to name if critical
                    let nameContent = ds.name || 'N/A';
                    if (usagePercent >= 95) {
                        nameContent = `${nameContent} [CRITICAL: ${usagePercent}% full]`;
                        createCell(nameContent, ['text-red-700', 'dark:text-red-300', 'font-semibold', 'sticky', 'left-0', 'bg-white', 'dark:bg-gray-800', 'z-10']);
                    } else if (usagePercent >= 85) {
                        nameContent = `${nameContent} [WARNING: ${usagePercent}% full]`;
                        createCell(nameContent, ['text-yellow-700', 'dark:text-yellow-300', 'font-semibold', 'sticky', 'left-0', 'bg-white', 'dark:bg-gray-800', 'z-10']);
                    } else {
                        createCell(nameContent, ['sticky', 'left-0', 'bg-white', 'dark:bg-gray-800', 'z-10']);
                    }

                    createCell(ds.path || 'N/A', [CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400, 'max-w-0', 'overflow-hidden', 'text-ellipsis']);
                    
                    // Simplified cell creation - the data is actually coming through correctly
                    createCell(ds.used !== null ? PulseApp.utils.formatBytes(ds.used) : 'N/A');
                    createCell(ds.available !== null ? PulseApp.utils.formatBytes(ds.available) : 'N/A');
                    createCell(ds.total !== null ? PulseApp.utils.formatBytes(ds.total) : 'N/A');

                    // Create usage cell with better formatting
                    const usageCell = row.insertCell();
                    usageCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.WHITESPACE_NOWRAP}`;
                    usageCell.style.minWidth = '150px';
                    if (totalBytes > 0) {
                        usageCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor, `${usagePercent}%`);
                    } else {
                        usageCell.textContent = 'N/A';
                    }

                    // Add deduplication factor column
                    const deduplicationText = ds.deduplicationFactor ? `${ds.deduplicationFactor}x` : 'N/A';
                    createCell(deduplicationText, [CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400, CSS_CLASSES.FONT_SEMIBOLD]);

                    createCell(gcStatusHtml, [], true);
                });
            }
        } else {
            const row = dsTableBody.insertRow();
            const cell = row.insertCell();
            cell.colSpan = 8;
            cell.className = `px-4 py-4 ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_400} text-center`;
            cell.textContent = statusText;
        }
        }); // End of preserveScrollPosition
        
        // Additional scroll position restoration for horizontal scrolling
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                scrollableContainer.scrollLeft = currentScrollLeft;
                scrollableContainer.scrollTop = currentScrollTop;
            });
        }
    };

    const _populateInstanceTaskSections = (detailsContainer, instanceId, pbsInstance, statusText, showDetails) => {
        const taskTypes = [
            { type: 'backup', data: pbsInstance.backupTasks, elementSuffix: ID_PREFIXES.PBS_RECENT_BACKUP_TASKS_TBODY + instanceId },
            { type: 'verify', data: pbsInstance.verificationTasks, elementSuffix: ID_PREFIXES.PBS_RECENT_VERIFY_TASKS_TBODY + instanceId },
            { type: 'sync', data: pbsInstance.syncTasks, elementSuffix: ID_PREFIXES.PBS_RECENT_SYNC_TASKS_TBODY + instanceId },
            { type: 'prunegc', data: pbsInstance.pruneTasks, elementSuffix: ID_PREFIXES.PBS_RECENT_PRUNEGC_TASKS_TBODY + instanceId }
        ];

        if (showDetails) {
            taskTypes.forEach(taskInfo => {
                const section = detailsContainer.querySelector(`.${CSS_CLASSES.PBS_TASK_SECTION}[${DATA_ATTRIBUTES.TASK_TYPE}="${taskInfo.type}"]`);
                if (section) populatePbsTaskTable(section, taskInfo.data?.recentTasks);
            });
        } else {
            taskTypes.forEach(taskInfo => {
                const tbody = document.getElementById(taskInfo.elementSuffix);
                if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_400} text-center">${statusText}</td></tr>`;
            });
        }
    };

    const _createHealthBadgeHTML = (health, title) => {
        let colorClass = CSS_CLASSES.BG_GRAY_400_DARK_BG_GRAY_500;
        if (health === 'ok') colorClass = CSS_CLASSES.BG_GREEN_500;
        else if (health === 'warning') colorClass = CSS_CLASSES.BG_YELLOW_500;
        else if (health === 'error') colorClass = CSS_CLASSES.BG_RED_500;
        const span = document.createElement('span');
        span.title = title;
        span.className = `${CSS_CLASSES.INLINE_BLOCK} ${CSS_CLASSES.W_3} ${CSS_CLASSES.H_3} ${colorClass} ${CSS_CLASSES.ROUNDED_FULL} ${CSS_CLASSES.MR2} ${CSS_CLASSES.FLEX_SHRINK_0}`;
        return span;
    };

    const _createInstanceHeaderDiv = (instanceName, overallHealth, healthTitle) => {
        const headerDiv = document.createElement('div');
        headerDiv.className = `${CSS_CLASSES.FLEX} ${CSS_CLASSES.JUSTIFY_BETWEEN} ${CSS_CLASSES.ITEMS_CENTER} ${CSS_CLASSES.MB3}`;
        const instanceTitleElement = document.createElement('h3');
        instanceTitleElement.className = `${CSS_CLASSES.TEXT_LG} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.TEXT_GRAY_800_DARK_GRAY_200} ${CSS_CLASSES.FLEX} ${CSS_CLASSES.ITEMS_CENTER}`;
        
        // Check if we can make this PBS instance name clickable
        const hostUrl = PulseApp.utils.getHostUrl(instanceName);
        if (hostUrl) {
            const linkElement = document.createElement('a');
            linkElement.href = hostUrl;
            linkElement.target = '_blank';
            linkElement.rel = 'noopener noreferrer';
            linkElement.className = 'text-gray-800 dark:text-gray-200 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer';
            linkElement.title = `Open ${instanceName} web interface`;
            linkElement.appendChild(document.createTextNode(instanceName));
            instanceTitleElement.appendChild(linkElement);
        } else {
            instanceTitleElement.appendChild(document.createTextNode(instanceName));
        }
        
        headerDiv.appendChild(instanceTitleElement);
        return headerDiv;
    };

    const _createDatastoreSectionElement = (instanceId) => {
        const dsSection = document.createElement('div');
        dsSection.id = ID_PREFIXES.PBS_DS_SECTION + instanceId;
        const dsHeading = document.createElement('h4');
        dsHeading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        dsHeading.textContent = 'Datastores';
        dsSection.appendChild(dsHeading);
        const dsTableContainer = document.createElement('div');
        dsTableContainer.className = `${CSS_CLASSES.OVERFLOW_X_AUTO} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED}`;
        const dsTable = document.createElement('table');
        dsTable.id = ID_PREFIXES.PBS_DS_TABLE + instanceId;
        dsTable.className = `${CSS_CLASSES.MIN_W_FULL} ${CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700} ${CSS_CLASSES.TEXT_SM}`;
        const dsThead = document.createElement('thead');
        dsThead.className = `${CSS_CLASSES.STICKY} ${CSS_CLASSES.TOP_0} ${CSS_CLASSES.Z_10} ${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_800}`;
        const dsHeaderRow = document.createElement('tr');
        dsHeaderRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TRACKING_WIDER} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;
        ['Name', 'Path', 'Used', 'Available', 'Total', 'Usage', 'Deduplication', 'GC Status'].forEach(headerText => {
            const th = document.createElement('th');
            th.scope = 'col';
            th.className = CSS_CLASSES.P1_PX2;
            th.textContent = headerText;
            dsHeaderRow.appendChild(th);
        });
        dsThead.appendChild(dsHeaderRow);
        dsTable.appendChild(dsThead);
        const dsTbody = document.createElement('tbody');
        dsTbody.id = ID_PREFIXES.PBS_DS_TBODY + instanceId;
        dsTbody.className = CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700;
        dsTable.appendChild(dsTbody);
        dsTableContainer.appendChild(dsTable);
        dsSection.appendChild(dsTableContainer);
        return dsSection;
    };

    // START: Definitions for functions that _createPbsInstanceElement depends on
    const _createPbsTaskHealthTable = (instanceId, pbsInstanceData) => {
        const sectionDiv = document.createElement('div');
        sectionDiv.id = ID_PREFIXES.PBS_SUMMARIES_SECTION + instanceId;
        sectionDiv.className = `${CSS_CLASSES.MB3}`;

        const heading = document.createElement('h4');
        heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        heading.textContent = 'PBS Task Summary';
        sectionDiv.appendChild(heading);

        const tableContainer = document.createElement('div');
        tableContainer.className = `${CSS_CLASSES.OVERFLOW_X_AUTO} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} ${CSS_CLASSES.BG_GRAY_50_DARK_BG_GRAY_800_30} p-3`;
        
        const table = document.createElement('table');
        table.className = `${CSS_CLASSES.MIN_W_FULL} ${CSS_CLASSES.TEXT_SM}`;
        
        const thead = document.createElement('thead');
        thead.className = `${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_800}`;
        const headerRow = document.createElement('tr');
        headerRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TRACKING_WIDER} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;

        const headers = ['Task Type', 'Status', 'Last Successful Run', 'Last Failure'];
        headers.forEach(text => {
            const th = document.createElement('th');
            th.scope = 'col';
            th.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_LEFT}`;
            th.textContent = text;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        table.appendChild(thead);

        const tbody = document.createElement('tbody');
        tbody.className = 'pbs-task-health-tbody';
        
        const taskHealthData = [
            { title: 'Backups', data: pbsInstanceData.backupTasks },
            { title: 'Verification', data: pbsInstanceData.verificationTasks },
            { title: 'Sync', data: pbsInstanceData.syncTasks },
            { title: 'Prune/GC', data: pbsInstanceData.pruneTasks }
        ];

        taskHealthData.forEach(taskItem => {
            const summary = taskItem.data?.summary || {};
            const ok = summary.ok ?? '-';
            const failed = summary.failed ?? 0;
            const lastOk = PulseApp.utils.formatPbsTimestamp(summary.lastOk);
            const lastFailed = PulseApp.utils.formatPbsTimestamp(summary.lastFailed);

            const row = tbody.insertRow();
            
            // Add row highlighting for failed tasks
            if (failed > 0) {
                row.className = `${CSS_CLASSES.BORDER_B_GRAY_200_DARK_GRAY_700} bg-red-50 dark:bg-red-900/20`;
            } else {
                row.className = CSS_CLASSES.BORDER_B_GRAY_200_DARK_GRAY_700;
            }

            const cellTaskType = row.insertCell();
            cellTaskType.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.TEXT_GRAY_800_DARK_GRAY_200}`;
            cellTaskType.textContent = taskItem.title;

            const cellStatus = row.insertCell();
            cellStatus.className = CSS_CLASSES.P1_PX2;
            
            // More descriptive status text
            let statusHtml = '';
            if (failed > 0) {
                statusHtml = `<span class="${CSS_CLASSES.TEXT_RED_600_DARK_RED_400} ${CSS_CLASSES.FONT_BOLD}">${failed} FAILED</span>`;
                if (ok > 0) {
                    statusHtml += ` / <span class="${CSS_CLASSES.TEXT_GREEN_600_DARK_GREEN_400}">${ok} OK</span>`;
                }
            } else if (ok > 0) {
                statusHtml = `<span class="${CSS_CLASSES.TEXT_GREEN_600_DARK_GREEN_400}">All OK (${ok})</span>`;
            } else {
                statusHtml = `<span class="${CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400}">No recent tasks</span>`;
            }
            cellStatus.innerHTML = statusHtml;
            
            const cellLastOk = row.insertCell();
            cellLastOk.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400}`;
            cellLastOk.textContent = lastOk;

            const cellLastFail = row.insertCell();
            cellLastFail.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400}`;
            if (failed > 0 && lastFailed !== '-') {
                cellLastFail.innerHTML = `<span class="text-red-600 dark:text-red-400 font-semibold">${lastFailed}</span>`;
            } else {
                cellLastFail.textContent = lastFailed;
            }
        });

        table.appendChild(tbody);
        tableContainer.appendChild(table);
        sectionDiv.appendChild(tableContainer);
        
        return sectionDiv;
    };

    const _createTaskTableElement = (tableId, title, idColumnHeader) => {
        const fragment = document.createDocumentFragment();

        const heading = document.createElement('h4');
        heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        heading.innerHTML = `Recent ${title} Tasks <span id="${tableId}-status" class="text-xs font-normal text-gray-500"></span><span id="${tableId}-priority" class="text-xs text-red-600 dark:text-red-400 ml-2 hidden">Failed tasks shown first</span>`;
        fragment.appendChild(heading);

        const tableContainer = document.createElement('div');
        tableContainer.className = `${CSS_CLASSES.OVERFLOW_X_AUTO} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED}`;

        const table = document.createElement('table');
        table.id = tableId;
        table.className = `${CSS_CLASSES.MIN_W_FULL} ${CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700} ${CSS_CLASSES.TEXT_SM}`;

        const thead = document.createElement('thead');
        thead.className = `${CSS_CLASSES.STICKY} ${CSS_CLASSES.TOP_0} ${CSS_CLASSES.Z_10} ${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_800}`;
        const headerRow = document.createElement('tr');
        headerRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TRACKING_WIDER} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;

        const headers = [idColumnHeader, 'Status', 'Namespace', 'Start Time', 'Duration', 'UPID'];
        headers.forEach(text => {
            const th = document.createElement('th');
            th.scope = 'col';
            th.className = CSS_CLASSES.P1_PX2;
            th.textContent = text;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        table.appendChild(thead);

        const tbody = document.createElement('tbody');
        tbody.id = tableId.replace('-table-', '-tbody-');
        tbody.className = `${CSS_CLASSES.PBS_TASK_TBODY} ${CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700}`;
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        fragment.appendChild(tableContainer);

        const toggleButtonContainer = document.createElement('div');
        toggleButtonContainer.id = tableId.replace('-table', '-toggle-container');
        toggleButtonContainer.className = `${CSS_CLASSES.PT1} ${CSS_CLASSES.TEXT_RIGHT}`;

        const showMoreButton = document.createElement('button');
        showMoreButton.className = `${CSS_CLASSES.PBS_SHOW_MORE} ${CSS_CLASSES.TEXT_BLUE_600_HOVER_BLUE_800_DARK_BLUE_400_DARK_HOVER_BLUE_300} ${CSS_CLASSES.HIDDEN}`;
        showMoreButton.textContent = 'Show More';
        toggleButtonContainer.appendChild(showMoreButton);

        const noTasksMessage = document.createElement('p');
        noTasksMessage.className = `${CSS_CLASSES.PBS_NO_TASKS} ${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.TEXT_GRAY_400} dark:text-gray-500 ${CSS_CLASSES.HIDDEN} ${CSS_CLASSES.ITALIC}`;
        noTasksMessage.textContent = 'No recent tasks found.';
        toggleButtonContainer.appendChild(noTasksMessage);

        fragment.appendChild(toggleButtonContainer);
        return fragment;
    };

    const _createSummariesSectionElement = (instanceId, pbsInstanceData) => {
        return _createPbsTaskHealthTable(instanceId, pbsInstanceData);
    };

    const _createAllTaskSectionsContainer = (instanceId) => {
        const container = document.createElement('div');
        container.className = CSS_CLASSES.SPACE_Y_4;

        const taskDefinitions = [
            { type: 'backup', title: 'Backup', idCol: 'Guest', tableIdPrefix: ID_PREFIXES.PBS_RECENT_BACKUP_TASKS_TABLE },
            { type: 'verify', title: 'Verification', idCol: 'Guest/Group', tableIdPrefix: ID_PREFIXES.PBS_RECENT_VERIFY_TASKS_TABLE },
            { type: 'sync', title: 'Sync', idCol: 'Job ID', tableIdPrefix: ID_PREFIXES.PBS_RECENT_SYNC_TASKS_TABLE },
            { type: 'prunegc', title: 'Prune/GC', idCol: 'Datastore/Group', tableIdPrefix: ID_PREFIXES.PBS_RECENT_PRUNEGC_TASKS_TABLE }
        ];

        taskDefinitions.forEach(def => {
            const taskSection = document.createElement('div');
            taskSection.className = CSS_CLASSES.PBS_TASK_SECTION;
            taskSection.dataset.taskType = def.type;
            taskSection.appendChild(_createTaskTableElement(def.tableIdPrefix + instanceId, def.title, def.idCol));
            container.appendChild(taskSection);
        });
        return container;
    };

    const _createPbsNodeStatusSection = (instanceId, pbsInstanceData) => {
        const sectionDiv = document.createElement('div');
        sectionDiv.id = `pbs-node-status-section-${instanceId}`;
        sectionDiv.className = `${CSS_CLASSES.MB3}`;

        const heading = document.createElement('h4');
        heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        heading.textContent = 'Server Status';
        sectionDiv.appendChild(heading);

        const tableContainer = document.createElement('div');
        tableContainer.className = `${CSS_CLASSES.OVERFLOW_X_AUTO} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED}`;
        const table = document.createElement('table');
        table.className = `${CSS_CLASSES.MIN_W_FULL} ${CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700} ${CSS_CLASSES.TEXT_SM}`;

        // Table header
        const thead = document.createElement('thead');
        thead.className = `${CSS_CLASSES.STICKY} ${CSS_CLASSES.TOP_0} ${CSS_CLASSES.Z_10} ${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_800}`;
        const headerRow = document.createElement('tr');
        headerRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TRACKING_WIDER} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;
        const headerClasses = [
            'w-24 truncate', // PBS VER
            'w-24 truncate', // Uptime
            'min-w-[180px]', // CPU
            'min-w-[180px]', // Mem
            'w-16 truncate'  // Load
        ];
        ['PBS VER', 'Uptime', 'CPU', 'Mem', 'Load'].forEach((headerText, i) => {
            const th = document.createElement('th');
            th.scope = 'col';
            th.className = `${CSS_CLASSES.P1_PX2} ${headerClasses[i]}`;
            th.textContent = headerText;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        table.appendChild(thead);

        // Table body
        const tbody = document.createElement('tbody');
        tbody.className = CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700;
        const valueRow = document.createElement('tr');

        // PBS VER
        const versionInfo = pbsInstanceData.versionInfo || {};
        const versionText = versionInfo.version ? (versionInfo.release ? `${versionInfo.version}-${versionInfo.release}` : versionInfo.version) : '-';
        const versionCell = document.createElement('td');
        versionCell.className = `${CSS_CLASSES.P1_PX2} w-24 truncate`;
        versionCell.title = versionText;
        versionCell.textContent = versionText;
        valueRow.appendChild(versionCell);

        // Uptime
        const nodeStatus = pbsInstanceData.nodeStatus || {};
        const uptimeCell = document.createElement('td');
        uptimeCell.className = `${CSS_CLASSES.P1_PX2} w-24 truncate`;
        uptimeCell.textContent = nodeStatus.uptime ? PulseApp.utils.formatUptime(nodeStatus.uptime) : '-';
        valueRow.appendChild(uptimeCell);

        // CPU
        const cpuCell = document.createElement('td');
        cpuCell.className = `${CSS_CLASSES.P1_PX2} min-w-[180px]`;
        if (nodeStatus.cpu !== null && nodeStatus.cpu !== undefined) {
            const cpuPercent = nodeStatus.cpu * 100;
            const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
            const cpuTooltipText = `${cpuPercent.toFixed(1)}%`;
            cpuCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(cpuPercent, cpuTooltipText, cpuColorClass);
        } else {
            cpuCell.textContent = '-';
        }
        valueRow.appendChild(cpuCell);

        // Mem
        const memCell = document.createElement('td');
        memCell.className = `${CSS_CLASSES.P1_PX2} min-w-[180px]`;
        if (nodeStatus.memory && nodeStatus.memory.total && nodeStatus.memory.used !== null) {
            const memUsed = nodeStatus.memory.used;
            const memTotal = nodeStatus.memory.total;
            const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
            const memColorClass = PulseApp.utils.getUsageColor(memPercent, 'memory');
            const memTooltipText = `${PulseApp.utils.formatBytes(memUsed)} / ${PulseApp.utils.formatBytes(memTotal)} (${memPercent.toFixed(1)}%)`;
            memCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(memPercent, memTooltipText, memColorClass);
        } else {
            memCell.textContent = '-';
        }
        valueRow.appendChild(memCell);

        // Load
        const loadCell = document.createElement('td');
        loadCell.className = `${CSS_CLASSES.P1_PX2} w-16 truncate`;
        if (nodeStatus.loadavg && Array.isArray(nodeStatus.loadavg) && nodeStatus.loadavg.length >= 1) {
            loadCell.textContent = nodeStatus.loadavg[0].toFixed(2);
        } else {
            loadCell.textContent = '-';
        }
        valueRow.appendChild(loadCell);

        tbody.appendChild(valueRow);
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        sectionDiv.appendChild(tableContainer);
        return sectionDiv;
    };

    const _createPbsInstanceElement = (pbsInstanceData, instanceId, instanceName, overallHealth, healthTitle, showDetails, statusText) => {
        const instanceWrapper = document.createElement('div');
        instanceWrapper.className = `${CSS_CLASSES.PBS_INSTANCE_SECTION} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} p-4 mb-4 bg-gray-50/30 dark:bg-gray-800/30`;
        instanceWrapper.id = ID_PREFIXES.PBS_INSTANCE + instanceId;

        instanceWrapper.appendChild(_createInstanceHeaderDiv(instanceName, overallHealth, healthTitle));

        const detailsContainer = document.createElement('div');
        const isError = pbsInstanceData.status === 'error';
        detailsContainer.className = `${CSS_CLASSES.PBS_INSTANCE_DETAILS} ${CSS_CLASSES.SPACE_Y_4} ${(showDetails || isError) ? '' : CSS_CLASSES.HIDDEN}`;
        detailsContainer.id = ID_PREFIXES.PBS_DETAILS + instanceId;

        if (isError) {
            // If there's an error, just show the statusText prominently
            const errorNoticeElement = document.createElement('div');
            errorNoticeElement.className = 'p-4 text-center text-red-600 dark:text-red-400 font-semibold';
            errorNoticeElement.textContent = statusText; // e.g., "Error: Connection failed"
            detailsContainer.appendChild(errorNoticeElement);
        } else if (showDetails) {
            // If no error and showDetails is true, build and populate the full UI
            detailsContainer.appendChild(_createPbsNodeStatusSection(instanceId, pbsInstanceData));
            detailsContainer.appendChild(_createDatastoreSectionElement(instanceId));
            detailsContainer.appendChild(_createSummariesSectionElement(instanceId, pbsInstanceData));
            detailsContainer.appendChild(_createAllTaskSectionsContainer(instanceId));
            
            // Populate the created sections
            const dsTableBodyElement = detailsContainer.querySelector(`#${ID_PREFIXES.PBS_DS_TBODY}${instanceId}`);
            _populateDsTableBody(dsTableBodyElement, pbsInstanceData.datastores, statusText, showDetails);
            _populateInstanceTaskSections(detailsContainer, instanceId, pbsInstanceData, statusText, showDetails);
        } else {
            // Fallback for non-error, non-detailed view (e.g., "Configured, attempting connection...")
            const statusNoticeElement = document.createElement('div');
            statusNoticeElement.className = 'p-4 text-center text-gray-500 dark:text-gray-400';
            statusNoticeElement.textContent = statusText;
            detailsContainer.appendChild(statusNoticeElement);
        }
        
        instanceWrapper.appendChild(detailsContainer);
        return instanceWrapper;
    };
    // END: Definitions for functions that _createPbsInstanceElement depends on

    // Mobile summary cards component
    const _createMobilePbsSummary = (pbsInstance) => {
        const summary = document.createElement('div');
        summary.className = 'mobile-pbs-summary grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4';
        
        // Calculate task summary
        const taskTypes = [
            pbsInstance.backupTasks,
            pbsInstance.verificationTasks,
            pbsInstance.syncTasks,
            pbsInstance.pruneTasks
        ];
        
        let totalTasks = 0;
        let failedTasks = 0;
        let runningTasks = 0;
        
        taskTypes.forEach(taskGroup => {
            if (taskGroup?.summary) {
                totalTasks += (taskGroup.summary.ok || 0) + (taskGroup.summary.failed || 0);
                failedTasks += taskGroup.summary.failed || 0;
            }
            if (taskGroup?.recentTasks) {
                runningTasks += taskGroup.recentTasks.filter(task => 
                    task.status && task.status.toLowerCase().includes('running')
                ).length;
            }
        });
        
        const datastoreCount = pbsInstance.datastores?.length || 0;
        const criticalDatastores = (pbsInstance.datastores || []).filter(ds => {
            const usagePercent = ds.total > 0 ? Math.round((ds.used / ds.total) * 100) : 0;
            return usagePercent >= 95;
        }).length;
        
        // Server status
        const nodeStatus = pbsInstance.nodeStatus || {};
        const isServerHealthy = pbsInstance.status === 'ok';
        
        summary.innerHTML = `
            <div class="summary-card p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg text-center border border-gray-200 dark:border-gray-600">
                <div class="text-lg font-semibold ${failedTasks > 0 ? 'text-red-600 dark:text-red-400' : runningTasks > 0 ? 'text-blue-600 dark:text-blue-400' : 'text-green-600 dark:text-green-400'}">${totalTasks}</div>
                <div class="text-xs text-gray-600 dark:text-gray-400">Tasks</div>
                ${failedTasks > 0 ? `<div class="text-xs text-red-600 dark:text-red-400 font-medium">${failedTasks} failed</div>` : ''}
                ${runningTasks > 0 ? `<div class="text-xs text-blue-600 dark:text-blue-400 font-medium">${runningTasks} running</div>` : ''}
            </div>
            <div class="summary-card p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg text-center border border-gray-200 dark:border-gray-600">
                <div class="text-lg font-semibold ${criticalDatastores > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-700 dark:text-gray-300'}">${datastoreCount}</div>
                <div class="text-xs text-gray-600 dark:text-gray-400">Datastores</div>
                ${criticalDatastores > 0 ? `<div class="text-xs text-red-600 dark:text-red-400 font-medium">${criticalDatastores} critical</div>` : ''}
            </div>
            <div class="summary-card p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg text-center border border-gray-200 dark:border-gray-600">
                <div class="text-lg font-semibold ${isServerHealthy ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}">
                    ${isServerHealthy ? 'OK' : 'ERROR'}
                </div>
                <div class="text-xs text-gray-600 dark:text-gray-400">Server</div>
                <div class="text-xs ${isServerHealthy ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}">${isServerHealthy ? 'Online' : 'Error'}</div>
            </div>
            <div class="summary-card p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg text-center border border-gray-200 dark:border-gray-600">
                <div class="text-lg font-semibold text-gray-700 dark:text-gray-300">
                    ${nodeStatus.uptime ? PulseApp.utils.formatUptime(nodeStatus.uptime) : '-'}
                </div>
                <div class="text-xs text-gray-600 dark:text-gray-400">Uptime</div>
            </div>
        `;
        
        return summary;
    };

    // Mobile accordion component for PBS instances
    const _createMobileInstanceAccordion = (pbsArray, mainContainer) => {
        const accordionContainer = document.createElement('div');
        accordionContainer.className = 'mobile-pbs-accordion space-y-3';
        
        pbsArray.forEach((pbsInstance, index) => {
            let baseId;
            if (pbsInstance.pbsInstanceName) {
                baseId = PulseApp.utils.sanitizeForId(pbsInstance.pbsInstanceName);
            } else if (pbsInstance.pbsEndpointId) {
                baseId = PulseApp.utils.sanitizeForId(pbsInstance.pbsEndpointId);
            } else {
                baseId = 'pbs-instance';
            }
            const instanceId = `${baseId}-${index}`;
            const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
            
            const accordion = document.createElement('div');
            accordion.className = 'border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden';
            
            // Accordion header
            const header = document.createElement('button');
            header.className = 'w-full p-4 text-left bg-gray-50 dark:bg-gray-700/50 flex justify-between items-center tap-target hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors';
            header.dataset.instanceIndex = index;
            
            const headerContent = document.createElement('div');
            headerContent.className = 'flex-1';
            
            // Instance name and status
            const overallHealthAndTitle = _calculateOverallHealth(pbsInstance);
            const statusInfo = _getInstanceStatusInfo(pbsInstance);
            
            let statusClass = 'text-green-600 dark:text-green-400';
            
            if (pbsInstance.status === 'error') {
                statusClass = 'text-red-600 dark:text-red-400';
            } else if (pbsInstance.status !== 'ok') {
                statusClass = 'text-yellow-600 dark:text-yellow-400';
            }
            
            // Check if we can make this PBS instance name clickable
            const hostUrl = PulseApp.utils.getHostUrl(instanceName);
            let instanceNameHtml = `<span class="font-medium text-sm">${instanceName}</span>`;
            
            if (hostUrl) {
                instanceNameHtml = `<a href="${hostUrl}" target="_blank" rel="noopener noreferrer" class="font-medium text-sm text-gray-700 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer" title="Open ${instanceName} web interface">${instanceName}</a>`;
            }
            
            headerContent.innerHTML = `
                <div class="flex items-center gap-2 mb-1">
                    ${instanceNameHtml}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400 truncate">${statusInfo.statusText}</div>
            `;
            
            const chevron = document.createElement('span');
            chevron.className = 'text-gray-400 transition-transform duration-200';
            chevron.innerHTML = '';
            
            header.appendChild(headerContent);
            header.appendChild(chevron);
            
            // Accordion content
            const content = document.createElement('div');
            content.className = 'hidden border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800';
            content.dataset.instanceId = instanceId;
            
            // Toggle functionality
            header.addEventListener('click', () => {
                const isExpanded = !content.classList.contains('hidden');
                
                if (isExpanded) {
                    // Collapse
                    content.classList.add('hidden');
                    chevron.style.transform = 'rotate(0deg)';
                } else {
                    // Expand
                    content.classList.remove('hidden');
                    chevron.style.transform = 'rotate(180deg)';
                    
                    // Populate content if not already done
                    if (content.children.length === 0) {
                        const contentDiv = document.createElement('div');
                        contentDiv.className = 'p-4 space-y-4';
                        
                        if (statusInfo.showDetails) {
                            // Add mobile summary
                            contentDiv.appendChild(_createMobilePbsSummary(pbsInstance));
                            
                            // Add sections based on mobile layout
                            contentDiv.appendChild(_createMobileNodeStatusSection(instanceId, pbsInstance));
                            contentDiv.appendChild(_createMobileDatastoreSection(instanceId, pbsInstance, statusInfo));
                            contentDiv.appendChild(_createMobileTaskSections(instanceId, pbsInstance, statusInfo));
                        } else {
                            const statusNotice = document.createElement('div');
                            statusNotice.className = 'p-4 text-center text-gray-500 dark:text-gray-400';
                            statusNotice.textContent = statusInfo.statusText;
                            contentDiv.appendChild(statusNotice);
                        }
                        
                        content.appendChild(contentDiv);
                    }
                }
            });
            
            accordion.appendChild(header);
            accordion.appendChild(content);
            accordionContainer.appendChild(accordion);
        });
        
        mainContainer.appendChild(accordionContainer);
    };

    // Mobile sections
    const _createMobileNodeStatusSection = (instanceId, pbsInstance) => {
        const section = document.createElement('div');
        section.className = 'mobile-node-status space-y-2';
        
        const heading = document.createElement('h4');
        heading.className = 'text-sm font-semibold text-gray-700 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700 pb-1';
        heading.textContent = 'Server Status';
        section.appendChild(heading);
        
        const nodeStatus = pbsInstance.nodeStatus || {};
        const versionInfo = pbsInstance.versionInfo || {};
        
        const grid = document.createElement('div');
        grid.className = 'grid grid-cols-2 gap-3 text-xs';
        
        grid.innerHTML = `
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Version</div>
                <div class="text-gray-600 dark:text-gray-400">${versionInfo.version ? (versionInfo.release ? `${versionInfo.version}-${versionInfo.release}` : versionInfo.version) : '-'}</div>
            </div>
            <div>
                <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">Load Avg</div>
                <div class="text-gray-600 dark:text-gray-400">${nodeStatus.loadavg && Array.isArray(nodeStatus.loadavg) && nodeStatus.loadavg.length >= 1 ? nodeStatus.loadavg[0].toFixed(2) : '-'}</div>
            </div>
        `;
        
        section.appendChild(grid);
        
        // CPU and Memory with progress bars
        if (nodeStatus.cpu !== null && nodeStatus.cpu !== undefined) {
            const cpuPercent = nodeStatus.cpu * 100;
            const cpuColor = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
            const cpuColorClass = {
                red: 'bg-red-500/60 dark:bg-red-500/50',
                yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
                green: 'bg-green-500/60 dark:bg-green-500/50'
            }[cpuColor] || 'bg-gray-500/60 dark:bg-gray-500/50';
            
            const cpuDiv = document.createElement('div');
            cpuDiv.className = 'space-y-1';
            cpuDiv.innerHTML = `
                <div class="flex justify-between text-xs">
                    <span class="font-medium text-gray-700 dark:text-gray-300">CPU Usage</span>
                    <span class="text-gray-600 dark:text-gray-400">${cpuPercent.toFixed(1)}%</span>
                </div>
                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
                    <div class="h-2 rounded-full transition-all duration-300 ${cpuColorClass}" style="width: ${cpuPercent}%"></div>
                </div>
            `;
            section.appendChild(cpuDiv);
        }
        
        if (nodeStatus.memory && nodeStatus.memory.total && nodeStatus.memory.used !== null) {
            const memUsed = nodeStatus.memory.used;
            const memTotal = nodeStatus.memory.total;
            const memPercent = (memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0;
            const memColor = PulseApp.utils.getUsageColor(memPercent, 'memory');
            const memColorClass = {
                red: 'bg-red-500/60 dark:bg-red-500/50',
                yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
                green: 'bg-green-500/60 dark:bg-green-500/50'
            }[memColor] || 'bg-gray-500/60 dark:bg-gray-500/50';
            
            const memDiv = document.createElement('div');
            memDiv.className = 'space-y-1';
            memDiv.innerHTML = `
                <div class="flex justify-between text-xs">
                    <span class="font-medium text-gray-700 dark:text-gray-300">Memory Usage</span>
                    <span class="text-gray-600 dark:text-gray-400">${PulseApp.utils.formatBytes(memUsed)} / ${PulseApp.utils.formatBytes(memTotal)}</span>
                </div>
                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
                    <div class="h-2 rounded-full transition-all duration-300 ${memColorClass}" style="width: ${memPercent}%"></div>
                </div>
            `;
            section.appendChild(memDiv);
        }
        
        return section;
    };

    const _createMobileDatastoreSection = (instanceId, pbsInstance, statusInfo) => {
        const section = document.createElement('div');
        section.className = 'mobile-datastore-section space-y-2';
        
        const heading = document.createElement('h4');
        heading.className = 'text-sm font-semibold text-gray-700 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700 pb-1';
        heading.textContent = 'Datastores';
        section.appendChild(heading);
        
        const datastoreContainer = _createMobileDatastoreContainer(
            pbsInstance.datastores, 
            statusInfo.statusText, 
            statusInfo.showDetails
        );
        section.appendChild(datastoreContainer);
        
        return section;
    };

    const _createMobileTaskSections = (instanceId, pbsInstance, statusInfo) => {
        const section = document.createElement('div');
        section.className = 'mobile-task-sections space-y-4';
        
        const taskTypes = [
            { type: 'backup', title: 'Backup Tasks', data: pbsInstance.backupTasks },
            { type: 'verify', title: 'Verification Tasks', data: pbsInstance.verificationTasks },
            { type: 'sync', title: 'Sync Tasks', data: pbsInstance.syncTasks },
            { type: 'prunegc', title: 'Prune/GC Tasks', data: pbsInstance.pruneTasks }
        ];
        
        taskTypes.forEach(taskType => {
            if (statusInfo.showDetails && taskType.data?.recentTasks?.length > 0) {
                const taskSection = document.createElement('div');
                taskSection.className = 'space-y-2';
                
                const heading = document.createElement('h5');
                heading.className = 'text-sm font-semibold text-gray-700 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700 pb-1';
                
                // Add task count and status
                const recentTasks = taskType.data.recentTasks || [];
                const failedTasks = recentTasks.filter(task => task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running'));
                const runningTasks = recentTasks.filter(task => task.status && task.status.toLowerCase().includes('running'));
                
                let statusText = `${taskType.title} (${recentTasks.length})`;
                if (failedTasks.length > 0) {
                    statusText += ` - ${failedTasks.length} failed`;
                }
                if (runningTasks.length > 0) {
                    statusText += ` - ${runningTasks.length} running`;
                }
                
                heading.textContent = statusText;
                taskSection.appendChild(heading);
                
                const taskContainer = _createMobileTaskContainer(recentTasks);
                taskSection.appendChild(taskContainer);
                
                section.appendChild(taskSection);
            }
        });
        
        return section;
    };

    const _createMobileTaskContainer = (tasks) => {
        const container = document.createElement('div');
        container.className = 'mobile-task-container space-y-2';
        
        // Show failed tasks first, then others (up to limit)
        const failedTasks = tasks.filter(task => task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running'));
        const otherTasks = tasks.filter(task => !task.status || task.status === 'OK' || task.status.toLowerCase().includes('running'));
        const prioritizedTasks = [...failedTasks, ...otherTasks];
        
        // Limit to 5 tasks on mobile for better performance
        const displayTasks = prioritizedTasks.slice(0, 5);
        
        displayTasks.forEach(task => {
            const taskCard = _createMobileTaskCard(task);
            container.appendChild(taskCard);
        });
        
        if (prioritizedTasks.length > 5) {
            const moreButton = document.createElement('button');
            moreButton.className = 'w-full py-2 px-3 text-xs text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-600 rounded bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 transition-colors';
            moreButton.textContent = `Show ${prioritizedTasks.length - 5} More Tasks`;
            
            moreButton.addEventListener('click', () => {
                const remainingTasks = prioritizedTasks.slice(5);
                remainingTasks.forEach(task => {
                    const taskCard = _createMobileTaskCard(task);
                    container.insertBefore(taskCard, moreButton);
                });
                moreButton.remove();
            });
            
            container.appendChild(moreButton);
        }
        
        return container;
    };

    function _createPbsInstanceTabs(pbsArray, mainContainer) {
        const tabContainer = document.createElement('div');
        tabContainer.className = CSS_CLASSES.PBS_TAB_CONTAINER;

        const tabContentArea = document.createElement('div');
        tabContentArea.className = CSS_CLASSES.PBS_TAB_CONTENT_AREA;

        let tabButtons = [];
        pbsArray.forEach((pbsInstance, index) => {
            let baseId;
            if (pbsInstance.pbsInstanceName) {
                baseId = PulseApp.utils.sanitizeForId(pbsInstance.pbsInstanceName);
            } else if (pbsInstance.pbsEndpointId) {
                baseId = PulseApp.utils.sanitizeForId(pbsInstance.pbsEndpointId);
            } else {
                baseId = 'pbs-instance';
            }
            const instanceId = `${baseId}-${index}`;
            const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;

            const tabButton = document.createElement('button');
            tabButton.id = `${ID_PREFIXES.PBS_TAB_BUTTON_PREFIX}${instanceId}`;
            tabButton.className = CSS_CLASSES.PBS_TAB_BUTTON;
            tabButton.textContent = instanceName;
            tabButton.dataset.instanceId = instanceId;
            tabButton.dataset.instanceIndex = index;

            tabButton.addEventListener('click', (event) => {
                tabContainer.querySelectorAll('button').forEach(btn => {
                    CSS_CLASSES.PBS_TAB_BUTTON_ACTIVE.split(' ').forEach(cls => btn.classList.remove(cls));
                });
                CSS_CLASSES.PBS_TAB_BUTTON_ACTIVE.split(' ').forEach(cls => event.currentTarget.classList.add(cls));

                tabContentArea.innerHTML = '';

                const selectedIndex = parseInt(event.currentTarget.dataset.instanceIndex);
                selectedPbsTabIndex = selectedIndex; // Update global selected tab index
                const selectedInstanceData = pbsArray[selectedIndex];
                
                let selectedBaseId;
                if (selectedInstanceData.pbsInstanceName) {
                    selectedBaseId = PulseApp.utils.sanitizeForId(selectedInstanceData.pbsInstanceName);
                } else if (selectedInstanceData.pbsEndpointId) {
                    selectedBaseId = PulseApp.utils.sanitizeForId(selectedInstanceData.pbsEndpointId);
                } else {
                    selectedBaseId = 'pbs-instance';
                }
                const currentInstanceId = `${selectedBaseId}-${selectedIndex}`;
                const currentInstanceName = selectedInstanceData.pbsInstanceName || `PBS Instance ${selectedIndex + 1}`;
                
                const overallHealthAndTitle = _calculateOverallHealth(selectedInstanceData);
                const statusInfo = _getInstanceStatusInfo(selectedInstanceData);

                const instanceElement = _createPbsInstanceElement(
                    selectedInstanceData,
                    currentInstanceId,
                    currentInstanceName,
                    overallHealthAndTitle.overallHealth,
                    overallHealthAndTitle.healthTitle,
                    statusInfo.showDetails,
                    statusInfo.statusText
                );
                
                tabContentArea.appendChild(instanceElement);
            });
            tabContainer.appendChild(tabButton);
            tabButtons.push(tabButton);
        });

        mainContainer.appendChild(tabContainer);
        mainContainer.appendChild(tabContentArea);

        // Auto-select the previously selected tab if possible, else the first tab
        const safeIndex = Math.max(0, Math.min(selectedPbsTabIndex, tabButtons.length - 1));
        if (tabButtons[safeIndex]) {
            tabButtons[safeIndex].click();
        }
    }

    const _getInstanceStatusInfo = (pbsInstance) => {
        let statusText = 'Loading...';
        let showDetails = false;
        switch (pbsInstance.status) {
            case 'configured':
                statusText = `Configured, attempting connection...`;
                break;
            case 'ok':
                statusText = `Status: OK`;
                showDetails = true;
                break;
            case 'error':
                statusText = `Error: ${pbsInstance.errorMessage || 'Connection failed'}`;
                break;
            default:
                statusText = `Status: ${pbsInstance.status || 'Unknown'}`;
                break;
        }
        return { statusText, showDetails };
    };

    const _calculateOverallHealth = (pbsInstance) => {
        let overallHealth = 'ok';
        let healthTitle = 'OK';

        if (pbsInstance.status === 'error') {
            overallHealth = 'error';
            healthTitle = `Error: ${pbsInstance.errorMessage || 'Connection failed'}`;
        } else if (pbsInstance.status !== 'ok') {
            overallHealth = 'warning';
            healthTitle = 'Connecting or unknown status';
        } else {
            const highUsageDatastore = (pbsInstance.datastores || []).find(ds => {
                const totalBytes = ds.total || 0;
                const usedBytes = ds.used || 0;
                const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
                return usagePercent > 85;
            });
            if (highUsageDatastore) {
                overallHealth = 'warning';
                healthTitle = `Warning: Datastore ${highUsageDatastore.name} usage high (${Math.round((highUsageDatastore.used / highUsageDatastore.total) * 100)}%)`;
            }

            if (overallHealth !== 'error') {
                const hasFailures = [
                    pbsInstance.backupTasks,
                    pbsInstance.verificationTasks,
                    pbsInstance.syncTasks,
                    pbsInstance.pruneTasks
                ].some(taskGroup => (taskGroup?.summary?.failed ?? 0) > 0);

                if (hasFailures) {
                    healthTitle = 'Error: One or more recent tasks failed';
                }
            }
        }
        return { overallHealth, healthTitle };
    };

    // Layout update function for responsive design
    function updatePbsLayoutForViewport() {
        const container = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        if (!container) return;
        
        const isMobile = isMobileView();
        
        if (isMobile) {
            container.classList.add('mobile-layout');
            
            // Hide desktop tables and show mobile containers
            const tables = container.querySelectorAll('table');
            tables.forEach(table => {
                table.style.display = 'none';
                
                // Find or create mobile alternative
                const tableContainer = table.closest('.table-container') || table.parentElement;
                if (tableContainer) {
                    let mobileContainer = tableContainer.querySelector('.mobile-datastore-container, .mobile-task-container');
                    
                    // If no mobile container exists, we'll need to recreate the data
                    if (!mobileContainer && table.id && table.id.includes('ds-table')) {
                        // This is a datastore table, create mobile version
                        const dsTableBody = table.querySelector('tbody');
                        if (dsTableBody && dsTableBody.children.length > 0) {
                            // Extract data from existing table and create mobile cards
                            const instanceId = table.id.replace(ID_PREFIXES.PBS_DS_TABLE, '');
                            mobileContainer = document.createElement('div');
                            mobileContainer.className = 'mobile-datastore-container space-y-3';
                            tableContainer.appendChild(mobileContainer);
                        }
                    }
                }
            });
            
        } else {
            container.classList.remove('mobile-layout');
            
            // Show desktop tables and hide mobile containers
            const tables = container.querySelectorAll('table');
            tables.forEach(table => {
                table.style.display = '';
            });
            
            const mobileContainers = container.querySelectorAll('.mobile-datastore-container, .mobile-task-container');
            mobileContainers.forEach(mobileContainer => {
                mobileContainer.style.display = 'none';
            });
        }
    }

    // Initialize PBS namespace filter
    function _initPbsNamespaceFilter() {
        pbsNamespaceFilter = document.getElementById('pbs-namespace-filter');
        const filterContainer = document.getElementById('pbs-filter-container');
        
        if (!pbsNamespaceFilter || !filterContainer) return;
        
        // Add change listener
        pbsNamespaceFilter.addEventListener('change', () => {
            PulseApp.state.set('pbsFilterNamespace', pbsNamespaceFilter.value);
            // Re-render PBS info with current data
            const pbsData = PulseApp.state.get('pbsDataArray');
            if (pbsData) {
                updatePbsInfo(pbsData);
            }
        });
        
        // Initial update
        _updatePbsNamespaceOptions();
    }

    // Update PBS namespace dropdown options
    function _updatePbsNamespaceOptions() {
        if (!pbsNamespaceFilter) return;
        
        const pbsDataArray = PulseApp.state.get('pbsDataArray') || [];
        const namespaces = new Set(['root']); // Always include root
        
        // Collect all namespaces from PBS data
        pbsDataArray.forEach(pbsInstance => {
            // Check datastore snapshots (like Backups tab does)
            if (pbsInstance.datastores) {
                pbsInstance.datastores.forEach(ds => {
                    if (ds.snapshots) {
                        ds.snapshots.forEach(snap => {
                            namespaces.add(snap.namespace || 'root');
                        });
                    }
                });
            }
            
            // Check backup tasks
            if (pbsInstance.backupTasks && pbsInstance.backupTasks.recentTasks) {
                pbsInstance.backupTasks.recentTasks.forEach(task => {
                    if (task.namespace) {
                        namespaces.add(task.namespace);
                    }
                });
            }
            
            // Check verification tasks
            if (pbsInstance.verificationTasks && pbsInstance.verificationTasks.recentTasks) {
                pbsInstance.verificationTasks.recentTasks.forEach(task => {
                    if (task.namespace) {
                        namespaces.add(task.namespace);
                    }
                });
            }
            
            // Check sync tasks
            if (pbsInstance.syncTasks && pbsInstance.syncTasks.recentTasks) {
                pbsInstance.syncTasks.recentTasks.forEach(task => {
                    if (task.namespace) {
                        namespaces.add(task.namespace);
                    }
                });
            }
            
            // Check prune tasks
            if (pbsInstance.pruneTasks && pbsInstance.pruneTasks.recentTasks) {
                pbsInstance.pruneTasks.recentTasks.forEach(task => {
                    if (task.namespace) {
                        namespaces.add(task.namespace);
                    }
                });
            }
        });
        
        // Update options
        const currentValue = pbsNamespaceFilter.value || 'all';
        pbsNamespaceFilter.innerHTML = '<option value="all">All Namespaces</option>';
        
        Array.from(namespaces).sort().forEach(ns => {
            const option = document.createElement('option');
            option.value = ns;
            option.textContent = ns === 'root' ? 'Root Namespace' : `${ns} Namespace`;
            pbsNamespaceFilter.appendChild(option);
        });
        
        // Restore selection
        pbsNamespaceFilter.value = currentValue;
        
        // Show/hide filter based on having PBS data
        const filterContainer = document.getElementById('pbs-filter-container');
        if (filterContainer) {
            const hasPBS = pbsDataArray.length > 0;
            filterContainer.style.display = hasPBS ? '' : 'none';
        }
    }

    // Filter PBS tasks by namespace
    function _filterPbsTasksByNamespace(tasks, selectedNamespace) {
        if (!selectedNamespace || selectedNamespace === 'all') {
            return tasks;
        }
        
        
        return tasks.filter(task => task.namespace === selectedNamespace);
    }

    function _recalculateTaskSummary(tasks) {
        if (!tasks || tasks.length === 0) {
            return { ok: 0, failed: 0, lastOk: null, lastFailed: null };
        }
        
        let ok = 0;
        let failed = 0;
        let lastOk = null;
        let lastFailed = null;
        
        tasks.forEach(task => {
            if (task.status === 'OK') {
                ok++;
                if (!lastOk || (task.startTime && task.startTime > lastOk)) {
                    lastOk = task.startTime;
                }
            } else if (task.status === 'ERROR' || task.status === 'FAILED') {
                failed++;
                if (!lastFailed || (task.startTime && task.startTime > lastFailed)) {
                    lastFailed = task.startTime;
                }
            }
        });
        
        return { ok, failed, lastOk, lastFailed };
    }

    function updatePbsInfo(pbsArray) {
        
        const container = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        if (!container) {
            console.error(`PBS container element #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found!`);
            return;
        }
        
        // Check if viewport has changed (mobile to desktop or vice versa)
        const currentlyMobile = window.innerWidth <= 768;
        const viewportChanged = lastKnownMobileState !== null && lastKnownMobileState !== currentlyMobile;
        
        // Clear persistent state on viewport change to allow layout rebuild
        if (viewportChanged) {
            persistentExpandedDetails.clear();
            // Force immediate container clear to rebuild layout for new viewport
            container.innerHTML = '';
        } else if (persistentExpandedDetails.size > 0 && currentlyMobile) {
            // Skip DOM rebuild if detail cards are expanded to prevent flicker (mobile only)
            return;
        }
        
        // Update the last known state
        lastKnownMobileState = currentlyMobile;
        
        // Clear container if not already cleared above
        if (!viewportChanged) {
            container.innerHTML = '';
        }

        const loadingMessage = document.getElementById('pbs-loading-message');
        if (loadingMessage) {
            loadingMessage.remove();
        }

        // Initialize namespace filter if not already done
        if (!pbsNamespaceFilter) {
            _initPbsNamespaceFilter();
        }

        if (!pbsArray || pbsArray.length === 0) {
            const placeholder = document.createElement('p');
            placeholder.className = CSS_CLASSES.TEXT_GRAY_500_DARK_TEXT_GRAY_400_P4_TEXT_CENTER_TEXT_SM;
            placeholder.textContent = 'Proxmox Backup Server integration is not configured.';
            container.appendChild(placeholder);
            return;
        }

        // Get current namespace filter selection
        const selectedNamespace = PulseApp.state.get('pbsFilterNamespace') || 'all';
        
        // Apply namespace filtering to all PBS instances
        const filteredPbsArray = pbsArray.map(pbsInstance => {
            const filteredInstance = { ...pbsInstance };
            
            // Filter backup tasks
            if (filteredInstance.backupTasks && filteredInstance.backupTasks.recentTasks) {
                const filteredTasks = _filterPbsTasksByNamespace(filteredInstance.backupTasks.recentTasks, selectedNamespace);
                filteredInstance.backupTasks = {
                    ...filteredInstance.backupTasks,
                    recentTasks: filteredTasks,
                    summary: _recalculateTaskSummary(filteredTasks)
                };
            }
            
            // Filter verification tasks
            if (filteredInstance.verificationTasks && filteredInstance.verificationTasks.recentTasks) {
                const filteredTasks = _filterPbsTasksByNamespace(filteredInstance.verificationTasks.recentTasks, selectedNamespace);
                filteredInstance.verificationTasks = {
                    ...filteredInstance.verificationTasks,
                    recentTasks: filteredTasks,
                    summary: _recalculateTaskSummary(filteredTasks)
                };
            }
            
            // Filter sync tasks
            if (filteredInstance.syncTasks && filteredInstance.syncTasks.recentTasks) {
                const filteredTasks = _filterPbsTasksByNamespace(filteredInstance.syncTasks.recentTasks, selectedNamespace);
                filteredInstance.syncTasks = {
                    ...filteredInstance.syncTasks,
                    recentTasks: filteredTasks,
                    summary: _recalculateTaskSummary(filteredTasks)
                };
            }
            
            // Filter prune tasks
            if (filteredInstance.pruneTasks && filteredInstance.pruneTasks.recentTasks) {
                const filteredTasks = _filterPbsTasksByNamespace(filteredInstance.pruneTasks.recentTasks, selectedNamespace);
                filteredInstance.pruneTasks = {
                    ...filteredInstance.pruneTasks,
                    recentTasks: filteredTasks,
                    summary: _recalculateTaskSummary(filteredTasks)
                };
            }
            
            // Filter datastores by namespace
            if (filteredInstance.datastores && selectedNamespace !== 'all') {
                filteredInstance.datastores = filteredInstance.datastores.map(ds => ({
                    ...ds,
                    snapshots: ds.snapshots ? ds.snapshots.filter(snap => 
                        (snap.namespace || 'root') === selectedNamespace
                    ) : []
                }));
            }
            
            return filteredInstance;
        });

        // Update namespace options with current data
        _updatePbsNamespaceOptions();

        // Determine layout based on viewport and number of instances
        const isMobile = isMobileView();
        
        if (filteredPbsArray.length === 1) {
            const pbsInstance = filteredPbsArray[0];
            let baseId;
            if (pbsInstance.pbsInstanceName) {
                baseId = PulseApp.utils.sanitizeForId(pbsInstance.pbsInstanceName);
            } else if (pbsInstance.pbsEndpointId) {
                baseId = PulseApp.utils.sanitizeForId(pbsInstance.pbsEndpointId);
            } else {
                baseId = 'pbs-instance';
            }
            const instanceId = `${baseId}-0`;
            const instanceName = pbsInstance.pbsInstanceName || `PBS Instance 1`;
            
            const overallHealthAndTitle = _calculateOverallHealth(pbsInstance);
            const statusInfo = _getInstanceStatusInfo(pbsInstance);

            if (isMobile) {
                // Use mobile single instance layout
                container.classList.add('mobile-layout');
                const mobileWrapper = document.createElement('div');
                mobileWrapper.className = 'mobile-single-instance space-y-4';
                
                // Add mobile summary at top
                if (statusInfo.showDetails) {
                    mobileWrapper.appendChild(_createMobilePbsSummary(pbsInstance));
                    mobileWrapper.appendChild(_createMobileNodeStatusSection(instanceId, pbsInstance));
                    mobileWrapper.appendChild(_createMobileDatastoreSection(instanceId, pbsInstance, statusInfo));
                    mobileWrapper.appendChild(_createMobileTaskSections(instanceId, pbsInstance, statusInfo));
                } else {
                    const statusCard = document.createElement('div');
                    statusCard.className = 'p-4 text-center text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-700';
                    statusCard.textContent = statusInfo.statusText;
                    mobileWrapper.appendChild(statusCard);
                }
                
                container.appendChild(mobileWrapper);
            } else {
                // Use desktop single instance layout
                const instanceElement = _createPbsInstanceElement(
                    pbsInstance,
                    instanceId,
                    instanceName,
                    overallHealthAndTitle.overallHealth,
                    overallHealthAndTitle.healthTitle,
                    statusInfo.showDetails,
                    statusInfo.statusText
                );
                container.appendChild(instanceElement);
            }
        } else {
            // Multiple instances
            if (isMobile) {
                container.classList.add('mobile-layout');
                _createMobileInstanceAccordion(filteredPbsArray, container);
            } else {
                _createPbsInstanceTabs(filteredPbsArray, container);
            }
        }
        
        // Initialize mobile features
        if (isMobile) {
            setTimeout(() => {
                _initMobileScrollIndicators();
                updatePbsLayoutForViewport();
            }, 100);
        }
        
        // Auto-expand detail cards after DOM rebuild is complete
        setTimeout(() => {
            persistentExpandedDetails.forEach(upid => {
                const taskCard = document.querySelector(`[data-upid="${CSS.escape(upid)}"]`);
                if (taskCard) {
                    const expandButton = taskCard.querySelector('button');
                    const existingDetailCard = taskCard.nextElementSibling;
                    
                    if (!existingDetailCard || !existingDetailCard.classList.contains('mobile-task-detail-card')) {
                        // Find the task data from the filtered PBS array
                        const taskData = findTaskByUpid(filteredPbsArray, upid);
                        if (taskData) {
                            const detailCard = _createMobileTaskDetailCard(taskData);
                            taskCard.insertAdjacentElement('afterend', detailCard);
                        }
                    }
                    
                    // Update button text
                    if (expandButton) {
                        expandButton.textContent = 'Hide Error Details';
                    }
                }
            });
        }, 200); // Wait longer than mobile features to ensure DOM is stable
    }

    function initPbsEventListeners() {
        const pbsInstancesContainer = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        if (!pbsInstancesContainer) {
            console.warn(`PBS instances container #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found. Some UI interactions might not work.`);
            return;
        }
        
        // Listen for viewport changes to switch layouts
        const debouncedLayoutUpdate = debounce(updatePbsLayoutForViewport, 250);
        window.addEventListener('resize', debouncedLayoutUpdate);
        window.addEventListener('orientationchange', () => {
            // Orientation change needs a slight delay to get correct dimensions
            setTimeout(debouncedLayoutUpdate, 100);
        });
        
        // Initial layout setup
        updatePbsLayoutForViewport();
    }

    return {
        updatePbsInfo,
        initPbsEventListeners,
        updatePbsLayoutForViewport,
        isMobileView,
        isTabletView
    };
})();
