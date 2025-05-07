PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs = (() => {

    const getPbsStatusIcon = (status) => {
        if (status === 'OK') {
            return '<span class="text-green-500 dark:text-green-400" title="OK">✓</span>';
        } else if (status === 'running') {
            return '<span class="inline-block animate-spin rounded-full h-3 w-3 border-t-2 border-b-2 border-blue-500" title="Running"></span>';
        } else if (status) {
            return `<span class="text-red-500 dark:text-red-400 font-bold" title="${status}">✗</span>`;
        } else {
            return '<span class="text-gray-400" title="Unknown">?</span>';
        }
    };

    const getPbsGcStatusText = (gcStatus) => {
      if (!gcStatus || gcStatus === 'unknown' || gcStatus === 'N/A') {
        return '<span class="text-xs text-gray-400">-</span>';
      }
      let colorClass = 'text-gray-600 dark:text-gray-400';
      if (gcStatus.includes('error') || gcStatus.includes('failed')) {
          colorClass = 'text-red-500 dark:text-red-400';
      } else if (gcStatus === 'OK') {
          colorClass = 'text-green-500 dark:text-green-400';
      }
      return `<span class="text-xs ${colorClass}">${gcStatus}</span>`;
    };

    function updatePbsTaskSummaryCard(prefix, summaryData) {
      const okEl = document.getElementById(`pbs-${prefix}-ok`);
      const failedEl = document.getElementById(`pbs-${prefix}-failed`);
      const totalEl = document.getElementById(`pbs-${prefix}-total`);
      const lastOkEl = document.getElementById(`pbs-${prefix}-last-ok`);
      const lastFailedEl = document.getElementById(`pbs-${prefix}-last-failed`);

      if (!okEl || !failedEl || !totalEl || !lastOkEl || !lastFailedEl) {
        console.warn(`UI elements for PBS task summary '${prefix}' not found.`);
        return;
      }

      if (summaryData && summaryData.summary) {
        const summary = summaryData.summary;
        okEl.textContent = summary.ok ?? '-';
        failedEl.textContent = summary.failed ?? '-';
        totalEl.textContent = summary.total ?? '-';
        lastOkEl.textContent = PulseApp.utils.formatPbsTimestamp(summary.lastOk);
        lastFailedEl.textContent = PulseApp.utils.formatPbsTimestamp(summary.lastFailed);

        failedEl.classList.toggle('font-bold', (summary.failed ?? 0) > 0);

      } else {
        okEl.textContent = '-';
        failedEl.textContent = '-';
        totalEl.textContent = '-';
        lastOkEl.textContent = '-';
        lastFailedEl.textContent = '-';
        failedEl.classList.remove('font-bold');
      }
    }

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

    function populatePbsTaskTable(parentSectionElement, fullTasksArray) {
      if (!parentSectionElement) {
          console.warn('[PBS UI] Parent element not found for task table');
          return;
      }
      const tableBody = parentSectionElement.querySelector('tbody');
      const showMoreButton = parentSectionElement.querySelector('.pbs-show-more');
      const noTasksMessage = parentSectionElement.querySelector('.pbs-no-tasks');
      const initialLimit = PulseApp.config.INITIAL_PBS_TASK_LIMIT;

      if (!tableBody) {
          console.warn('[PBS UI] Table body not found within', parentSectionElement);
          return;
      }

      tableBody.innerHTML = '';

      const tasks = fullTasksArray || [];
      let displayedTasks = tasks.slice(0, initialLimit);

      if (tasks.length === 0) {
          if (noTasksMessage) noTasksMessage.classList.remove('hidden');
          if (showMoreButton) showMoreButton.classList.add('hidden');
      } else {
          if (noTasksMessage) noTasksMessage.classList.add('hidden');

          displayedTasks.forEach(task => {
              const target = parsePbsTaskTarget(task);
              const statusIcon = getPbsStatusIcon(task.status);
              const startTime = task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A';
              const duration = task.duration !== null ? PulseApp.utils.formatDuration(task.duration) : 'N/A';
              const upid = task.upid || 'N/A';
              const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

              const row = document.createElement('tr');
              row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-150 ease-in-out';
              row.innerHTML = `
                  <td class="p-1 px-2 text-sm text-gray-700 dark:text-gray-300">${target}</td>
                  <td class="p-1 px-2 text-sm">${statusIcon}</td>
                  <td class="p-1 px-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${startTime}</td>
                  <td class="p-1 px-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${duration}</td>
                  <td class="p-1 px-2 text-xs font-mono text-gray-400 dark:text-gray-500 truncate" title="${upid}">${shortUpid}</td>
              `;
              tableBody.appendChild(row);
          });

          if (showMoreButton) {
              if (tasks.length > initialLimit) {
                  showMoreButton.classList.remove('hidden');
                  const remainingCount = tasks.length - initialLimit;
                  showMoreButton.textContent = `Show More (${remainingCount} older)`;
                  if (!showMoreButton.dataset.handlerAttached) {
                      showMoreButton.addEventListener('click', () => {
                          tasks.slice(initialLimit).forEach(task => {
                             const target = parsePbsTaskTarget(task);
                             const statusIcon = getPbsStatusIcon(task.status);
                             const startTime = task.startTime ? PulseApp.utils.formatPbsTimestamp(task.startTime) : 'N/A';
                             const duration = task.duration !== null ? PulseApp.utils.formatDuration(task.duration) : 'N/A';
                             const upid = task.upid || 'N/A';
                             const shortUpid = upid.length > 30 ? `${upid.substring(0, 15)}...${upid.substring(upid.length - 15)}` : upid;

                             const row = document.createElement('tr');
                             row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-150 ease-in-out';
                             row.innerHTML = `
                                 <td class="p-1 px-2 text-sm text-gray-700 dark:text-gray-300">${target}</td>
                                 <td class="p-1 px-2 text-sm">${statusIcon}</td>
                                 <td class="p-1 px-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${startTime}</td>
                                 <td class="p-1 px-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">${duration}</td>
                                 <td class="p-1 px-2 text-xs font-mono text-gray-400 dark:text-gray-500 truncate" title="${upid}">${shortUpid}</td>
                             `;
                              tableBody.appendChild(row);
                          });
                          showMoreButton.classList.add('hidden');
                      });
                       showMoreButton.dataset.handlerAttached = 'true';
                  }
              } else {
                  showMoreButton.classList.add('hidden');
              }
          }
      }
  }

    function updatePbsInfo(pbsArray) {
      const container = document.getElementById('pbs-instances-container');
      if (!container) {
          console.error("PBS container element #pbs-instances-container not found!");
          return;
      }

      if (!pbsArray || pbsArray.length === 0) {
          container.innerHTML = '';
          const placeholder = document.createElement('p');
          placeholder.className = 'text-gray-500 dark:text-gray-400 p-4 text-center text-sm';
          placeholder.textContent = 'Proxmox Backup Server integration is not configured.';
          container.appendChild(placeholder);
          return;
      }

      const loadingMessage = document.getElementById('pbs-loading-message');
      if (loadingMessage) {
          loadingMessage.remove();
      }

      const notConfiguredBanner = container.querySelector('.pbs-not-configured-banner');
      if (notConfiguredBanner) {
          notConfiguredBanner.remove();
      }

      const currentInstanceIds = new Set();

      const createSummaryCard = (type, title, summaryData) => {
          const card = document.createElement('div');
          const summary = summaryData?.summary || {};
          const ok = summary.ok ?? '-';
          const failed = summary.failed ?? '-';
          const total = summary.total ?? '-';
          const lastOk = PulseApp.utils.formatPbsTimestamp(summary.lastOk);
          const lastFailed = PulseApp.utils.formatPbsTimestamp(summary.lastFailed);
          const failedStyle = (failed > 0) ? 'font-bold text-red-600 dark:text-red-400' : 'text-red-600 dark:text-red-400 font-semibold';

          const highlightClass = (failed > 0) ? 'border-l-4 border-red-500 dark:border-red-400' : 'border-l-4 border-transparent';
          card.className = `border border-gray-200 dark:border-gray-700 rounded p-3 bg-gray-100/50 dark:bg-gray-700/50 ${highlightClass}`;

          card.innerHTML = `
              <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">${title} (7d)</h4>
              <div class="space-y-1 text-sm">
                  <div><span class="font-medium text-gray-800 dark:text-gray-200">OK:</span> <span class="ml-1 text-green-600 dark:text-green-400 font-semibold">${ok}</span></div>
                  <div><span class="font-medium text-gray-800 dark:text-gray-200">Failed:</span> <span class="ml-1 ${failedStyle}">${failed}</span></div>
                  <div><span class="font-medium text-gray-800 dark:text-gray-200">Total:</span> <span class="ml-1 text-gray-700 dark:text-gray-300 font-semibold">${total}</span></div>
                  <div><span class="font-medium text-gray-800 dark:text-gray-200">Last OK:</span> <span class="ml-1 text-gray-600 dark:text-gray-400 text-xs">${lastOk}</span></div>
                  <div><span class="font-medium text-gray-800 dark:text-gray-200">Last Fail:</span> <span class="ml-1 text-gray-600 dark:text-gray-400 text-xs">${lastFailed}</span></div>
              </div>`;
          return card;
      };

       const createTaskTableHTML = (tableId, title, idColumnHeader) => {
          const tbodyId = tableId.replace('-table-', '-tbody-');
          const toggleButtonContainerId = tableId.replace('-table', '-toggle-container');
          return `
          <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">Recent ${title} Tasks</h4>
          <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded">
              <table id="${tableId}" class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                  <thead class="sticky top-0 z-10 bg-gray-100 dark:bg-gray-800">
                      <tr class="text-xs font-medium tracking-wider text-left text-gray-600 uppercase dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                          <th scope="col" class="p-1 px-2">${idColumnHeader}</th>
                          <th scope="col" class="p-1 px-2">Status</th>
                          <th scope="col" class="p-1 px-2">Start Time</th>
                          <th scope="col" class="p-1 px-2">Duration</th>
                          <th scope="col" class="p-1 px-2">UPID</th>
                      </tr>
                  </thead>
                  <tbody id="${tbodyId}" class="pbs-task-tbody divide-y divide-gray-200 dark:divide-gray-700">
                       <!-- Populated by JS -->
                  </tbody>
              </table>
          </div>
           <div id="${toggleButtonContainerId}" class="pbs-toggle-button-container pt-1 text-right">
                <button class="pbs-show-more text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300 hidden">Show More</button>
                <p class="pbs-no-tasks text-xs text-gray-400 dark:text-gray-500 hidden italic">No recent tasks found.</p>
           </div>
          `;
      };

      pbsArray.forEach((pbsInstance, index) => {
        const rawInstanceId = pbsInstance.pbsEndpointId || `instance-${index}`;
        const instanceId = PulseApp.utils.sanitizeForId(rawInstanceId);
        const instanceName = pbsInstance.pbsInstanceName || `PBS Instance ${index + 1}`;
        const instanceElementId = `pbs-instance-${instanceId}`;
        currentInstanceIds.add(instanceElementId);

        let instanceWrapper = document.getElementById(instanceElementId);
        let detailsContainer, dsTableBody, instanceTitleElement;

        let statusText = 'Loading...';
        let showDetails = false;
        let statusColorClass = 'text-gray-600 dark:text-gray-400';
        switch (pbsInstance.status) {
            case 'configured':
                statusText = `Configured, attempting connection...`;
                statusColorClass = 'text-gray-600 dark:text-gray-400';
                break;
            case 'ok':
                statusText = `Status: OK`;
                statusColorClass = 'text-green-600 dark:text-green-400';
                showDetails = true;
                break;
            case 'error':
                statusText = `Error: ${pbsInstance.errorMessage || 'Connection failed'}`;
                statusColorClass = 'text-red-600 dark:text-red-400';
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

        const createHealthBadgeHTML = (health, title) => {
            let colorClass = 'bg-gray-400 dark:bg-gray-500';
            if (health === 'ok') colorClass = 'bg-green-500';
            else if (health === 'warning') colorClass = 'bg-yellow-500';
            else if (health === 'error') colorClass = 'bg-red-500';
            return `<span title="${title}" class="inline-block w-3 h-3 ${colorClass} rounded-full mr-2 flex-shrink-0"></span>`;
        };

        if (instanceWrapper) {
            detailsContainer = instanceWrapper.querySelector(`#pbs-details-${instanceId}`);
            instanceTitleElement = instanceWrapper.querySelector('h3');

            if (instanceTitleElement) {
                instanceTitleElement.innerHTML = `${createHealthBadgeHTML(overallHealth, healthTitle)}${instanceName}`;
            }

            if (detailsContainer) {
                 dsTableBody = detailsContainer.querySelector(`#pbs-ds-tbody-${instanceId}`);
                 if (dsTableBody) {
                     dsTableBody.innerHTML = '';
                     if (showDetails && pbsInstance.datastores) {
                         if (pbsInstance.datastores.length === 0) {
                             dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">No PBS datastores found or accessible.</td></tr>`;
                         } else {
                             pbsInstance.datastores.forEach(ds => {
                                const totalBytes = ds.total || 0;
                                const usedBytes = ds.used || 0;
                                const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0);
                                const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
                                const usageColor = PulseApp.utils.getUsageColor(usagePercent);
                                const usageText = totalBytes > 0 ? `${usagePercent}% (${PulseApp.utils.formatBytes(usedBytes)} of ${PulseApp.utils.formatBytes(totalBytes)})` : 'N/A';
                                const gcStatusHtml = getPbsGcStatusText(ds.gcStatus);
                                const row = document.createElement('tr');
                                row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50';
                                row.innerHTML = `<td class="p-1 px-2 whitespace-nowrap">${ds.name || 'N/A'}</td> <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${ds.path || 'N/A'}</td> <td class="p-1 px-2 whitespace-nowrap">${PulseApp.utils.formatBytes(usedBytes)}</td> <td class="p-1 px-2 whitespace-nowrap">${PulseApp.utils.formatBytes(availableBytes)}</td> <td class="p-1 px-2 whitespace-nowrap">${totalBytes > 0 ? PulseApp.utils.formatBytes(totalBytes) : 'N/A'}</td> <td class="p-1 px-2 min-w-[150px]">${totalBytes > 0 ? PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor) : '-'}</td> <td class="p-1 px-2 whitespace-nowrap">${gcStatusHtml}</td>`;
                                dsTableBody.appendChild(row);
                             });
                         }
                    } else {
                         dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    }
                 }

                 const summariesSection = detailsContainer.querySelector(`#pbs-summaries-section-${instanceId}`);
                 if (summariesSection) {
                   summariesSection.innerHTML = '';
                   summariesSection.appendChild(createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
                   summariesSection.appendChild(createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
                   summariesSection.appendChild(createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
                   summariesSection.appendChild(createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
                 }

                if (showDetails) {
                    const backupSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="backup"]');
                    if (backupSection) populatePbsTaskTable(backupSection, pbsInstance.backupTasks?.recentTasks);

                    const verifySection = detailsContainer.querySelector('.pbs-task-section[data-task-type="verify"]');
                    if (verifySection) populatePbsTaskTable(verifySection, pbsInstance.verificationTasks?.recentTasks);

                    const syncSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="sync"]');
                    if (syncSection) populatePbsTaskTable(syncSection, pbsInstance.syncTasks?.recentTasks);

                    const pruneGcSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="prunegc"]');
                    if (pruneGcSection) populatePbsTaskTable(pruneGcSection, pbsInstance.pruneTasks?.recentTasks);

                } else {
                    const backupTbody = document.getElementById(`pbs-recent-backup-tasks-tbody-${instanceId}`);
                    if (backupTbody) backupTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    const verifyTbody = document.getElementById(`pbs-recent-verify-tasks-tbody-${instanceId}`);
                    if (verifyTbody) verifyTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    const syncTbody = document.getElementById(`pbs-recent-sync-tasks-tbody-${instanceId}`);
                    if (syncTbody) syncTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                    const pruneTbody = document.getElementById(`pbs-recent-prunegc-tasks-tbody-${instanceId}`);
                    if (pruneTbody) pruneTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                }

                detailsContainer.classList.toggle('hidden', !showDetails);
            }

        } else {
            instanceWrapper = document.createElement('div');
            instanceWrapper.className = 'pbs-instance-section border border-gray-200 dark:border-gray-700 rounded p-4 mb-4 bg-gray-50/30 dark:bg-gray-800/30';
            instanceWrapper.id = instanceElementId;

            const headerDiv = document.createElement('div');
            headerDiv.className = 'flex justify-between items-center mb-3';
            instanceTitleElement = document.createElement('h3');
            instanceTitleElement.className = 'text-lg font-semibold text-gray-800 dark:text-gray-200 flex items-center';
            instanceTitleElement.innerHTML = `${createHealthBadgeHTML(overallHealth, healthTitle)}${instanceName}`;
            headerDiv.appendChild(instanceTitleElement);
            instanceWrapper.appendChild(headerDiv);

            detailsContainer = document.createElement('div');
            detailsContainer.className = `pbs-instance-details space-y-4 ${showDetails ? '' : 'hidden'}`;
            detailsContainer.id = `pbs-details-${instanceId}`;

            const dsSection = document.createElement('div');
            dsSection.id = `pbs-ds-section-${instanceId}`;
            dsSection.innerHTML = `
                <h4 class="text-md font-semibold mb-2 text-gray-700 dark:text-gray-300">Datastores</h4>
                <div class="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded">
                    <table id="pbs-ds-table-${instanceId}" class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                         <thead class="sticky top-0 z-10 bg-gray-100 dark:bg-gray-800">
                             <tr class="text-xs font-medium tracking-wider text-left text-gray-600 uppercase dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                                 <th scope="col" class="p-1 px-2">Name</th>
                                 <th scope="col" class="p-1 px-2">Path</th>
                                 <th scope="col" class="p-1 px-2">Used</th>
                                 <th scope="col" class="p-1 px-2">Available</th>
                                 <th scope="col" class="p-1 px-2">Total</th>
                                 <th scope="col" class="p-1 px-2">Usage</th>
                                 <th scope="col" class="p-1 px-2">GC Status</th>
                             </tr>
                         </thead>
                        <tbody id="pbs-ds-tbody-${instanceId}" class="divide-y divide-gray-200 dark:divide-gray-700"></tbody>
                    </table>
                </div>`;
            detailsContainer.appendChild(dsSection);

            const summariesSection = document.createElement('div');
            summariesSection.id = `pbs-summaries-section-${instanceId}`;
            summariesSection.className = 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4';
            summariesSection.appendChild(createSummaryCard('backup', 'Backups', pbsInstance.backupTasks));
            summariesSection.appendChild(createSummaryCard('verify', 'Verification', pbsInstance.verificationTasks));
            summariesSection.appendChild(createSummaryCard('sync', 'Sync', pbsInstance.syncTasks));
            summariesSection.appendChild(createSummaryCard('prune', 'Prune/GC', pbsInstance.pruneTasks));
            detailsContainer.appendChild(summariesSection);

             const recentBackupTasksSection = document.createElement('div');
             recentBackupTasksSection.className = 'pbs-task-section';
             recentBackupTasksSection.dataset.taskType = 'backup';
             recentBackupTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-backup-tasks-table-${instanceId}`, 'Backup', 'Guest');
             detailsContainer.appendChild(recentBackupTasksSection);

             const recentVerifyTasksSection = document.createElement('div');
             recentVerifyTasksSection.className = 'pbs-task-section';
             recentVerifyTasksSection.dataset.taskType = 'verify';
             recentVerifyTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-verify-tasks-table-${instanceId}`, 'Verification', 'Guest/Group');
             detailsContainer.appendChild(recentVerifyTasksSection);

             const recentSyncTasksSection = document.createElement('div');
             recentSyncTasksSection.className = 'pbs-task-section';
             recentSyncTasksSection.dataset.taskType = 'sync';
             recentSyncTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-sync-tasks-table-${instanceId}`, 'Sync', 'Job ID');
             detailsContainer.appendChild(recentSyncTasksSection);

             const recentPruneGcTasksSection = document.createElement('div');
             recentPruneGcTasksSection.className = 'pbs-task-section';
             recentPruneGcTasksSection.dataset.taskType = 'prunegc';
             recentPruneGcTasksSection.innerHTML = createTaskTableHTML(`pbs-recent-prunegc-tasks-table-${instanceId}`, 'Prune/GC', 'Datastore/Group');
             detailsContainer.appendChild(recentPruneGcTasksSection);

            instanceWrapper.appendChild(detailsContainer);
            container.appendChild(instanceWrapper);

            dsTableBody = instanceWrapper.querySelector(`#pbs-ds-tbody-${instanceId}`);
             if (dsTableBody) {
                 if (showDetails && pbsInstance.datastores) {
                      if (pbsInstance.datastores.length === 0) { dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">No PBS datastores found or accessible.</td></tr>`; }
                      else {
                          pbsInstance.datastores.forEach(ds => {
                            const totalBytes = ds.total || 0;
                            const usedBytes = ds.used || 0;
                            const availableBytes = (ds.available !== null && ds.available !== undefined) ? ds.available : (totalBytes > 0 ? totalBytes - usedBytes : 0);
                            const usagePercent = totalBytes > 0 ? Math.round((usedBytes / totalBytes) * 100) : 0;
                            const usageColor = PulseApp.utils.getUsageColor(usagePercent);
                            const usageText = totalBytes > 0 ? `${usagePercent}% (${PulseApp.utils.formatBytes(usedBytes)} of ${PulseApp.utils.formatBytes(totalBytes)})` : 'N/A';
                            const gcStatusHtml = getPbsGcStatusText(ds.gcStatus);
                            const row = document.createElement('tr');
                            row.className = 'hover:bg-gray-50 dark:hover:bg-gray-700/50';
                            row.innerHTML = `<td class="p-1 px-2 whitespace-nowrap">${ds.name || 'N/A'}</td> <td class="p-1 px-2 whitespace-nowrap text-gray-500 dark:text-gray-400">${ds.path || 'N/A'}</td> <td class="p-1 px-2 whitespace-nowrap">${PulseApp.utils.formatBytes(usedBytes)}</td> <td class="p-1 px-2 whitespace-nowrap">${PulseApp.utils.formatBytes(availableBytes)}</td> <td class="p-1 px-2 whitespace-nowrap">${totalBytes > 0 ? PulseApp.utils.formatBytes(totalBytes) : 'N/A'}</td> <td class="p-1 px-2 min-w-[150px]">${totalBytes > 0 ? PulseApp.utils.createProgressTextBarHTML(usagePercent, usageText, usageColor) : '-'}</td> <td class="p-1 px-2 whitespace-nowrap">${gcStatusHtml}</td>`;
                            dsTableBody.appendChild(row);
                          });
                      }
                 } else { dsTableBody.innerHTML = `<tr><td colspan="7" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`; }
             }

             if (showDetails) {
                const backupSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="backup"]');
                if (backupSection) populatePbsTaskTable(backupSection, pbsInstance.backupTasks?.recentTasks);

                const verifySection = detailsContainer.querySelector('.pbs-task-section[data-task-type="verify"]');
                if (verifySection) populatePbsTaskTable(verifySection, pbsInstance.verificationTasks?.recentTasks);

                const syncSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="sync"]');
                if (syncSection) populatePbsTaskTable(syncSection, pbsInstance.syncTasks?.recentTasks);

                const pruneGcSection = detailsContainer.querySelector('.pbs-task-section[data-task-type="prunegc"]');
                if (pruneGcSection) populatePbsTaskTable(pruneGcSection, pbsInstance.pruneTasks?.recentTasks);

            } else {
                const backupTbody = document.getElementById(`pbs-recent-backup-tasks-tbody-${instanceId}`);
                if (backupTbody) backupTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                const verifyTbody = document.getElementById(`pbs-recent-verify-tasks-tbody-${instanceId}`);
                if (verifyTbody) verifyTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                const syncTbody = document.getElementById(`pbs-recent-sync-tasks-tbody-${instanceId}`);
                if (syncTbody) syncTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
                const pruneTbody = document.getElementById(`pbs-recent-prunegc-tasks-tbody-${instanceId}`);
                if (pruneTbody) pruneTbody.innerHTML = `<tr><td colspan="5" class="px-4 py-4 text-sm text-gray-400 text-center">${statusText}</td></tr>`;
            }

        }

    });

    container.querySelectorAll('.pbs-instance-section').forEach(el => {
        if (!currentInstanceIds.has(el.id)) {
            el.remove();
        }
    });

  }

    function initPbsEventListeners() {
        const pbsInstancesContainer = document.getElementById('pbs-instances-container');
        if (pbsInstancesContainer) {
            pbsInstancesContainer.addEventListener('click', (event) => {
                const button = event.target.closest('.pbs-show-more'); // Target only show more buttons
                if (!button) return;

                const parentSection = button.closest('.pbs-task-section');
                if (!parentSection) {
                    console.error("Parent task section (.pbs-task-section) not found for show more button");
                    return;
                }
                // We don't need the full tasks from dataset anymore, assume populate handles it
                // const fullTasks = JSON.parse(parentSection.dataset.fullTasks || '[]');
                // populatePbsTaskTable(parentSection, fullTasks);
                // The click handler inside populatePbsTaskTable should handle showing more.
            });
        } else {
            console.warn("PBS instances container not found, toggle functionality will not work.");
        }
    }

    return {
        updatePbsInfo,
        initPbsEventListeners
    };
})(); 