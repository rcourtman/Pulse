PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.storage = (() => {

    function getStorageTypeIcon(type) {
        switch(type) {
            case 'dir':
                return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-yellow-600 dark:text-yellow-400"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>';
            case 'lvm':
            case 'lvmthin':
                return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-purple-600 dark:text-purple-400"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>';
            case 'zfs':
            case 'zfspool':
                 return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-red-600 dark:text-red-400"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>';
            case 'nfs':
            case 'cifs':
                 return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-blue-600 dark:text-blue-400"><path d="M16 17l5-5-5-5"></path><path d="M8 17l-5-5 5-5"></path></svg>';
            case 'cephfs':
            case 'rbd':
                 return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-indigo-600 dark:text-indigo-400"><path d="M18 8h1a4 4 0 0 1 0 8h-1"></path><path d="M2 8h16v9a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4V8z"></path><line x1="6" y1="1" x2="6" y2="4"></line><line x1="10" y1="1" x2="10" y2="4"></line><line x1="14" y1="1" x2="14" y2="4"></line></svg>';
            default:
                return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block mr-1 align-middle text-gray-500"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>';
        }
    }

    function getContentBadgeDetails(contentType) {
        let details = {
            badgeClass: 'bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300',
            tooltip: `Content type: ${contentType}`
        };

        switch(contentType) {
            case 'iso':
                details.badgeClass = 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300';
                details.tooltip = 'ISO images (e.g., for OS installation)';
                break;
            case 'vztmpl':
                details.badgeClass = 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300';
                details.tooltip = 'Container templates';
                break;
            case 'backup':
                details.badgeClass = 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300';
                details.tooltip = 'VM/Container backup files (vzdump)';
                break;
            case 'images':
                details.badgeClass = 'bg-teal-100 dark:bg-teal-900/50 text-teal-700 dark:text-teal-300';
                details.tooltip = 'VM disk images (qcow2, raw, etc.)';
                break;
            case 'rootdir':
                 details.badgeClass = 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300';
                 details.tooltip = 'Storage for container root filesystems';
                 break;
             case 'snippets':
                 details.badgeClass = 'bg-pink-100 dark:bg-pink-900/50 text-pink-700 dark:text-pink-300';
                 details.tooltip = 'Snippet files (e.g., cloud-init configs)';
                 break;
        }
        return details;
    }

    function sortNodeStorageData(storageArray) {
        if (!storageArray || !Array.isArray(storageArray)) return [];
        const sortedArray = [...storageArray];
        sortedArray.sort((a, b) => {
            const nameA = String(a.storage || '').toLowerCase();
            const nameB = String(b.storage || '').toLowerCase();
            return nameA.localeCompare(nameB);
        });
        return sortedArray;
    }

    function updateStorageInfo() {
        const contentDiv = document.getElementById('storage-info-content');
        if (!contentDiv) return;
        contentDiv.innerHTML = '';
        contentDiv.className = '';

        const nodes = PulseApp.state.get('nodesData') || [];

        if (!Array.isArray(nodes) || nodes.length === 0) {
            contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No node or storage data available.</p>';
            return;
        }

        const storageByNode = nodes.reduce((acc, node) => {
            if (node && node.node) {
                acc[node.node] = Array.isArray(node.storage) ? node.storage : [];
            }
            return acc;
        }, {});

        const nodeKeys = Object.keys(storageByNode);

        if (nodeKeys.length === 0) {
          contentDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400 p-4 text-center">No storage data found associated with nodes.</p>';
          return;
        }

        const table = document.createElement('table');
        table.className = 'w-full text-sm border-collapse table-auto min-w-full';

        const thead = document.createElement('thead');
        thead.innerHTML = `
            <tr class="border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 sticky top-0 z-10 text-xs font-medium tracking-wider text-left text-gray-600 uppercase dark:text-gray-300">
              <th class="p-1 px-2">Storage</th>
              <th class="p-1 px-2">Content</th>
              <th class="p-1 px-2">Type</th>
              <th class="p-1 px-2">Shared</th>
              <th class="p-1 px-2">Usage</th>
              <th class="p-1 px-2">Avail</th>
              <th class="p-1 px-2">Total</th>
            </tr>
          `;
        table.appendChild(thead);

        const tbody = document.createElement('tbody');
        tbody.className = 'divide-y divide-gray-200 dark:divide-gray-600';

        const sortedNodeNames = Object.keys(storageByNode).sort((a, b) => a.localeCompare(b));

        sortedNodeNames.forEach(nodeName => {
          const nodeStorageData = storageByNode[nodeName];

          const nodeHeaderRow = document.createElement('tr');
          nodeHeaderRow.className = 'bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs node-storage-header';
          nodeHeaderRow.innerHTML = PulseApp.ui.common.generateNodeGroupHeaderCellHTML(`Node: ${nodeName}`, 7, 'td');
          tbody.appendChild(nodeHeaderRow);

          if (nodeStorageData.length === 0) {
            const noDataRow = document.createElement('tr');
            noDataRow.innerHTML = `<td colspan="7" class="p-2 px-3 text-sm text-gray-500 dark:text-gray-400 italic">No storage configured or found for this node.</td>`;
            tbody.appendChild(noDataRow);
            return;
          }

          const sortedNodeStorageData = sortNodeStorageData(nodeStorageData);
          sortedNodeStorageData.forEach(store => {
              const row = _createStorageRow(store);
              tbody.appendChild(row);
          });
        });

        table.appendChild(thead);
        table.appendChild(tbody);
        contentDiv.appendChild(table);
    }

    function _createStorageRow(store) {
        const row = document.createElement('tr');
        const isDisabled = store.enabled === 0 || store.active === 0;
        row.className = `transition-all duration-150 ease-out hover:bg-gray-100 dark:hover:bg-gray-700/60 hover:shadow-md hover:-translate-y-px ${isDisabled ? 'opacity-50 grayscale-[50%]' : ''}`;

        const usagePercent = store.total > 0 ? (store.used / store.total) * 100 : 0;
        const usageTooltipText = `${PulseApp.utils.formatBytes(store.used)} / ${PulseApp.utils.formatBytes(store.total)} (${usagePercent.toFixed(1)}%)`;
        const usageColorClass = PulseApp.utils.getUsageColor(usagePercent);
        const usageBarHTML = PulseApp.utils.createProgressTextBarHTML(usagePercent, usageTooltipText, usageColorClass);

        const sharedIconTooltip = store.shared === 1 ? 'Shared across cluster' : 'Local to node';
        const sharedIcon = store.shared === 1 ? `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block text-green-600 dark:text-green-400"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>`
            : `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="inline-block text-gray-400 dark:text-gray-500 opacity-50"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>`;

        const contentTypes = (store.content || '').split(',').map(ct => ct.trim()).filter(ct => ct);
        contentTypes.sort();
        const contentBadges = contentTypes.map(ct => {
            const details = getContentBadgeDetails(ct);
            return `<span data-tooltip="${details.tooltip}" class="storage-tooltip-trigger inline-block ${details.badgeClass} rounded px-1.5 py-0.5 text-xs font-medium mr-1 cursor-default">${ct}</span>`;
        }).join('');

        row.innerHTML = `
            <td class="p-1 px-2 whitespace-nowrap text-gray-900 dark:text-gray-100 font-medium">${store.storage || 'N/A'}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs flex items-center">${contentBadges || '-'}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${store.type || 'N/A'}</td>
            <td class="p-1 px-2 whitespace-nowrap storage-tooltip-trigger cursor-default" data-tooltip="${sharedIconTooltip}">${sharedIcon}</td>
            <td class="p-1 px-2 text-gray-600 dark:text-gray-300 min-w-[250px]">${usageBarHTML}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${PulseApp.utils.formatBytes(store.avail)}</td>
            <td class="p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300">${PulseApp.utils.formatBytes(store.total)}</td>
        `;
        return row;
    }

    return {
        updateStorageInfo
    };
})();
