PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {

    // Global state tracker for expanded PBS tasks
    let expandedTaskState = new Set();
    let expandedShowMoreState = new Set();
    let taskStatusInfo = new Map(); // Track status info for each table
    
    let selectedNamespaceTab = 'root'; // Track selected namespace tab (for single instance)
    let selectedNamespaceByInstance = new Map(); // Track selected namespace per PBS instance
    
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
    


    // Common CSS classes used frequently throughout the PBS UI
    const CSS_CLASSES = {
        // Text sizes
        TEXT_XS: 'text-xs',
        TEXT_SM: 'text-sm',
        TEXT_MD: 'text-md',
        TEXT_LG: 'text-lg',
        
        // Text colors
        TEXT_GRAY_400: 'text-gray-400',
        TEXT_GRAY_600_DARK_GRAY_400: 'text-gray-600 dark:text-gray-400',
        TEXT_GRAY_700_DARK_GRAY_300: 'text-gray-700 dark:text-gray-300',
        TEXT_GRAY_500_DARK_GRAY_400: 'text-gray-500 dark:text-gray-400',
        TEXT_GRAY_800_DARK_GRAY_200: 'text-gray-800 dark:text-gray-200',
        
        // Font weights
        FONT_SEMIBOLD: 'font-semibold',
        
        // Layout
        HIDDEN: 'hidden',
        P1_PX2: 'p-1 px-2',
        P3: 'p-3',
        P4: 'p-4',
        WHITESPACE_NOWRAP: 'whitespace-nowrap',
        TEXT_LEFT: 'text-left',
        ROUNDED: 'rounded',
        OVERFLOW_X_AUTO: 'overflow-x-auto',
        MIN_W_FULL: 'min-w-full',
        TRACKING_WIDER: 'tracking-wider',
        FLEX: 'flex',
        JUSTIFY_BETWEEN: 'justify-between',
        ITEMS_CENTER: 'items-center',
        
        // Spacing
        MB2: 'mb-2',
        MB3: 'mb-3',
        MB4: 'mb-4',
        SPACE_Y_1: 'space-y-1',
        SPACE_Y_2: 'space-y-2',
        SPACE_Y_3: 'space-y-3',
        SPACE_Y_4: 'space-y-4',
        
        // Borders & dividers
        BORDER_GRAY_200_DARK_BORDER_GRAY_700: 'border border-gray-200 dark:border-gray-700',
        DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700: 'divide-y divide-gray-200 dark:divide-gray-700',
        BORDER_B_GRAY_300_DARK_BORDER_GRAY_600: 'border-b border-gray-300 dark:border-gray-600',
        
        // Background colors
        BG_GRAY_100_DARK_BG_GRAY_800: 'bg-gray-100 dark:bg-gray-800',
        BG_GRAY_100_DARK_BG_GRAY_700: 'bg-gray-100 dark:bg-gray-700',
        HOVER_BG_GRAY_50_DARK_HOVER_BG_GRAY_700_50: 'hover:bg-gray-50 dark:hover:bg-gray-700/50',
        
        // Text styling
        TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300: 'text-gray-600 uppercase dark:text-gray-300',
        FONT_MEDIUM: 'font-medium',
        
        // Positioning
        STICKY: 'sticky',
        TOP_0: 'top-0',
        Z_10: 'z-10',
        
        // PBS specific
        PBS_TASK_TBODY: 'pbs-task-tbody',
        PBS_INSTANCE_SECTION: 'pbs-instance-section mb-6',
        PBS_INSTANCE_DETAILS: 'pbs-instance-details',
        PBS_TASK_SECTION: 'pbs-task-section mb-6',
        PBS_SHOW_MORE: 'pbs-show-more',
        PBS_NO_TASKS: 'pbs-no-tasks',
        PBS_TAB_CONTENT_AREA: 'pbs-tab-content-area mt-4',
        
        // Combined classes
        TEXT_GRAY_P4_CENTER: 'text-gray-500 dark:text-gray-400 p-4 text-center text-sm'
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

    const getPbsStatusIcon = (status) => {
        if (status === 'OK') {
            return `<span class="text-green-500 dark:text-green-400" title="OK">OK</span>`;
        } else if (status === 'running') {
            return `<span class="inline-block animate-spin rounded-full h-3 w-3 border-t-2 border-b-2 border-blue-500" title="Running"></span>`;
        } else if (status) {
            return `<span class="text-red-500 dark:text-red-400 font-bold" title="${status}">ERROR</span>`;
        } else {
            return `<span class="${CSS_CLASSES.TEXT_GRAY_400}" title="Unknown">?</span>`;
        }
    };

    const getPbsStatusDisplay = (status) => {
        if (status === 'OK') {
            return `<span class="text-green-500 dark:text-green-400">OK</span>`;
        } else if (status === 'running') {
            return `<span class="inline-block animate-spin rounded-full h-3 w-3 border-t-2 border-b-2 border-blue-500" title="Running"></span> <span class="text-blue-600 dark:text-blue-400">Running</span>`;
        } else if (status) {
            // For failed tasks, show the full error message
            const shortStatus = status.length > 50 ? `${status.substring(0, 47)}...` : status;
            return `<span class="text-red-500 dark:text-red-400 font-bold" title="${status}">ERROR</span> <span class="text-red-600 dark:text-red-400 ${CSS_CLASSES.TEXT_XS}" title="${status}">${shortStatus}</span>`;
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
          colorClass = 'text-red-500 dark:text-red-400';
      } else if (gcStatus === 'OK') {
          colorClass = 'text-green-500 dark:text-green-400';
      }
      return `<span class="${CSS_CLASSES.TEXT_XS} ${colorClass}">${gcStatus}</span>`;
    };

    // Helper function to find guest name from VM/container data
    const findGuestName = (guestType, guestId) => {
        try {
            // Check if we have initial data loaded
            if (!PulseApp.state || !PulseApp.state.get('initialDataReceived')) {
                return null;
            }
            
            // Get guest arrays directly from state using correct keys
            const containers = PulseApp.state.get('containersData') || [];
            const vms = PulseApp.state.get('vmsData') || [];
            
            // Handle both "ct" and "qemu"/"vm" guest types
            const guestArray = (guestType === 'ct' || guestType === 'lxc') 
                ? containers 
                : vms;
            
            if (!guestArray || !Array.isArray(guestArray)) {
                return null;
            }
            
            const guest = guestArray.find(g => g.vmid === parseInt(guestId));
            return guest ? guest.name : null;
        } catch (error) {
            // Silently fail if we can't get guest name
            return null;
        }
    };

    const parsePbsTaskTarget = (task) => {
      // For synthetic backup run tasks, enhance with guest name if available
      if (task.guest && task.pbsBackupRun) {
          // Check if guest field is in format "ct/103" or "qemu/102"
          const guestParts = task.guest.split('/');
          if (guestParts.length === 2) {
              const guestType = guestParts[0];
              const guestId = guestParts[1];
              const guestName = findGuestName(guestType, guestId);
              return guestName ? `${task.guest} (${guestName})` : task.guest;
          }
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
              const baseTarget = `${guestType}/${guestId}`;
              
              // Try to find guest name and append it if found
              const guestName = findGuestName(guestType, guestId);
              displayTarget = guestName ? `${baseTarget} (${guestName})` : baseTarget;
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

    // Helper functions to reduce duplication
    const createElement = (tag, className, innerHTML = '') => {
        const element = document.createElement(tag);
        if (className) element.className = className;
        if (innerHTML) element.innerHTML = innerHTML;
        return element;
    };

    const createTableCell = (content, className = '') => {
        const td = document.createElement('td');
        td.className = CSS_CLASSES.P1_PX2 + (className ? ' ' + className : '');
        td.innerHTML = content;
        return td;
    };

    const formatTaskTiming = (task) => {
        const startTime = task.startTime ? PulseApp.utils.formatPbsTimestampRelative(task.startTime) : 'N/A';
        const duration = task.duration !== null ? PulseApp.utils.formatDuration(task.duration) : 'N/A';
        return { startTime, duration };
    };

    const getShortUpid = (upid) => {
        if (!upid || upid === 'N/A') return upid;
        return upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;
    };

    const isTaskFailed = (task) => {
        return task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running');
    };

    // Create a standard section container
    const createSection = (className = '', content = '') => {
        const section = createElement('div', className);
        if (content) section.innerHTML = content;
        return section;
    };

    // Create text elements with common styling
    const createText = (tag, text, className = '') => {
        const element = createElement(tag, className);
        element.textContent = text;
        return element;
    };

    // Apply status-based styling to element
    const applyTaskStatusStyling = (element, task) => {
        if (isTaskFailed(task)) {
            element.classList.add('border-red-300', 'dark:border-red-600', 'bg-red-50', 'dark:bg-red-900/10');
        } else if (task.status && task.status.toLowerCase().includes('running')) {
            element.classList.add('border-blue-300', 'dark:border-blue-600', 'bg-blue-50', 'dark:bg-blue-900/10');
        }
    };

    // Mobile-friendly task card component
    const _createMobileTaskCard = (task) => {
        const target = parsePbsTaskTarget(task);
        const statusDisplayHTML = getPbsStatusDisplay(task.status);
        const { startTime, duration } = formatTaskTiming(task);
        const upid = task.upid || 'N/A';
        const shortUpid = getShortUpid(upid);

        const card = createElement('div', 'mobile-task-card p-3 border border-gray-200 dark:border-gray-700 rounded-lg mb-3 bg-white dark:bg-gray-800 transition-all duration-200');
        
        // Add UPID as data attribute for tracking expanded state
        card.dataset.upid = task.upid || '';
        
        // Add status-based styling for better problem visibility
        const isFailed = isTaskFailed(task);
        applyTaskStatusStyling(card, task);

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
        
        const endTime = task.endTime ? PulseApp.utils.formatPbsTimestampRelative(task.endTime) : 'N/A';
        const exitCodeDisplay = task.exitCode !== undefined ? task.exitCode : 'N/A';
        const exitCodeClass = task.exitCode !== undefined && task.exitCode !== 0 ? 'text-red-600 dark:text-red-400 font-semibold' : '';
        
        infoSection.innerHTML = `
            <div class="grid grid-cols-1 gap-2">
                <div><strong>Task Type:</strong> ${task.type || 'N/A'}</div>
                <div><strong>Node:</strong> ${task.node || 'N/A'}</div>
                <div><strong>User:</strong> ${task.user || 'N/A'}</div>
                <div><strong>Start Time:</strong> ${task.startTime ? PulseApp.utils.formatPbsTimestampRelative(task.startTime) : 'N/A'}</div>
                <div><strong>End Time:</strong> ${endTime}</div>
                <div><strong>Exit Code:</strong> <span class="${exitCodeClass}">${exitCodeDisplay}</span></div>
                <div><strong>Full UPID:</strong> <span class="font-mono break-all">${task.upid || 'N/A'}</span></div>
            </div>
        `;
        
        detailContent.appendChild(infoSection);
        detailCard.appendChild(detailContent);
        
        return detailCard;
    };

    const _createTaskTableRow = (task, isBackupTable = false) => {
        const target = parsePbsTaskTarget(task);
        const statusDisplayHTML = getPbsStatusDisplay(task.status);
        const { startTime, duration } = formatTaskTiming(task);
        const upid = task.upid || 'N/A';
        const shortUpid = getShortUpid(upid);

        const row = document.createElement('tr');
        
        // Add UPID as data attribute for tracking expanded state
        row.dataset.upid = task.upid || '';
        
        // Add status-based row styling for better problem visibility
        const isFailed = isTaskFailed(task);
        let specialBgClass = '';
        let specialHoverClass = '';
        let additionalClasses = 'transition-colors duration-150 ease-in-out';
        
        if (isFailed) {
            // Failed tasks get red background and cursor pointer if expandable
            specialBgClass = 'bg-red-50 dark:bg-red-900/20';
            specialHoverClass = 'hover:bg-red-100 dark:hover:bg-red-900/30';
            additionalClasses += ' cursor-pointer';
        } else if (task.status && task.status.toLowerCase().includes('running')) {
            // Running tasks get blue background
            specialBgClass = 'bg-blue-50 dark:bg-blue-900/20';
            specialHoverClass = 'hover:bg-blue-100 dark:hover:bg-blue-900/30';
        }
        
        // Use helper to create consistent row
        const newRow = PulseApp.ui.common.createTableRow({
            classes: additionalClasses,
            isSpecialRow: !!(specialBgClass),
            specialBgClass: specialBgClass,
            specialHoverClass: specialHoverClass
        });
        
        row.className = newRow.className;

        const targetCell = document.createElement('td');
        
        if (isBackupTable) {
            // Only apply sticky to backup tasks table
            targetCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300} sticky left-0 z-10 bg-white dark:bg-gray-800 border-r border-gray-300 dark:border-gray-600`;
        } else {
            targetCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        }
        
        // Add expand indicator for failed tasks
        if (isFailed) {
            targetCell.innerHTML = `${target}`;
        } else {
            targetCell.textContent = target;
        }
        row.appendChild(targetCell);

        row.appendChild(createTableCell(statusDisplayHTML, `${CSS_CLASSES.TEXT_SM} min-w-48`));
        
        const namespaceText = task.namespace === 'root' ? 'Root' : (task.namespace || 'Root');
        row.appendChild(createTableCell(namespaceText, `${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.WHITESPACE_NOWRAP}`));
        
        row.appendChild(createTableCell(startTime, `${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.WHITESPACE_NOWRAP}`));
        
        row.appendChild(createTableCell(duration, `${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.WHITESPACE_NOWRAP}`));

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
        const endTime = task.endTime ? PulseApp.utils.formatPbsTimestampRelative(task.endTime) : 'N/A';
        const exitCodeDisplay = task.exitCode !== undefined ? task.exitCode : 'N/A';
        const exitCodeClass = task.exitCode !== undefined && task.exitCode !== 0 ? 'text-red-600 dark:text-red-400 font-semibold' : '';
        
        rightInfo.innerHTML = `
            <div><strong>Start Time:</strong> ${task.startTime ? PulseApp.utils.formatPbsTimestampRelative(task.startTime) : 'N/A'}</div>
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
      
      // Count failed tasks for status display
      const failedTasks = tasks.filter(task => task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running'));
      
      // Determine which tasks to display based on "Show More" state
      let displayedTasks;
      if (isShowMoreExpanded) {
          // If "Show More" was previously expanded, show all tasks
          displayedTasks = tasks;
      } else {
          // Otherwise, show limited tasks
          displayedTasks = tasks.slice(0, initialLimit);
      }

      // Update status indicator in header
      if (tableId) {
          const runningTasks = tasks.filter(task => task.status && task.status.toLowerCase().includes('running')).length;
          
          let statusText = `(${tasks.length} total`;
          if (failedTasks.length > 0) {
              statusText += `, <span class="text-red-600 dark:text-red-400 font-semibold">${failedTasks.length} failed</span>`;
          }
          if (runningTasks > 0) {
              statusText += `, <span class="text-blue-600 dark:text-blue-400">${runningTasks} running</span>`;
          }
          statusText += ')';
          
          // Store the status info globally
          taskStatusInfo.set(tableId, {
              statusText: statusText
          });
          
          // Apply to DOM if elements exist
          const statusSpan = document.getElementById(`${tableId}-status`);
          
          if (statusSpan) {
              statusSpan.innerHTML = statusText;
          }
      }

      if (tasks.length === 0) {
          if (noTasksMessage) noTasksMessage.classList.remove(CSS_CLASSES.HIDDEN);
          if (showMoreButton) showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
      } else {
          if (noTasksMessage) noTasksMessage.classList.add(CSS_CLASSES.HIDDEN);

          displayedTasks.forEach(task => {
              const isBackupTable = tableId && tableId.includes('backup');
              const taskRow = _createTaskTableRow(task, isBackupTable);
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
              if (tasks.length > initialLimit) {
                  showMoreButton.classList.remove(CSS_CLASSES.HIDDEN);
                  
                  // Update button text and state based on current expansion
                  if (isShowMoreExpanded) {
                      showMoreButton.textContent = 'Show Less';
                  } else {
                      const remainingCount = tasks.length - initialLimit;
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
      // Use double requestAnimationFrame to ensure DOM is fully rendered
      if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
          requestAnimationFrame(() => {
              requestAnimationFrame(() => {
                  scrollableContainer.scrollLeft = currentScrollLeft;
                  scrollableContainer.scrollTop = currentScrollTop;
              });
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
            nameElement.innerHTML = `<span class="text-red-700 dark:text-red-300">${nameContent}</span><div class="text-xs text-red-600 dark:text-red-400 font-normal mt-1">CRITICAL: ${usagePercent.toFixed(1)}% full</div>`;
        } else if (usagePercent >= 85) {
            nameElement.innerHTML = `<span class="text-yellow-700 dark:text-yellow-300">${nameContent}</span><div class="text-xs text-yellow-600 dark:text-yellow-400 font-normal mt-1">WARNING: ${usagePercent.toFixed(1)}% full</div>`;
        } else {
            nameElement.textContent = nameContent;
        }
        
        const usageElement = document.createElement('div');
        usageElement.className = 'text-right flex-shrink-0';
        usageElement.innerHTML = `
            <div class="text-lg font-semibold ${usageColor.replace('bg-', 'text-').replace('-500', '-600').replace('-400', '-500')}">${usagePercent.toFixed(1)}%</div>
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

                    // Use consolidated table row helper
                    let specialBgClass = '';
                    let specialHoverClass = '';
                    
                    if (usagePercent >= 95) {
                        specialBgClass = 'bg-red-50 dark:bg-red-900/20';
                        specialHoverClass = 'hover:bg-red-100 dark:hover:bg-red-900/30';
                    } else if (usagePercent >= 85) {
                        specialBgClass = 'bg-yellow-50 dark:bg-yellow-900/20';
                        specialHoverClass = 'hover:bg-yellow-100 dark:hover:bg-yellow-900/30';
                    }
                    
                    const row = PulseApp.ui.common.createTableRow({
                        isSpecialRow: !!(specialBgClass),
                        specialBgClass: specialBgClass,
                        specialHoverClass: specialHoverClass
                    });

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
                        nameContent = `${nameContent} [CRITICAL: ${usagePercent.toFixed(1)}% full]`;
                        createCell(nameContent, ['text-red-700', 'dark:text-red-300', 'font-semibold']);
                    } else if (usagePercent >= 85) {
                        nameContent = `${nameContent} [WARNING: ${usagePercent.toFixed(1)}% full]`;
                        createCell(nameContent, ['text-yellow-700', 'dark:text-yellow-300', 'font-semibold']);
                    } else {
                        createCell(nameContent);
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
                        usageCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor, `${usagePercent.toFixed(1)}%`);
                    } else {
                        usageCell.textContent = 'N/A';
                    }

                    // Add deduplication factor column
                    const deduplicationText = ds.deduplicationFactor ? `${ds.deduplicationFactor}x` : 'N/A';
                    createCell(deduplicationText, [CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400, CSS_CLASSES.FONT_SEMIBOLD]);

                    createCell(gcStatusHtml, [], true);
                    
                    dsTableBody.appendChild(row);
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
        // Use double requestAnimationFrame to ensure DOM is fully rendered
        if (scrollableContainer && (currentScrollLeft > 0 || currentScrollTop > 0)) {
            requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                    scrollableContainer.scrollLeft = currentScrollLeft;
                    scrollableContainer.scrollTop = currentScrollTop;
                });
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
                const section = detailsContainer.querySelector(`.pbs-task-section[${DATA_ATTRIBUTES.TASK_TYPE}="${taskInfo.type}"]`);
                if (section) populatePbsTaskTable(section, taskInfo.data?.recentTasks);
            });
        } else {
            taskTypes.forEach(taskInfo => {
                const tbody = document.getElementById(taskInfo.elementSuffix);
                if (tbody) tbody.innerHTML = `<tr><td colspan="4" class="px-4 py-4 ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_400} text-center">${statusText}</td></tr>`;
            });
        }
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
        dsSection.className = CSS_CLASSES.MB4;
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
        dsThead.className = `${CSS_CLASSES.STICKY} ${CSS_CLASSES.TOP_0} ${CSS_CLASSES.Z_10} ${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_700}`;
        const dsHeaderRow = document.createElement('tr');
        dsHeaderRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;
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
        sectionDiv.className = `${CSS_CLASSES.MB4}`;

        const heading = document.createElement('h4');
        heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        heading.textContent = 'PBS Task Summary';
        sectionDiv.appendChild(heading);

        const tableContainer = document.createElement('div');
        tableContainer.className = `${CSS_CLASSES.OVERFLOW_X_AUTO} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} ${CSS_CLASSES.BG_GRAY_50_DARK_BG_GRAY_800_30} p-3`;
        
        const table = document.createElement('table');
        table.className = `${CSS_CLASSES.MIN_W_FULL} ${CSS_CLASSES.TEXT_SM}`;
        
        const thead = document.createElement('thead');
        thead.className = `${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_700}`;
        const headerRow = document.createElement('tr');
        headerRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;

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
            const lastOk = PulseApp.utils.formatPbsTimestampRelative(summary.lastOk);
            const lastFailed = PulseApp.utils.formatPbsTimestampRelative(summary.lastFailed);

            // Use consolidated table row helper
            const row = PulseApp.ui.common.createTableRow({
                isSpecialRow: failed > 0,
                specialBgClass: failed > 0 ? 'bg-red-50 dark:bg-red-900/20' : '',
                specialHoverClass: failed > 0 ? 'hover:bg-red-100 dark:hover:bg-red-900/30' : ''
            });

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
            
            tbody.appendChild(row);
        });

        table.appendChild(tbody);
        tableContainer.appendChild(table);
        sectionDiv.appendChild(tableContainer);
        
        return sectionDiv;
    };

    const _createTaskTableElement = (tableId, title, idColumnHeader, taskData) => {
        const fragment = document.createDocumentFragment();

        const heading = document.createElement('h4');
        heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        
        // Calculate status immediately if we have task data
        let statusContent = '';
        if (taskData && taskData.recentTasks) {
            const tasks = taskData.recentTasks;
            const failedTasks = tasks.filter(task => task.status && task.status !== 'OK' && !task.status.toLowerCase().includes('running'));
            const runningTasks = tasks.filter(task => task.status && task.status.toLowerCase().includes('running')).length;
            
            statusContent = `(${tasks.length} total`;
            if (failedTasks.length > 0) {
                statusContent += `, <span class="text-red-600 dark:text-red-400 font-semibold">${failedTasks.length} failed</span>`;
            }
            if (runningTasks > 0) {
                statusContent += `, <span class="text-blue-600 dark:text-blue-400">${runningTasks} running</span>`;
            }
            statusContent += ')';
            
            // Store it for later updates
            taskStatusInfo.set(tableId, {
                statusText: statusContent
            });
        } else {
            // Check if we have stored status info for this table
            const storedStatus = taskStatusInfo.get(tableId);
            statusContent = storedStatus ? storedStatus.statusText : '';
        }
        
        heading.innerHTML = `Recent ${title} Tasks <span id="${tableId}-status" class="text-xs font-normal text-gray-500">${statusContent}</span>`;
        fragment.appendChild(heading);

        const tableContainer = document.createElement('div');
        tableContainer.className = `${CSS_CLASSES.OVERFLOW_X_AUTO} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED}`;

        const table = document.createElement('table');
        table.id = tableId;
        table.className = `${CSS_CLASSES.MIN_W_FULL} ${CSS_CLASSES.DIVIDE_Y_GRAY_200_DARK_DIVIDE_GRAY_700} ${CSS_CLASSES.TEXT_SM}`;

        const thead = document.createElement('thead');
        thead.className = `${CSS_CLASSES.STICKY} ${CSS_CLASSES.TOP_0} ${CSS_CLASSES.Z_10} ${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_700}`;
        const headerRow = document.createElement('tr');
        headerRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;

        const headers = [idColumnHeader, 'Status', 'Namespace', 'Start Time', 'Duration'];
        const isBackupTable = tableId && tableId.includes('backup');
        
        headers.forEach((text, index) => {
            const th = document.createElement('th');
            th.scope = 'col';
            
            if (index === 0 && isBackupTable) {
                // Make first header sticky for backup tables
                th.className = `${CSS_CLASSES.P1_PX2} sticky left-0 bg-gray-100 dark:bg-gray-700 z-20 border-r border-gray-300 dark:border-gray-600`;
            } else {
                th.className = CSS_CLASSES.P1_PX2;
            }
            
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
        toggleButtonContainer.className = `pt-3 ${CSS_CLASSES.TEXT_RIGHT}`;

        const showMoreButton = document.createElement('button');
        showMoreButton.className = `${CSS_CLASSES.PBS_SHOW_MORE} px-3 py-1 text-xs text-blue-600 dark:text-blue-400 border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors duration-150 ${CSS_CLASSES.HIDDEN}`;
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

    const _createAllTaskSectionsContainer = (instanceId, pbsInstanceData) => {
        const container = document.createElement('div');
        container.className = CSS_CLASSES.SPACE_Y_4;

        const taskDefinitions = [
            { type: 'backup', title: 'Backup', idCol: 'Guest', tableIdPrefix: ID_PREFIXES.PBS_RECENT_BACKUP_TASKS_TABLE, data: pbsInstanceData?.backupTasks },
            { type: 'verify', title: 'Verification', idCol: 'Guest/Group', tableIdPrefix: ID_PREFIXES.PBS_RECENT_VERIFY_TASKS_TABLE, data: pbsInstanceData?.verificationTasks },
            { type: 'sync', title: 'Sync', idCol: 'Job ID', tableIdPrefix: ID_PREFIXES.PBS_RECENT_SYNC_TASKS_TABLE, data: pbsInstanceData?.syncTasks },
            { type: 'prunegc', title: 'Prune/GC', idCol: 'Datastore/Group', tableIdPrefix: ID_PREFIXES.PBS_RECENT_PRUNEGC_TASKS_TABLE, data: pbsInstanceData?.pruneTasks }
        ];

        taskDefinitions.forEach(def => {
            const taskSection = document.createElement('div');
            taskSection.className = CSS_CLASSES.PBS_TASK_SECTION;
            taskSection.dataset.taskType = def.type;
            taskSection.appendChild(_createTaskTableElement(def.tableIdPrefix + instanceId, def.title, def.idCol, def.data));
            container.appendChild(taskSection);
        });
        return container;
    };

    const _createPbsNodeStatusSection = (instanceId, pbsInstanceData) => {
        const sectionDiv = document.createElement('div');
        sectionDiv.id = `pbs-node-status-section-${instanceId}`;
        sectionDiv.className = `${CSS_CLASSES.MB4}`;

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
        thead.className = `${CSS_CLASSES.STICKY} ${CSS_CLASSES.TOP_0} ${CSS_CLASSES.Z_10} ${CSS_CLASSES.BG_GRAY_100_DARK_BG_GRAY_700}`;
        const headerRow = document.createElement('tr');
        headerRow.className = `${CSS_CLASSES.TEXT_XS} ${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TEXT_LEFT} ${CSS_CLASSES.TEXT_GRAY_600_UPPERCASE_DARK_TEXT_GRAY_300} ${CSS_CLASSES.BORDER_B_GRAY_300_DARK_BORDER_GRAY_600}`;
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
        
        // Use consolidated table row helper
        const valueRow = PulseApp.ui.common.createTableRow();

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
            const cpuPercent = parseFloat((nodeStatus.cpu * 100).toFixed(1));
            const cpuColorClass = PulseApp.utils.getUsageColor(cpuPercent, 'cpu');
            const cpuTooltipText = `${cpuPercent}%`;
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
            const memPercent = parseFloat(((memUsed && memTotal > 0) ? (memUsed / memTotal * 100) : 0).toFixed(1));
            const memColorClass = PulseApp.utils.getUsageColor(memPercent, 'memory');
            const memTooltipText = `${PulseApp.utils.formatBytes(memUsed)} / ${PulseApp.utils.formatBytes(memTotal)} (${memPercent}%)`;
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
        instanceWrapper.className = `${CSS_CLASSES.PBS_INSTANCE_SECTION} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} ${CSS_CLASSES.P4} bg-gray-50/30 dark:bg-gray-800/30`;
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
            detailsContainer.appendChild(_createAllTaskSectionsContainer(instanceId, pbsInstanceData));
            
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
                    <div class="h-2 rounded-full transition-all duration-300 ${cpuColorClass}" style="width: ${cpuPercent.toFixed(1)}%"></div>
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
                    <div class="h-2 rounded-full transition-all duration-300 ${memColorClass}" style="width: ${memPercent.toFixed(1)}%"></div>
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
        // Create a wrapper for better mobile handling
        const tabWrapper = document.createElement('div');
        tabWrapper.className = 'mb-6';
        
        // For mobile or many instances, consider using a dropdown
        const useDropdown = pbsArray.length > 4 || window.innerWidth < 768;
        
        if (useDropdown) {
            // Create dropdown for mobile or many instances
            const dropdownContainer = document.createElement('div');
            dropdownContainer.className = 'mb-4';
            
            const dropdownLabel = document.createElement('label');
            dropdownLabel.className = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2';
            dropdownLabel.textContent = 'Select PBS Instance:';
            
            const dropdown = document.createElement('select');
            dropdown.className = 'block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-gray-200';
            
            pbsArray.forEach((pbsInstance, index) => {
                const option = document.createElement('option');
                option.value = index;
                option.textContent = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
                dropdown.appendChild(option);
            });
            
            // Ensure selected index is valid
            const safeDropdownIndex = Math.max(0, Math.min(selectedPbsTabIndex, pbsArray.length - 1));
            dropdown.value = safeDropdownIndex;
            selectedPbsTabIndex = safeDropdownIndex;
            
            dropdown.addEventListener('change', (e) => {
                selectedPbsTabIndex = parseInt(e.target.value);
                _renderPbsInstanceContent(pbsArray[selectedPbsTabIndex], selectedPbsTabIndex, pbsArray, tabContentArea);
            });
            
            dropdownContainer.appendChild(dropdownLabel);
            dropdownContainer.appendChild(dropdown);
            mainContainer.appendChild(dropdownContainer);
            
            const tabContentArea = document.createElement('div');
            tabContentArea.className = CSS_CLASSES.PBS_TAB_CONTENT_AREA;
            mainContainer.appendChild(tabContentArea);
            
            // Render initial content
            _renderPbsInstanceContent(pbsArray[safeDropdownIndex], safeDropdownIndex, pbsArray, tabContentArea);
            return;
        }
        
        // Tab header with instance selector for desktop
        const tabHeader = document.createElement('div');
        tabHeader.className = 'bg-gray-50 dark:bg-gray-800 rounded-t-lg border border-gray-200 dark:border-gray-700 p-2';
        
        const tabContainer = document.createElement('div');
        tabContainer.className = 'flex flex-wrap gap-2 overflow-x-auto';

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
            
            // Calculate instance health
            const overallHealth = _calculateOverallHealth(pbsInstance);
            const namespaceCount = _collectNamespaces(pbsArray, index).length;

            const tabButton = document.createElement('button');
            tabButton.id = `${ID_PREFIXES.PBS_TAB_BUTTON_PREFIX}${instanceId}`;
            tabButton.className = 'flex items-center gap-2 px-4 py-2 rounded-lg border-2 border-transparent font-medium text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 hover:bg-white dark:hover:bg-gray-700 hover:border-gray-300 dark:hover:border-gray-600 transition-all focus:outline-none focus:ring-2 focus:ring-blue-500';
            
            // Create tab content with health indicator and info
            const tabContent = document.createElement('div');
            tabContent.className = 'flex items-center gap-2';
            
            // Health indicator
            const healthDot = document.createElement('span');
            healthDot.className = 'w-2 h-2 rounded-full';
            if (overallHealth.overallHealth === 'ok') {
                healthDot.classList.add('bg-green-500');
            } else if (overallHealth.overallHealth === 'warning') {
                healthDot.classList.add('bg-yellow-500');
            } else if (overallHealth.overallHealth === 'error') {
                healthDot.classList.add('bg-red-500');
            }
            
            // Instance name
            const nameSpan = document.createElement('span');
            nameSpan.textContent = instanceName;
            
            // Namespace count badge
            if (namespaceCount > 1) {
                const nsBadge = document.createElement('span');
                nsBadge.className = 'ml-1 px-2 py-0.5 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded-full';
                nsBadge.textContent = `${namespaceCount} NS`;
                tabContent.appendChild(healthDot);
                tabContent.appendChild(nameSpan);
                tabContent.appendChild(nsBadge);
            } else {
                tabContent.appendChild(healthDot);
                tabContent.appendChild(nameSpan);
            }
            
            tabButton.appendChild(tabContent);
            tabButton.dataset.instanceId = instanceId;
            tabButton.dataset.instanceIndex = index;

            tabButton.addEventListener('click', (event) => {
                // Update all tabs to unselected state
                tabContainer.querySelectorAll('button').forEach(btn => {
                    btn.classList.remove('bg-blue-500', 'text-white', 'border-blue-500', 'hover:bg-white', 'hover:text-gray-800', 'dark:hover:bg-gray-700', 'dark:hover:text-gray-200');
                    btn.classList.add('text-gray-600', 'dark:text-gray-400', 'hover:text-gray-800', 'dark:hover:text-gray-200', 'hover:bg-white', 'dark:hover:bg-gray-700');
                });
                
                // Style selected tab
                const currentBtn = event.currentTarget;
                currentBtn.classList.remove('text-gray-600', 'dark:text-gray-400', 'hover:text-gray-800', 'dark:hover:text-gray-200', 'hover:bg-white', 'dark:hover:bg-gray-700');
                currentBtn.classList.add('bg-blue-500', 'text-white', 'border-blue-500');

                const selectedIndex = parseInt(event.currentTarget.dataset.instanceIndex);
                selectedPbsTabIndex = selectedIndex; // Update global selected tab index
                
                // Use helper function to render content
                _renderPbsInstanceContent(pbsArray[selectedIndex], selectedIndex, pbsArray, tabContentArea);
            });
            tabContainer.appendChild(tabButton);
            tabButtons.push(tabButton);
        });

        // Add tabs to header
        tabHeader.appendChild(tabContainer);
        tabWrapper.appendChild(tabHeader);
        
        // Add content area to wrapper
        tabWrapper.appendChild(tabContentArea);
        
        // Add wrapper to main container
        mainContainer.appendChild(tabWrapper);

        // Auto-select the previously selected tab if possible, else the first tab
        const safeIndex = Math.max(0, Math.min(selectedPbsTabIndex, tabButtons.length - 1));
        if (tabButtons[safeIndex]) {
            tabButtons[safeIndex].click();
        }
    }

    function _renderPbsInstanceContent(pbsInstance, instanceIndex, pbsArray, container) {
        container.innerHTML = '';
        
        // Get namespaces for this instance
        const instanceNamespaces = _collectNamespaces(pbsArray, instanceIndex);
        const hasMultipleNamespaces = instanceNamespaces.length > 1;
        
        // Add namespace tabs if multiple namespaces exist for this instance
        if (hasMultipleNamespaces) {
            const namespaceTabs = _createNamespaceTabs(instanceNamespaces, container, instanceIndex);
            container.appendChild(namespaceTabs);
            
            // Add a divider
            const divider = document.createElement('hr');
            divider.className = 'my-4 border-gray-200 dark:border-gray-700';
            container.appendChild(divider);
        }
        
        // Get selected namespace for this instance
        const selectedNamespace = hasMultipleNamespaces 
            ? (selectedNamespaceByInstance.get(instanceIndex) || 'root')
            : null;
        
        // Filter the instance data by namespace
        const filteredInstanceData = _filterPbsInstanceByNamespace(pbsInstance, selectedNamespace);
        
        let baseId;
        if (filteredInstanceData.pbsInstanceName) {
            baseId = PulseApp.utils.sanitizeForId(filteredInstanceData.pbsInstanceName);
        } else if (filteredInstanceData.pbsEndpointId) {
            baseId = PulseApp.utils.sanitizeForId(filteredInstanceData.pbsEndpointId);
        } else {
            baseId = 'pbs-instance';
        }
        const instanceId = `${baseId}-${instanceIndex}`;
        const instanceName = filteredInstanceData.pbsInstanceName || `PBS Instance ${instanceIndex + 1}`;
        
        const overallHealthAndTitle = _calculateOverallHealth(filteredInstanceData);
        const statusInfo = _getInstanceStatusInfo(filteredInstanceData);

        const instanceElement = _createPbsInstanceElement(
            filteredInstanceData,
            instanceId,
            instanceName,
            overallHealthAndTitle.overallHealth,
            overallHealthAndTitle.healthTitle,
            statusInfo.showDetails,
            statusInfo.statusText
        );
        
        container.appendChild(instanceElement);
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

    // Collect all namespaces from PBS data
    function _collectNamespaces(pbsDataArray, instanceIndex = null) {
        const namespaces = new Set(['root']); // Always include root
        
        // If instanceIndex is provided, collect only from that instance
        const instancesToCheck = instanceIndex !== null 
            ? [pbsDataArray[instanceIndex]]
            : pbsDataArray;
        
        instancesToCheck.forEach(pbsInstance => {
            // Check datastore snapshots
            if (pbsInstance.datastores) {
                pbsInstance.datastores.forEach(ds => {
                    if (ds.snapshots) {
                        ds.snapshots.forEach(snap => {
                            namespaces.add(snap.namespace || 'root');
                        });
                    }
                });
            }
            
            // Check all task types
            const taskTypes = ['backupTasks', 'verificationTasks', 'syncTasks', 'pruneTasks'];
            taskTypes.forEach(taskType => {
                if (pbsInstance[taskType] && pbsInstance[taskType].recentTasks) {
                    pbsInstance[taskType].recentTasks.forEach(task => {
                        namespaces.add(task.namespace || 'root');
                    });
                }
            });
        });
        
        return Array.from(namespaces).sort((a, b) => {
            // Root namespace first, then alphabetical
            if (a === 'root') return -1;
            if (b === 'root') return 1;
            return a.localeCompare(b);
        });
    }
    
    // Create namespace tabs
    function _createNamespaceTabs(namespaces, container, instanceIndex = null) {
        const tabsContainer = document.createElement('div');
        tabsContainer.className = 'mb-4';
        
        // Add a label for namespace selection
        const nsLabel = document.createElement('div');
        nsLabel.className = 'text-sm font-medium text-gray-700 dark:text-gray-300 mb-2';
        nsLabel.textContent = 'Select Namespace:';
        tabsContainer.appendChild(nsLabel);
        
        const tabsList = document.createElement('nav');
        tabsList.className = 'flex flex-wrap gap-2';
        tabsList.setAttribute('aria-label', 'Namespace tabs');
        
        // Get selected namespace for this instance
        const selectedNamespace = instanceIndex !== null 
            ? (selectedNamespaceByInstance.get(instanceIndex) || 'root')
            : selectedNamespaceTab;
        
        namespaces.forEach(namespace => {
            const tab = document.createElement('button');
            tab.className = namespace === selectedNamespace 
                ? 'px-4 py-2 rounded-full bg-blue-500 text-white text-sm font-medium transition-colors'
                : 'px-4 py-2 rounded-full bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm font-medium hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors';
            tab.textContent = namespace === 'root' ? 'Root' : namespace;
            tab.dataset.namespace = namespace;
            
            tab.addEventListener('click', () => {
                // Clear cached status info when namespace changes
                taskStatusInfo.clear();
                
                if (instanceIndex !== null) {
                    // Multi-instance mode: track per instance
                    selectedNamespaceByInstance.set(instanceIndex, namespace);
                } else {
                    // Single instance mode
                    selectedNamespaceTab = namespace;
                }
                
                // Re-render PBS info with current data
                const pbsData = PulseApp.state.get('pbsDataArray');
                if (pbsData) {
                    updatePbsInfo(pbsData);
                }
            });
            
            tabsList.appendChild(tab);
        });
        
        tabsContainer.appendChild(tabsList);
        return tabsContainer;
    }


    // Filter PBS tasks by namespace
    function _filterPbsTasksByNamespace(tasks, selectedNamespace) {
        if (!selectedNamespace) {
            // No filtering needed (only root namespace exists)
            return tasks;
        }
        
        return tasks.filter(task => (task.namespace || 'root') === selectedNamespace);
    }
    
    // Filter PBS instance data by namespace
    function _filterPbsInstanceByNamespace(pbsInstance, selectedNamespace) {
        if (!selectedNamespace) {
            // No filtering needed
            return pbsInstance;
        }
        
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
        if (filteredInstance.datastores && selectedNamespace) {
            filteredInstance.datastores = filteredInstance.datastores.map(ds => ({
                ...ds,
                snapshots: ds.snapshots ? ds.snapshots.filter(snap => 
                    (snap.namespace || 'root') === selectedNamespace
                ) : []
            }));
        }
        
        return filteredInstance;
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

    // Track last data hash to avoid unnecessary rebuilds
    let lastPbsDataHash = null;
    
    function updatePbsInfo(pbsArray) {
        const container = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        if (!container) {
            console.error(`PBS container element #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found!`);
            return;
        }
        
        // Generate a simple hash of the data to detect changes
        const currentDataHash = JSON.stringify({
            length: pbsArray?.length || 0,
            namespaces: pbsArray ? _collectNamespaces(pbsArray).join(',') : '',
            selectedNamespace: selectedNamespaceTab, // Include selected namespace in hash
            taskCounts: pbsArray?.map(pbs => ({
                backup: pbs.backupTasks?.recentTasks?.length || 0,
                verify: pbs.verificationTasks?.recentTasks?.length || 0,
                sync: pbs.syncTasks?.recentTasks?.length || 0,
                prune: pbs.pruneTasks?.recentTasks?.length || 0
            }))
        });
        
        // If data hasn't changed, don't rebuild the DOM
        if (lastPbsDataHash === currentDataHash) {
            return;
        }
        lastPbsDataHash = currentDataHash;
        
        // Store scroll positions of all scrollable elements before clearing
        const scrollPositions = new Map();
        const scrollableElements = container.querySelectorAll('.overflow-x-auto, .overflow-auto, .overflow-y-auto, .overflow-scroll');
        scrollableElements.forEach(el => {
            if (el.scrollLeft > 0 || el.scrollTop > 0) {
                // Try to get a unique identifier for this element
                let identifier = el.id;
                if (!identifier) {
                    // Try to find the closest table's ID
                    const table = el.querySelector('table[id]') || el.closest('table[id]');
                    if (table) {
                        identifier = table.id;
                    } else {
                        // Use the closest parent with an ID
                        const parent = el.closest('[id]');
                        identifier = parent ? parent.id : el.className;
                    }
                }
                
                scrollPositions.set(identifier, {
                    left: el.scrollLeft,
                    top: el.scrollTop,
                    width: el.scrollWidth,
                    height: el.scrollHeight
                });
            }
        });
        
        // Clear container
        container.innerHTML = '';

        const loadingMessage = document.getElementById('pbs-loading-message');
        if (loadingMessage) {
            loadingMessage.remove();
        }

        if (!pbsArray || pbsArray.length === 0) {
            const placeholder = document.createElement('p');
            placeholder.className = CSS_CLASSES.TEXT_GRAY_P4_CENTER;
            placeholder.textContent = 'Proxmox Backup Server integration is not configured.';
            container.appendChild(placeholder);
            return;
        }

        // Use same layout for all viewports
        if (pbsArray.length === 1) {
            const pbsInstance = pbsArray[0];
            
            // Collect namespaces for single instance
            const allNamespaces = _collectNamespaces(pbsArray);
            const hasMultipleNamespaces = allNamespaces.length > 1;
            
            // Add namespace tabs if multiple namespaces exist
            if (hasMultipleNamespaces) {
                const tabsContainer = _createNamespaceTabs(allNamespaces, container);
                container.appendChild(tabsContainer);
            }
            
            // Get current namespace filter selection
            const selectedNamespace = hasMultipleNamespaces ? selectedNamespaceTab : null;
            
            // Filter the instance by namespace
            const filteredInstance = _filterPbsInstanceByNamespace(pbsInstance, selectedNamespace);
            let baseId;
            if (filteredInstance.pbsInstanceName) {
                baseId = PulseApp.utils.sanitizeForId(filteredInstance.pbsInstanceName);
            } else if (filteredInstance.pbsEndpointId) {
                baseId = PulseApp.utils.sanitizeForId(filteredInstance.pbsEndpointId);
            } else {
                baseId = 'pbs-instance';
            }
            const instanceId = `${baseId}-0`;
            const instanceName = filteredInstance.pbsInstanceName || `PBS Instance 1`;
            
            const overallHealthAndTitle = _calculateOverallHealth(filteredInstance);
            const statusInfo = _getInstanceStatusInfo(filteredInstance);

            // Use same layout for all viewports
            const instanceElement = _createPbsInstanceElement(
                filteredInstance,
                instanceId,
                instanceName,
                overallHealthAndTitle.overallHealth,
                overallHealthAndTitle.healthTitle,
                statusInfo.showDetails,
                statusInfo.statusText
            );
            container.appendChild(instanceElement);
        } else {
            // Multiple instances - use tabs for all viewports
            _createPbsInstanceTabs(pbsArray, container);
        }
        
        // Restore scroll positions after DOM is rebuilt
        if (scrollPositions.size > 0) {
            // Wait for next frame to ensure DOM is ready
            requestAnimationFrame(() => {
                // Wait one more frame to ensure layout is complete
                requestAnimationFrame(() => {
                    scrollPositions.forEach((position, identifier) => {
                        let element = null;
                        
                        // Try to find the scrollable container by table ID
                        const table = container.querySelector(`#${identifier}`);
                        if (table) {
                            element = table.closest('.overflow-x-auto, .overflow-auto') || table.parentElement;
                        }
                        
                        // If not found, try element by ID directly
                        if (!element) {
                            element = container.querySelector(`#${identifier}`);
                        }
                        
                        // If still not found, try to find a parent with the ID that contains a scrollable element
                        if (!element) {
                            const parent = container.querySelector(`#${identifier}`);
                            if (parent) {
                                element = parent.querySelector('.overflow-x-auto, .overflow-auto');
                            }
                        }
                        
                        if (element && element.scrollWidth > element.clientWidth) {
                            // Only restore if the element is actually scrollable
                            element.scrollLeft = position.left;
                            if (element.scrollHeight > element.clientHeight) {
                                element.scrollTop = position.top;
                            }
                        }
                    });
                });
            });
        }
        
        // Initialize fixed table line for mobile
        _initTableFixedLine();
    }

    function _initTableFixedLine() {
        // No longer needed - using CSS border styling instead
    }

    function initPbsEventListeners() {
        const pbsInstancesContainer = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        if (!pbsInstancesContainer) {
            console.warn(`PBS instances container #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found. Some UI interactions might not work.`);
            return;
        }
    }

    return {
        updatePbsInfo,
        initPbsEventListeners
    };
})();
