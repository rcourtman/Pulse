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


    function updatePbsInfo(pbsArray) {
      const container = document.getElementById(ID_PREFIXES.PBS_INSTANCES_CONTAINER);
      if (!container) {
          console.error(`PBS container element #${ID_PREFIXES.PBS_INSTANCES_CONTAINER} not found!`);
          return;
      }

      if (!pbsArray || pbsArray.length === 0) {
          container.innerHTML = '';
          const placeholder = document.createElement('p');
          placeholder.className = CSS_CLASSES.TEXT_GRAY_500_DARK_TEXT_GRAY_400_P4_TEXT_CENTER_TEXT_SM;
          placeholder.textContent = 'Proxmox Backup Server integration is not configured.';
          container.appendChild(placeholder);
          return;
      }

      const loadingMessage = document.getElementById('pbs-loading-message'); // Assuming this ID is unique and used elsewhere
      if (loadingMessage) {
          loadingMessage.remove();
      }

      const notConfiguredBanner = container.querySelector('.pbs-not-configured-banner'); // Assuming this class is unique
      if (notConfiguredBanner) {
          notConfiguredBanner.remove();
      }

      const currentInstanceIds = new Set();

      const _createSummaryCard = (type, title, summaryData) => {
          const card = document.createElement('div');
          const summary = summaryData?.summary || {};
          const ok = summary.ok ?? '-';
          const failed = summary.failed ?? '-';
          const total = summary.total ?? '-';
          const lastOk = PulseApp.utils.formatPbsTimestamp(summary.lastOk);
          const lastFailed = PulseApp.utils.formatPbsTimestamp(summary.lastFailed);
          const failedStyle = (failed > 0) ? `${CSS_CLASSES.FONT_BOLD} ${CSS_CLASSES.TEXT_RED_600_DARK_RED_400}` : `${CSS_CLASSES.TEXT_RED_600_DARK_RED_400} ${CSS_CLASSES.FONT_SEMIBOLD}`;
          const highlightClass = (failed > 0) ? CSS_CLASSES.BORDER_L_4_RED_500_DARK_RED_400 : CSS_CLASSES.BORDER_L_4_TRANSPARENT;

          card.className = `${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} ${CSS_CLASSES.P3} ${CSS_CLASSES.BG_GRAY_100_50_DARK_BG_GRAY_700_50} ${highlightClass}`;

          const heading = document.createElement('h4');
          heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
          heading.textContent = `${title} (7d)`;
          card.appendChild(heading);

          const contentDiv = document.createElement('div');
          contentDiv.className = `${CSS_CLASSES.SPACE_Y_1} ${CSS_CLASSES.TEXT_SM}`;

          const createStatDiv = (label, value, valueClass = '') => {
              const div = document.createElement('div');
              const labelSpan = document.createElement('span');
              labelSpan.className = `${CSS_CLASSES.FONT_MEDIUM} ${CSS_CLASSES.TEXT_GRAY_800_DARK_GRAY_200}`;
              labelSpan.textContent = `${label}:`;
              div.appendChild(labelSpan);

              const valueSpan = document.createElement('span');
              valueSpan.className = `${CSS_CLASSES.ML1} ${valueClass}`;
              valueSpan.textContent = value;
              div.appendChild(valueSpan);
              return div;
          };

          contentDiv.appendChild(createStatDiv('OK', ok, `${CSS_CLASSES.TEXT_GREEN_600_DARK_GREEN_400} ${CSS_CLASSES.FONT_SEMIBOLD}`));
          contentDiv.appendChild(createStatDiv('Failed', failed, failedStyle));
          contentDiv.appendChild(createStatDiv('Total', total, `${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300} ${CSS_CLASSES.FONT_SEMIBOLD}`));
          contentDiv.appendChild(createStatDiv('Last OK', lastOk, `${CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400} ${CSS_CLASSES.TEXT_XS}`));
          contentDiv.appendChild(createStatDiv('Last Fail', lastFailed, `${CSS_CLASSES.TEXT_GRAY_600_DARK_GRAY_400} ${CSS_CLASSES.TEXT_XS}`));

          card.appendChild(contentDiv);
          return card;
      };

      const _createTaskTableElement = (tableId, title, idColumnHeader) => {
          const fragment = document.createDocumentFragment();

          const heading = document.createElement('h4');
          heading.className = `${CSS_CLASSES.TEXT_MD} ${CSS_CLASSES.FONT_SEMIBOLD} ${CSS_CLASSES.MB2} ${CSS_CLASSES.TEXT_GRAY_700_DARK_GRAY_300}`;
          heading.textContent = `Recent ${title} Tasks`;
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
          toggleButtonContainer.className = `pbs-toggle-button-container ${CSS_CLASSES.PT1} ${CSS_CLASSES.TEXT_RIGHT}`;

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

      pbsArray.forEach((pbsInstance, index) => {
        const rawInstanceId = pbsInstance.pbsEndpointId || `instance-${index}`;
        const instanceId = PulseApp.utils.sanitizeForId(rawInstanceId);
        const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
        const instanceElementId = ID_PREFIXES.PBS_INSTANCE + instanceId;
        currentInstanceIds.add(instanceElementId);

        let instanceWrapper = document.getElementById(instanceElementId);
        let detailsContainer, dsTableBody, instanceTitleElement;

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
                    overallHealth = 'error';
                    healthTitle = 'Error: One or more recent tasks failed';
                }
            }
        }

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
            instanceTitleElement.appendChild(_createHealthBadgeHTML(overallHealth, healthTitle));
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

        const _createSummariesSectionElement = (instanceId, pbsInstanceData) => {
            const summariesSection = document.createElement('div');
            summariesSection.id = ID_PREFIXES.PBS_SUMMARIES_SECTION + instanceId;
            summariesSection.className = `${CSS_CLASSES.GRID} ${CSS_CLASSES.GRID_COLS_1} ${CSS_CLASSES.MD_GRID_COLS_2} ${CSS_CLASSES.LG_GRID_COLS_4} ${CSS_CLASSES.GAP_4}`;
            summariesSection.appendChild(_createSummaryCard('backup', 'Backups', pbsInstanceData.backupTasks));
            summariesSection.appendChild(_createSummaryCard('verify', 'Verification', pbsInstanceData.verificationTasks));
            summariesSection.appendChild(_createSummaryCard('sync', 'Sync', pbsInstanceData.syncTasks));
            summariesSection.appendChild(_createSummaryCard('prune', 'Prune/GC', pbsInstanceData.pruneTasks));
            return summariesSection;
        };

        const _createAllTaskSectionsContainer = (instanceId) => {
            const container = document.createElement('div'); // Or DocumentFragment
            container.className = CSS_CLASSES.SPACE_Y_4; // Add some spacing between task sections

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

        const _createPbsInstanceElement = (pbsInstanceData, instanceId, instanceName, overallHealth, healthTitle, showDetails, statusText) => {
            const instanceWrapper = document.createElement('div');
            instanceWrapper.className = `${CSS_CLASSES.PBS_INSTANCE_SECTION} ${CSS_CLASSES.BORDER_GRAY_200_DARK_BORDER_GRAY_700} ${CSS_CLASSES.ROUNDED} p-4 mb-4 bg-gray-50/30 dark:bg-gray-800/30`;
            instanceWrapper.id = ID_PREFIXES.PBS_INSTANCE + instanceId;

            instanceWrapper.appendChild(_createInstanceHeaderDiv(instanceName, overallHealth, healthTitle));

            const detailsContainer = document.createElement('div');
            detailsContainer.className = `${CSS_CLASSES.PBS_INSTANCE_DETAILS} ${CSS_CLASSES.SPACE_Y_4} ${showDetails ? '' : CSS_CLASSES.HIDDEN}`;
            detailsContainer.id = ID_PREFIXES.PBS_DETAILS + instanceId;

            detailsContainer.appendChild(_createDatastoreSectionElement(instanceId));
            detailsContainer.appendChild(_createSummariesSectionElement(instanceId, pbsInstanceData));
            detailsContainer.appendChild(_createAllTaskSectionsContainer(instanceId));
            
            instanceWrapper.appendChild(detailsContainer);

            // Populate dynamic content after structure is built
            const dsTableBodyElement = instanceWrapper.querySelector(`#${ID_PREFIXES.PBS_DS_TBODY}${instanceId}`);
            _populateDsTableBody(dsTableBodyElement, pbsInstanceData.datastores, statusText, showDetails);
            _populateInstanceTaskSections(detailsContainer, instanceId, pbsInstanceData, statusText, showDetails);

            return instanceWrapper;
        };

        if (instanceWrapper) {
            detailsContainer = instanceWrapper.querySelector(`#${ID_PREFIXES.PBS_DETAILS}${instanceId}`);
            instanceTitleElement = instanceWrapper.querySelector('h3');
            if (instanceTitleElement) {
                instanceTitleElement.innerHTML = ''; // Clear existing content
                instanceTitleElement.appendChild(_createHealthBadgeHTML(overallHealth, healthTitle));
                instanceTitleElement.appendChild(document.createTextNode(instanceName));
            }

            if (detailsContainer) {
                 dsTableBody = detailsContainer.querySelector(`#${ID_PREFIXES.PBS_DS_TBODY}${instanceId}`);
                 _populateDsTableBody(dsTableBody, pbsInstance.datastores, statusText, showDetails);

                 const summariesSection = detailsContainer.querySelector(`#${ID_PREFIXES.PBS_SUMMARIES_SECTION}${instanceId}`);
                 if (summariesSection) {
                   summariesSection.innerHTML = '';
                   summariesSection.appendChild(_createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
                   summariesSection.appendChild(_createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
                   summariesSection.appendChild(_createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
                   summariesSection.appendChild(_createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
                 }

                _populateInstanceTaskSections(detailsContainer, instanceId, pbsInstance, statusText, showDetails);
                detailsContainer.classList.toggle(CSS_CLASSES.HIDDEN, !showDetails);
            }
        } else {
            instanceWrapper = _createPbsInstanceElement(pbsInstance, instanceId, instanceName, overallHealth, healthTitle, showDetails, statusText);
            container.appendChild(instanceWrapper);
        }
    });

    container.querySelectorAll(`.${CSS_CLASSES.PBS_INSTANCE_SECTION}`).forEach(el => {
        if (!currentInstanceIds.has(el.id)) {
            el.remove();
        }
    });

  }

    function initPbsEventListeners() {
        // Event listeners for "Show More" buttons are now handled within populatePbsTaskTable
        // to ensure they are attached correctly when tables are dynamically populated.
        // This function is kept if other PBS-specific global event listeners are needed in the future.
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
