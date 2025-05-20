 PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {

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
        PBS_INSTANCES_CONTAINER: 'pbs-instances-container', // Kept for reference, not primary use now
        PBS_SUB_TABS_NAV: 'pbs-sub-tabs-nav',
        PBS_SUB_TABS_CONTENT: 'pbs-sub-tabs-content',
        PBS_SUB_TAB_CONTENT_PANEL_PREFIX: 'pbs-instance-content-'
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
        const statusIconHTML = getPbsStatusIcon(task.status);
        const startTime = task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A';
        const duration = task.duration !== null ? PulseApp.utils.formatDuration(task.duration) : 'N/A';
        const upid = task.upid || 'N/A';
        const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

        const row = document.createElement('tr');
        row.className = `${CSS_CLASSES.BORDER_B_GRAY_200_DARK_GRAY_700} ${CSS_CLASSES.HOVER_BG_GRAY_50_DARK_HOVER_BG_GRAY_700_50} ${CSS_CLASSES.TRANSITION_COLORS} ${CSS_CLASSES.DURATION_150} ${CSS_CLASSES.EASE_IN_OUT}`;

        const targetCell = document.createElement('td');
        targetCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
        targetCell.textContent = target;
        row.appendChild(targetCell);

        const statusCell = document.createElement('td');
        statusCell.className = `${CSS_CLASSES.P1_PX2} ${CSS_CLASSES.TEXT_SM}`;
        statusCell.innerHTML = statusIconHTML;
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

        return row;
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

      tableBody.innerHTML = '';

      const tasks = fullTasksArray || [];
      let displayedTasks = tasks.slice(0, initialLimit);

      if (tasks.length === 0) {
          if (noTasksMessage) noTasksMessage.classList.remove(CSS_CLASSES.HIDDEN);
          if (showMoreButton) showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
      } else {
          if (noTasksMessage) noTasksMessage.classList.add(CSS_CLASSES.HIDDEN);

          displayedTasks.forEach(task => {
              tableBody.appendChild(_createTaskTableRow(task));
          });

          if (showMoreButton) {
              if (tasks.length > initialLimit) {
                  showMoreButton.classList.remove(CSS_CLASSES.HIDDEN);
                  const remainingCount = tasks.length - initialLimit;
                  showMoreButton.textContent = `Show More (${remainingCount} older)`;
                  if (!showMoreButton.dataset.handlerAttached) {
                      showMoreButton.addEventListener('click', () => {
                          tasks.slice(initialLimit).forEach(task => {
                              tableBody.appendChild(_createTaskTableRow(task));
                          });
                          showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
                      });
                       showMoreButton.dataset.handlerAttached = 'true';
                  }
              } else {
                  showMoreButton.classList.add(CSS_CLASSES.HIDDEN);
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
                cell.colSpan = 7;
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
                    row.className = `${CSS_CLASSES.HOVER_BG_GRAY_50_DARK_HOVER_BG_GRAY_700_50}`;

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

                    createCell(ds.name || 'N/A');
                    createCell(ds.path || 'N/A', [CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400]);
                    createCell(PulseApp.utils.formatBytes(usedBytes));
                    createCell(PulseApp.utils.formatBytes(availableBytes));
                    createCell(totalBytes > 0 ? PulseApp.utils.formatBytes(totalBytes) : 'N/A');

                    const usageCell = createCell(totalBytes > 0 ? PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor) : '-', [], true);
                    usageCell.style.minWidth = '150px';

                    createCell(gcStatusHtml, [], true);
                });
            }
        } else {
            const row = dsTableBody.insertRow();
            const cell = row.insertCell();
            cell.colSpan = 7;
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
        // instanceTitleElement.appendChild(_createHealthBadgeHTML(overallHealth, healthTitle));
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
        ['Name', 'Path', 'Used', 'Available', 'Total', 'Usage', 'GC Status'].forEach(headerText => {
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

    function updatePbsInfo(pbsArray) {
        const pbsSubTabsNav = document.getElementById(ID_PREFIXES.PBS_SUB_TABS_NAV);
        const pbsSubTabsContent = document.getElementById(ID_PREFIXES.PBS_SUB_TABS_CONTENT);
        const pbsLoadingMessageEl = document.getElementById('pbs-loading-message'); 

        if (!pbsSubTabsNav || !pbsSubTabsContent) {
            console.error('[PBS UI] Sub-tab navigation or content area not found in HTML.');
            if (pbsLoadingMessageEl) pbsLoadingMessageEl.textContent = 'UI Error: PBS tab structure missing.';
            return;
        }

        pbsSubTabsNav.innerHTML = '';
        pbsSubTabsContent.innerHTML = ''; 

        const validPbsInstances = pbsArray ? pbsArray.filter(p => p && p.status !== 'error' && p.status !== 'pending_initialization') : [];

        if (!pbsArray || pbsArray.length === 0 || validPbsInstances.length === 0) {
            let message = 'No PBS instances configured or all are currently unavailable.';
            if (!pbsArray || pbsArray.length === 0) {
                message = 'No PBS instances configured.';
            } else if (validPbsInstances.length === 0) {
                message = 'Could not retrieve data from any configured PBS instances.';
            }
            
            const messageElement = document.createElement('p');
            messageElement.textContent = message;
            messageElement.className = `${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400_P4_TEXT_CENTER_TEXT_SM}`;
            pbsSubTabsContent.appendChild(messageElement);
            return;
        }

        pbsArray.forEach((pbsInstanceData, index) => {
            if (!pbsInstanceData || !pbsInstanceData.pbsEndpointId) {
                console.warn('[PBS UI] Skipping PBS instance due to missing data or endpoint ID:', pbsInstanceData);
                return; 
            }
            const instanceId = pbsInstanceData.pbsEndpointId;
            const instanceName = pbsInstanceData.pbsInstanceName || `PBS ${index + 1}`;

            const tabButton = document.createElement('button');
            tabButton.textContent = instanceName;
            tabButton.setAttribute('data-pbs-instance-id', instanceId);
            tabButton.className = `pbs-sub-tab-button py-2 px-4 -mb-px border-b-2 border-transparent hover:text-blue-600 dark:hover:text-blue-400 hover:border-blue-500 dark:hover:border-blue-400 focus:outline-none text-sm font-medium text-gray-500 dark:text-gray-400`;

            const contentPanel = document.createElement('div');
            contentPanel.id = `${ID_PREFIXES.PBS_SUB_TAB_CONTENT_PANEL_PREFIX}${instanceId}`;
            contentPanel.className = `pbs-instance-content-panel ${CSS_CLASSES.SPACE_Y_4}`;
            if (index !== 0) { 
                contentPanel.classList.add(CSS_CLASSES.HIDDEN);
            }

            const overallHealth = pbsInstanceData.status === 'ok' ? 'ok' : (pbsInstanceData.status || 'error');
            const healthTitle = pbsInstanceData.status === 'ok' ? 'Instance OK' : `Instance status: ${pbsInstanceData.status || 'Unknown'}`;
            const showDetails = PulseApp.state.getPbShowDetailsState(instanceId, true); 
            const statusText = (pbsInstanceData.status !== 'ok' && pbsInstanceData.status !== 'pending_initialization') ? `Instance reported status: ${pbsInstanceData.status}` : '';

            _createPbsInstanceElement(pbsInstanceData, instanceId, instanceName, overallHealth, healthTitle, showDetails, statusText, contentPanel);

            pbsSubTabsNav.appendChild(tabButton);
            pbsSubTabsContent.appendChild(contentPanel);

            // Add Click Listener to Tab Button
            tabButton.addEventListener('click', () => {
                // Deactivate all other tabs
                pbsSubTabsNav.querySelectorAll('.pbs-sub-tab-button').forEach(btn => {
                    btn.classList.remove('text-blue-600', 'dark:text-blue-300', 'border-blue-500', 'dark:border-blue-400', 'font-semibold');
                    btn.classList.add('text-gray-500', 'dark:text-gray-400', 'border-transparent');
                });
                // Activate clicked tab
                tabButton.classList.add('text-blue-600', 'dark:text-blue-300', 'border-blue-500', 'dark:border-blue-400', 'font-semibold');
                tabButton.classList.remove('text-gray-500', 'dark:text-gray-400', 'border-transparent');

                // Hide all content panels
                pbsSubTabsContent.querySelectorAll('.pbs-instance-content-panel').forEach(panel => {
                    panel.classList.add(CSS_CLASSES.HIDDEN);
                });
                // Show clicked tab's content panel
                const targetPanelId = `${ID_PREFIXES.PBS_SUB_TAB_CONTENT_PANEL_PREFIX}${tabButton.getAttribute('data-pbs-instance-id')}`;
                const targetPanel = document.getElementById(targetPanelId);
                if (targetPanel) {
                    targetPanel.classList.remove(CSS_CLASSES.HIDDEN);
                }
            });
        });

        // Activate the first tab by default if tabs were created
        if (pbsSubTabsNav.firstChild && typeof pbsSubTabsNav.firstChild.click === 'function') {
            pbsSubTabsNav.firstChild.click();
        }
    }

    const _createAllTaskSectionsContainer = (instanceId) => {
        // ... existing implementation ...
    };

    // MODIFIED: Added targetPanelElement argument, appends to it, returns instanceSection
    const _createPbsInstanceElement = (pbsInstanceData, instanceId, instanceName, overallHealth, healthTitle, showDetails, statusText, targetPanelElement) => {
        if (!targetPanelElement) {
            console.error(`[PBS UI _createPbsInstanceElement] No targetPanelElement provided for instance ${instanceName} (${instanceId})`);
            return null;
        }
        
        targetPanelElement.innerHTML = ''; // Clear the target panel first

        const instanceSection = document.createElement('div');
        instanceSection.id = `${ID_PREFIXES.PBS_INSTANCE}${instanceId}`;
        instanceSection.className = `${CSS_CLASSES.PBS_INSTANCE_SECTION} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} ${CSS_CLASSES.P3} ${CSS_CLASSES.BG_GRAY_100_50_DARK_BG_GRAY_700_50}`;
        instanceSection.classList.toggle(CSS_CLASSES.BORDER_L_4_RED_500_DARK_RED_400, overallHealth === 'error');
        instanceSection.classList.toggle(CSS_CLASSES.BORDER_L_4_TRANSPARENT, overallHealth !== 'error');

        const headerDiv = _createInstanceHeaderDiv(instanceName, overallHealth, healthTitle);
        instanceSection.appendChild(headerDiv);
        
        const detailsContainer = document.createElement('div');
        detailsContainer.id = `${ID_PREFIXES.PBS_DETAILS}${instanceId}`;
        detailsContainer.className = CSS_CLASSES.PBS_INSTANCE_DETAILS;
        detailsContainer.classList.toggle(CSS_CLASSES.HIDDEN, !showDetails);

        if (overallHealth === 'error' && statusText) {
            const errorPara = document.createElement('p');
            errorPara.className = `${CSS_CLASSES.TEXT_RED_500_DARK_RED_400} ${CSS_CLASSES.TEXT_SM} ${CSS_CLASSES.MB2}`;
            errorPara.textContent = statusText;
            detailsContainer.appendChild(errorPara);
        } else if (pbsInstanceData && pbsInstanceData.status === 'ok') {
            const dsSection = _createDatastoreSectionElement(instanceId);
            _populateDsTableBody(dsSection.querySelector('tbody'), pbsInstanceData.datastores || [], statusText, showDetails);
            detailsContainer.appendChild(dsSection);

            const taskHealthTableSection = _createPbsTaskHealthTable(instanceId, pbsInstanceData);
            detailsContainer.appendChild(taskHealthTableSection);
            
            const allTaskSectionsContainer = _createAllTaskSectionsContainer(instanceId);
             _populateInstanceTaskSections(allTaskSectionsContainer, instanceId, pbsInstanceData, statusText, showDetails);
            detailsContainer.appendChild(allTaskSectionsContainer);

        } else if (pbsInstanceData && (pbsInstanceData.status === 'pending_initialization' || !pbsInstanceData.status)) {
             const pendingMsg = document.createElement('p');
             pendingMsg.textContent = `Data for ${instanceName} is still loading or initializing...`;
             pendingMsg.className = `${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.TEXT_SM}`;
             detailsContainer.appendChild(pendingMsg);
        } else {
            const noDataMsg = document.createElement('p');
            noDataMsg.textContent = `No detailed data available for ${instanceName}.`;
            noDataMsg.className = `${CSS_CLASSES.TEXT_GRAY_500_DARK_GRAY_400} ${CSS_CLASSES.TEXT_SM}`;
            detailsContainer.appendChild(noDataMsg);
        }
        
        instanceSection.appendChild(detailsContainer);
        targetPanelElement.appendChild(instanceSection);
        
        return instanceSection;
    };

    function initPbsEventListeners() {
        // Event listeners for "Show More" buttons are now handled within populatePbsTaskTable
        // to ensure they are attached correctly when tables are dynamically populated.
        // This function is kept if other PBS-specific global event listeners are needed in the future.
        // const pbsInstancesContainer = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
        // if (!pbsInstancesContainer) {
        //     console.warn(`PBS instances container #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found. Some UI interactions might not work.`);
        // }
    }

    return {
        updatePbsInfo,
        initPbsEventListeners
    };
})();
