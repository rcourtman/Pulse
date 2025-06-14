// Status icons component
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.statusIcons = (() => {
    
    const icons = {
        vm: {
            svg: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="8" x="2" y="2" rx="2" ry="2"/><rect width="20" height="8" x="2" y="14" rx="2" ry="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/></svg>`,
            label: 'Virtual Machine'
        },
        lxc: {
            svg: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" x2="12" y1="22.08" y2="12"/></svg>`,
            label: 'Container'
        },
        running: {
            svg: `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none"><circle cx="12" cy="12" r="10"/></svg>`,
            label: 'Running'
        },
        stopped: {
            svg: `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/></svg>`,
            label: 'Stopped'
        },
        paused: {
            svg: `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none"><rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/></svg>`,
            label: 'Paused'
        },
        error: {
            svg: `<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>`,
            label: 'Error'
        }
    };

    function createTypeIcon(type) {
        const isVM = type === 'VM' || type === 'qemu';
        const icon = isVM ? icons.vm : icons.lxc;
        const colorClass = isVM 
            ? 'text-blue-600 dark:text-blue-400' 
            : 'text-green-600 dark:text-green-400';
        
        return `
            <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-xs font-medium ${isVM ? 'bg-blue-100 dark:bg-blue-900/30' : 'bg-green-100 dark:bg-green-900/30'}">
                <span class="${colorClass}">${icon.svg}</span>
                <span class="${colorClass}">${isVM ? 'VM' : 'LXC'}</span>
            </span>
        `;
    }

    function createStatusIcon(status) {
        let icon, colorClass, bgClass;
        
        switch(status) {
            case 'running':
                icon = icons.running;
                colorClass = 'text-green-500';
                bgClass = '';
                break;
            case 'stopped':
                icon = icons.stopped;
                colorClass = 'text-gray-600 dark:text-gray-400';
                bgClass = '';
                break;
            case 'paused':
                icon = icons.paused;
                colorClass = 'text-yellow-500';
                bgClass = '';
                break;
            default:
                icon = icons.error;
                colorClass = 'text-red-500';
                bgClass = '';
        }
        
        return `<span class="inline-flex items-center ${colorClass}" title="${icon.label}">${icon.svg}</span>`;
    }

    function createStatusBadge(status) {
        let icon, textColor, bgColor, text;
        
        switch(status) {
            case 'running':
                icon = icons.running;
                textColor = 'text-green-700 dark:text-green-300';
                bgColor = 'bg-green-100 dark:bg-green-900/30';
                text = 'Running';
                break;
            case 'stopped':
                icon = icons.stopped;
                textColor = 'text-gray-700 dark:text-gray-300';
                bgColor = 'bg-gray-100 dark:bg-gray-800/30';
                text = 'Stopped';
                break;
            case 'paused':
                icon = icons.paused;
                textColor = 'text-yellow-700 dark:text-yellow-300';
                bgColor = 'bg-yellow-100 dark:bg-yellow-900/30';
                text = 'Paused';
                break;
            default:
                icon = icons.error;
                textColor = 'text-red-700 dark:text-red-300';
                bgColor = 'bg-red-100 dark:bg-red-900/30';
                text = 'Error';
        }
        
        return `
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${bgColor} ${textColor}">
                ${icon.svg}
                <span>${text}</span>
            </span>
        `;
    }

    return {
        createTypeIcon,
        createStatusIcon,
        createStatusBadge
    };
})();