PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {

    // Global state tracker for expanded PBS tasks
    let expandedTaskState = new Set();
    let expandedShowMoreState = new Set();

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

    const getPbsStatusIcon = (status) => {
        if (status === 'OK') {
            return `<span class="${CSS_CLASSES.TEXT_GREEN_500_DARK_GREEN_400}" title="OK">✓</span>`;
        } else if (status === 'running') {
            return `<span class="${CSS_CLASSES.INLINE_BLOCK} ${CSS_CLASSES.ANIMATE_SPIN} ${CSS_CLASSES.ROUNDED_FULL} ${CSS_CLASSES.H_3} ${CSS_CLASSES.W_3} ${CSS_CLASSES.BORDER_T_2} ${CSS_CLASSES.BORDER_B_2} ${CSS_CLASSES.BORDER_BLUE_500}" title="Running"></span>`;
        } else if (status) {
            return `<span class="${CSS_CLASSES.TEXT_RED_500_DARK_RED_400} ${CSS_CLASSES.FONT_BOLD}" title="${status}">✗</span>`;
        } else {
            return `<span class="${CSS_CLASSES.TEXT_GRAY_400}" title="Unknown">?</span>`;
        }
    };

    const getPbsStatusDisplay = (status) => {
        if (status === 'OK') {
            return `<span class="${CSS_CLASSES.TEXT_GREEN_500_DARK_GREEN_400}">✓ OK</span>`;
        } else if (status === 'running') {
            return `<span class="${CSS_CLASSES.INLINE_BLOCK} ${CSS_CLASSES.ANIMATE_SPIN} ${CSS_CLASSES.ROUNDED_FULL} ${CSS_CLASSES.H_3} ${CSS_CLASSES.W_3} ${CSS_CLASSES.BORDER_T_2} ${CSS_CLASSES.BORDER_B_2} ${CSS_CLASSES.BORDER_BLUE_500}" title="Running"></span> <span class="${CSS_CLASSES.TEXT_BLUE_600_DARK_BLUE_400}">Running</span>`;
        } else if (status) {
            // For failed tasks, show the full error message
            const shortStatus = status.length > 50 ? `${status.substring(0, 47)}...` : status;
            return `<span class="${CSS_CLASSES.TEXT_RED_500_DARK_RED_400} ${CSS_CLASSES.FONT_BOLD}" title="${status}">✗</span> <span class="${CSS_CLASSES.TEXT_RED_600_DARK_RED_400} ${CSS_CLASSES.TEXT_XS}" title="${status}">${shortStatus}</span>`;
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
            rowClasses += ` ${CSS_CLASSES.HOVER_BG_GRAY_50_DARK_HOVER_BG_GRAY_700_50}`;
        }
        
        row.className = rowClasses;

        const targetCell = document.createElement('td');
        targetCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        
        // Add expand indicator for failed tasks
        if (isFailed) {
            targetCell.innerHTML = `<span class="text-xs text-gray-400 mr-1">▶</span>${target}`;
        } else {
            targetCell.textContent = target;
        }
        row.appendChild(targetCell);

        const statusCell = document.createElement('td');
        statusCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} min-w-48`;
        statusCell.innerHTML = statusDisplayHTML;
        row.appendChild(statusCell);

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
                event.preventDefault();  // Prevent any default behavior
                
                const upid = task.upid;
                const existingDetailRow = row.nextElementSibling;
                
                if (existingDetailRow && existingDetailRow.classList.contains('task-detail-row')) {
                    // Toggle existing detail row - collapse
                    existingDetailRow.remove();
                    targetCell.innerHTML = `<span class="text-xs text-gray-400 mr-1">▶</span>${target}`;
                    expandedTaskState.delete(upid); // Remove from global state
                } else {
                    // Create and show detail row - expand
                    const detailRow = _createTaskDetailRow(task);
                    row.insertAdjacentElement('afterend', detailRow);
                    targetCell.innerHTML = `<span class="text-xs text-gray-400 mr-1">▼</span>${target}`;
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
        detailCell.colSpan = 5;
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

      // Use global expanded state instead of scanning DOM
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
                  targetCell.innerHTML = `<span class="text-xs text-gray-400 mr-1">▼</span>${target}`;
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
                  
                  // Clear existing handlers and attach new one
                  showMoreButton.replaceWith(showMoreButton.cloneNode(true));
                  const newShowMoreButton = parentSectionElement.querySelector(`.${CSS_CLASSES.PBS_SHOW_MORE}`);
                  
                  newShowMoreButton.addEventListener('click', () => {
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
              } else {
                  showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
                  // Remove from state if there are no more tasks to show
                  if (tableId && expandedShowMoreState.has(tableId)) {
                      expandedShowMoreState.delete(tableId);
                  }
              }
          }
      }
  }

    const _populateDsTableBody = (dsTableBody, datastores, statusText, showDetails) => {
        if (!dsTableBody) return;
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
                        nameContent = `⚠ ${nameContent} [CRITICAL: ${usagePercent}% full]`;
                        createCell(nameContent, ['text-red-700', 'dark:text-red-300', 'font-semibold']);
                    } else if (usagePercent >= 85) {
                        nameContent = `⚠ ${nameContent} [WARNING: ${usagePercent}% full]`;
                        createCell(nameContent, ['text-yellow-700', 'dark:text-yellow-300', 'font-semibold']);
                    } else {
                        createCell(nameContent);
                    }

                    createCell(ds.path || 'N/A', [CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400]);
                    
                    // Simplified cell creation - the data is actually coming through correctly
                    createCell(ds.used !== null ? PulseApp.utils.formatBytes(ds.used) : 'N/A');
                    createCell(ds.available !== null ? PulseApp.utils.formatBytes(ds.available) : 'N/A');
                    createCell(ds.total !== null ? PulseApp.utils.formatBytes(ds.total) : 'N/A');

                    // Create usage cell with better formatting
                    const usageCell = row.insertCell();
                    usageCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.WHITESPACE_NOWRAP}`;
                    usageCell.style.minWidth = '150px';
                    if (totalBytes > 0) {
                        usageCell.innerHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor);
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
        instanceTitleElement.appendChild(document.createTextNode(instanceName));
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
        heading.textContent = 'PBS Task Summary (Last 30 Days)';
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
                statusHtml = `<span class="${CSS_CLASSES.TEXT_RED_600_DARK_RED_400} ${CSS_CLASSES.FONT_BOLD}">⚠ ${failed} FAILED</span>`;
                if (ok > 0) {
                    statusHtml += ` / <span class="${CSS_CLASSES.TEXT_GREEN_600_DARK_GREEN_400}">${ok} OK</span>`;
                }
            } else if (ok > 0) {
                statusHtml = `<span class="${CSS_CLASSES.TEXT_GREEN_600_DARK_GREEN_400}">✓ All OK (${ok})</span>`;
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
        heading.innerHTML = `Recent ${title} Tasks <span id="${tableId}-status" class="text-xs font-normal text-gray-500"></span><span id="${tableId}-priority" class="text-xs text-red-600 dark:text-red-400 ml-2 hidden">⚠ Failed tasks shown first</span>`;
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

        const headers = [idColumnHeader, 'Status', 'Start Time', 'Duration', 'UPID'];
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
        heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} mb-2 ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        heading.textContent = 'Server Status';
        sectionDiv.appendChild(heading);

        const statusContainer = document.createElement('div');
        statusContainer.className = `${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} ${CSS_CLASSES.BG_GRAY_50_DARK_BG_GRAY_800_30} p-2`; // Reduced padding for a tighter overall look
        
        const nodeStatus = pbsInstanceData.nodeStatus || {};
        const versionInfo = pbsInstanceData.versionInfo || {};
        
        const metricsRow = document.createElement('div');
        metricsRow.className = 'flex flex-wrap -mx-px'; // Use negative margin to counteract border width on items

        const createMetricBlock = (label, value, details, isLast = false) => {
            const block = document.createElement('div');
            let blockClasses = 'flex-1 px-2 py-1 text-center min-w-[80px]'; // min-width to prevent excessive squishing
            if (!isLast) {
                blockClasses += ' border-r border-gray-300 dark:border-gray-600';
            }
            block.className = blockClasses;
            
            let innerHTML = `<div class="text-xs text-gray-500 dark:text-gray-400">${label}</div>
                             <div class="text-sm font-semibold text-gray-900 dark:text-gray-100">${value}</div>`;
            if (details) {
                innerHTML += `<div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">${details}</div>`;
            }
            block.innerHTML = innerHTML;
            return block;
        };

        const metricItems = [];

        // PBS Version
        if (versionInfo.version) {
            const versionText = versionInfo.release ? `${versionInfo.version}-${versionInfo.release}` : versionInfo.version;
            metricItems.push({ label: 'PBS VER', value: versionText });
        }

        // Uptime
        if (nodeStatus.uptime) {
            metricItems.push({ label: 'UPTIME', value: PulseApp.utils.formatUptime(nodeStatus.uptime) });
        }

        // CPU Usage
        if (nodeStatus.cpu !== null && nodeStatus.cpu !== undefined) {
            metricItems.push({ label: 'CPU', value: `${Math.round(nodeStatus.cpu * 100)}%` });
        }
        
        // Memory Usage
        if (nodeStatus.memory && nodeStatus.memory.total && nodeStatus.memory.used !== null) {
            const memoryPercent = Math.round((nodeStatus.memory.used / nodeStatus.memory.total) * 100);
            const memoryDetails = `(${PulseApp.utils.formatBytes(nodeStatus.memory.used)} / ${PulseApp.utils.formatBytes(nodeStatus.memory.total)})`;
            metricItems.push({ label: 'MEMORY', value: `${memoryPercent}%`, details: memoryDetails });
        }

        // Load Average (1-min)
        if (nodeStatus.loadavg && Array.isArray(nodeStatus.loadavg) && nodeStatus.loadavg.length >= 1) {
            metricItems.push({ label: 'LOAD', value: nodeStatus.loadavg[0].toFixed(2) });
        }

        metricItems.forEach((item, index) => {
            metricsRow.appendChild(createMetricBlock(item.label, item.value, item.details, index === metricItems.length - 1));
        });
        
        statusContainer.appendChild(metricsRow);
        sectionDiv.appendChild(statusContainer);
        
        return sectionDiv;
    };

    const _createPbsInstanceElement = (pbsInstanceData, instanceId, instanceName, overallHealth, healthTitle, showDetails, statusText) => {
        const instanceWrapper = document.createElement('div');
        instanceWrapper.className = `${CSS_CLASSES.PBS_INSTANCE_SECTION} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} p-4 mb-4 bg-gray-50/30 dark:bg-gray-800/30`;
        instanceWrapper.id = ID_PREFIXES.PBS_INSTANCE + instanceId;

        instanceWrapper.appendChild(_createInstanceHeaderDiv(instanceName, overallHealth, healthTitle));

        const detailsContainer = document.createElement('div');
        detailsContainer.className = `${CSS_CLASSES.PBS_INSTANCE_DETAILS} ${CSS_CLASSES.SPACE_Y_4} ${showDetails ? '' : CSS_CLASSES.HIDDEN}`;
        detailsContainer.id = ID_PREFIXES.PBS_DETAILS + instanceId;

        detailsContainer.appendChild(_createPbsNodeStatusSection(instanceId, pbsInstanceData));
        detailsContainer.appendChild(_createDatastoreSectionElement(instanceId));
        detailsContainer.appendChild(_createSummariesSectionElement(instanceId, pbsInstanceData));
        detailsContainer.appendChild(_createAllTaskSectionsContainer(instanceId));
        
        instanceWrapper.appendChild(detailsContainer);

        const dsTableBodyElement = instanceWrapper.querySelector(`#${ID_PREFIXES.PBS_DS_TBODY}${instanceId}`);
        _populateDsTableBody(dsTableBodyElement, pbsInstanceData.datastores, statusText, showDetails);
        _populateInstanceTaskSections(detailsContainer, instanceId, pbsInstanceData, statusText, showDetails);

        return instanceWrapper;
    };
    // END: Definitions for functions that _createPbsInstanceElement depends on

    function _createPbsInstanceTabs(pbsArray, mainContainer) {
        const tabContainer = document.createElement('div');
        tabContainer.className = CSS_CLASSES.PBS_TAB_CONTAINER;

        const tabContentArea = document.createElement('div');
        tabContentArea.className = CSS_CLASSES.PBS_TAB_CONTENT_AREA;

        pbsArray.forEach((pbsInstance, index) => {
            let rawInstanceId = pbsInstance.pbsEndpointId || `instance-${index}`;
            let instanceId = PulseApp.utils.sanitizeForId(rawInstanceId);
            instanceId = `${instanceId}-${index}`;
            const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;

            const tabButton = document.createElement('button');
            tabButton.id = `${ID_PREFIXES.PBS_TAB_BUTTON_PREFIX}${instanceId}`;
            tabButton.className = CSS_CLASSES.PBS_TAB_BUTTON;
            tabButton.textContent = instanceName;
            tabButton.dataset.instanceId = instanceId;
            tabButton.dataset.instanceIndex = index;

            tabButton.addEventListener('click', (event) => {
                tabContainer.querySelectorAll('button').forEach(btn => {
                    btn.classList.remove(CSS_CLASSES.PBS_TAB_BUTTON_ACTIVE);
                });
                event.currentTarget.classList.add(CSS_CLASSES.PBS_TAB_BUTTON_ACTIVE);

                tabContentArea.innerHTML = '';

                const selectedInstanceData = pbsArray[parseInt(event.currentTarget.dataset.instanceIndex)];
                const overallHealthAndTitle = _calculateOverallHealth(selectedInstanceData);
                const statusInfo = _getInstanceStatusInfo(selectedInstanceData);

                const instanceElement = _createPbsInstanceElement(
                    selectedInstanceData,
                    instanceId,
                    instanceName,
                    overallHealthAndTitle.overallHealth,
                    overallHealthAndTitle.healthTitle,
                    statusInfo.showDetails,
                    statusInfo.statusText
                );
                tabContentArea.appendChild(instanceElement);
            });
            tabContainer.appendChild(tabButton);
        });

        mainContainer.appendChild(tabContainer);
        mainContainer.appendChild(tabContentArea);

        if (tabContainer.firstChild) {
            tabContainer.firstChild.click();
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

    function updatePbsInfo(pbsArray) {
      const container = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
      if (!container) {
          console.error(`PBS container element #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found!`);
          return;
      }
      container.innerHTML = '';

      const loadingMessage = document.getElementById('pbs-loading-message');
      if (loadingMessage) {
          loadingMessage.remove();
      }

      if (!pbsArray || pbsArray.length === 0) {
          const placeholder = document.createElement('p');
          placeholder.className = CSS_CLASSES.TEXT_GRAY_500_DARK_TEXT_GRAY_400_P4_TEXT_CENTER_TEXT_SM;
          placeholder.textContent = 'Proxmox Backup Server integration is not configured.';
          container.appendChild(placeholder);
          return;
      }

      if (pbsArray.length === 1) {
          const pbsInstance = pbsArray[0];
          const rawInstanceId = pbsInstance.pbsEndpointId || `instance-0`;
          let instanceId = PulseApp.utils.sanitizeForId(rawInstanceId);
          instanceId = `${instanceId}-0`;
          const instanceName = pbsInstance.pbsInstanceName || `PBS Instance 1`;
          
          const overallHealthAndTitle = _calculateOverallHealth(pbsInstance);
          const statusInfo = _getInstanceStatusInfo(pbsInstance);

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
      } else {
          _createPbsInstanceTabs(pbsArray, container);
      }
  }

    function initPbsEventListeners() {
        const pbsInstancesContainer = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        if (!pbsInstancesContainer) {
            console.warn(`PBS instances container #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found. Some UI interactions might not work.`);
        }
    }

    return {
        updatePbsInfo,
        initPbsEventListeners
    };
})();
