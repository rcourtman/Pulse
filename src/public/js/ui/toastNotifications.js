PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.toast = (() => {
    let toastContainer = null;
    const MAX_TOASTS = 5;
    let soundEnabled = true;
    const toastQueue = [];
    let isProcessingQueue = false;

    function init() {
        createToastContainer();
    }

    function createToastContainer() {
        if (toastContainer) return;
        
        toastContainer = document.createElement('div');
        toastContainer.id = 'pulse-toast-container';
        toastContainer.className = 'fixed bottom-4 left-4 z-50 space-y-2 pointer-events-none';
        toastContainer.style.maxWidth = '400px';
        toastContainer.style.zIndex = '9999'; // Ensure very high z-index
        document.body.appendChild(toastContainer);
    }

    function showToast(message, type = 'info', duration = 5000, options = {}) {
        if (!toastContainer) {
            createToastContainer();
        }

        // Add to queue if we're showing too many
        if (toastContainer.children.length >= MAX_TOASTS) {
            toastQueue.push({ message, type, duration, options });
            processQueue();
            return;
        }

        const toastId = `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
        const toast = document.createElement('div');
        toast.id = toastId;
        toast.className = 'pointer-events-auto transform transition-all duration-300 ease-out opacity-0 translate-x-full scale-95';

        const typeClasses = getTypeClasses(type);
        const icon = getTypeIcon(type);
        
        // Play sound for alerts
        if (type === 'alert' && soundEnabled && options.playSound !== false) {
            playAlertSound();
        }

        const actionButtons = options.actions ? `
            <div class="mt-3 flex gap-2">
                ${options.actions.map(action => `
                    <button onclick="PulseApp.ui.toast.handleAction('${toastId}', ${JSON.stringify(action).replace(/"/g, '&quot;')})" 
                            class="px-3 py-1 text-xs font-medium rounded-md ${action.style || 'bg-gray-600 hover:bg-gray-700 text-white'} transition-colors">
                        ${action.label}
                    </button>
                `).join('')}
            </div>
        ` : '';
        
        const progressBar = type === 'alert' ? `
            <div class="absolute bottom-0 left-0 right-0 h-1 bg-gray-200 dark:bg-gray-700">
                <div id="${toastId}-progress" class="h-full ${typeClasses.progressBg} transition-all duration-linear" style="width: 100%"></div>
            </div>
        ` : '';

        toast.innerHTML = `
            <div class="${typeClasses.bg} ${typeClasses.border} ${typeClasses.text} shadow-lg rounded-lg border overflow-hidden backdrop-blur-sm relative">
                <div class="p-4">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            ${icon}
                        </div>
                        <div class="ml-3 flex-1">
                            <p class="text-sm font-medium">${message}</p>
                            ${options.details ? `<p class="text-xs mt-1 opacity-75">${options.details}</p>` : ''}
                            ${actionButtons}
                        </div>
                        <div class="ml-4 flex-shrink-0">
                            <button onclick="PulseApp.ui.toast.dismissToast('${toastId}')" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors">
                                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                </svg>
                            </button>
                        </div>
                    </div>
                </div>
                ${progressBar}
            </div>
        `;

        toastContainer.appendChild(toast);

        // Animate in
        requestAnimationFrame(() => {
            toast.classList.remove('opacity-0', 'translate-x-full', 'scale-95');
            toast.classList.add('opacity-100', 'translate-x-0', 'scale-100');
        });

        // Auto remove after duration with progress bar
        if (duration > 0) {
            if (type === 'alert') {
                // Animate progress bar
                const progressBar = document.getElementById(`${toastId}-progress`);
                if (progressBar) {
                    progressBar.style.transition = `width ${duration}ms linear`;
                    requestAnimationFrame(() => {
                        progressBar.style.width = '0%';
                    });
                }
            }
            
            setTimeout(() => {
                dismissToast(toastId);
            }, duration);
        }

        // Limit number of toasts
        while (toastContainer.children.length > MAX_TOASTS) {
            const firstToast = toastContainer.firstChild;
            if (firstToast) {
                dismissToast(firstToast.id);
            }
        }

        return toastId;
    }

    function dismissToast(toastId) {
        const toast = document.getElementById(toastId);
        if (!toast) return;

        toast.classList.remove('opacity-100', 'translate-x-0', 'scale-100');
        toast.classList.add('opacity-0', 'translate-x-full', 'scale-95');
        
        setTimeout(() => {
            if (toast.parentNode) {
                toast.remove();
            }
        }, 300);
    }

    function getTypeClasses(type) {
        switch (type) {
            case 'success':
                return {
                    bg: 'bg-green-50 dark:bg-green-900/20',
                    border: 'border-green-200 dark:border-green-800',
                    text: 'text-green-800 dark:text-green-200',
                    progressBg: 'bg-green-500'
                };
            case 'error':
                return {
                    bg: 'bg-red-50 dark:bg-red-900/20',
                    border: 'border-red-200 dark:border-red-800',
                    text: 'text-red-800 dark:text-red-200',
                    progressBg: 'bg-red-500'
                };
            case 'warning':
                return {
                    bg: 'bg-yellow-50 dark:bg-yellow-900/20',
                    border: 'border-yellow-200 dark:border-yellow-800',
                    text: 'text-yellow-800 dark:text-yellow-200',
                    progressBg: 'bg-yellow-500'
                };
            case 'alert':
                return {
                    bg: 'bg-purple-50 dark:bg-purple-900/20',
                    border: 'border-purple-200 dark:border-purple-800',
                    text: 'text-purple-800 dark:text-purple-200',
                    progressBg: 'bg-purple-500'
                };
            case 'info':
            default:
                return {
                    bg: 'bg-blue-50 dark:bg-blue-900/20',
                    border: 'border-blue-200 dark:border-blue-800',
                    text: 'text-blue-800 dark:text-blue-200',
                    progressBg: 'bg-blue-500'
                };
        }
    }

    function getTypeIcon(type) {
        switch (type) {
            case 'success':
                return `<svg class="h-5 w-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>`;
            case 'error':
                return `<svg class="h-5 w-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>`;
            case 'warning':
                return `<svg class="h-5 w-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
                </svg>`;
            case 'alert':
                return `<svg class="h-5 w-5 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"></path>
                </svg>`;
            case 'info':
            default:
                return `<svg class="h-5 w-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>`;
        }
    }

    // Enhanced confirmation dialog replacement
    function showConfirmToast(message, onConfirm, onCancel = null) {
        if (!toastContainer) {
            createToastContainer();
        }

        const toastId = `confirm-toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
        const toast = document.createElement('div');
        toast.id = toastId;
        toast.className = 'pointer-events-auto transform transition-all duration-300 ease-out opacity-0 translate-x-full scale-95';

        toast.innerHTML = `
            <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 text-yellow-800 dark:text-yellow-200 shadow-lg rounded-lg overflow-hidden backdrop-blur-sm">
                <div class="p-4">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                        </div>
                        <div class="ml-3 flex-1">
                            <p class="text-sm font-medium mb-3">${message}</p>
                            <div class="flex gap-2">
                                <button onclick="PulseApp.ui.toast.handleConfirm('${toastId}', true)" 
                                        class="px-3 py-1 bg-yellow-600 hover:bg-yellow-700 text-white text-xs font-medium rounded transition-colors">
                                    Confirm
                                </button>
                                <button onclick="PulseApp.ui.toast.handleConfirm('${toastId}', false)" 
                                        class="px-3 py-1 bg-gray-600 hover:bg-gray-700 text-white text-xs font-medium rounded transition-colors">
                                    Cancel
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;

        // Store callbacks for this toast
        toast._onConfirm = onConfirm;
        toast._onCancel = onCancel;

        toastContainer.appendChild(toast);

        // Animate in
        requestAnimationFrame(() => {
            toast.classList.remove('opacity-0', 'translate-x-full', 'scale-95');
            toast.classList.add('opacity-100', 'translate-x-0', 'scale-100');
        });

        return toastId;
    }

    function handleConfirm(toastId, confirmed) {
        const toast = document.getElementById(toastId);
        if (!toast) return;

        if (confirmed && toast._onConfirm) {
            toast._onConfirm();
        } else if (!confirmed && toast._onCancel) {
            toast._onCancel();
        }

        dismissToast(toastId);
    }

    // Process queued toasts
    function processQueue() {
        if (isProcessingQueue || toastQueue.length === 0) return;
        
        isProcessingQueue = true;
        setTimeout(() => {
            if (toastContainer.children.length < MAX_TOASTS && toastQueue.length > 0) {
                const { message, type, duration, options } = toastQueue.shift();
                showToast(message, type, duration, options);
            }
            isProcessingQueue = false;
            if (toastQueue.length > 0) {
                processQueue();
            }
        }, 300);
    }
    
    // Handle action button clicks
    function handleAction(toastId, action) {
        if (action.callback) {
            // Execute the callback function by name
            const fn = new Function('return ' + action.callback)();
            if (typeof fn === 'function') {
                fn();
            }
        }
        if (action.dismiss !== false) {
            dismissToast(toastId);
        }
    }
    
    // Play alert sound
    function playAlertSound() {
        try {
            const audio = new Audio('data:audio/wav;base64,UklGRnoGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQoGAACBhYqFbF1fdJivrJBhNjVgodDbq2EcBj+a2/LDciUFLIHO8tiJNwgZaLvt559NEAxQp+PwtmMcBjiR1/LMeSwFJHfH8N2QQAoUXrTp66hVFApGn+DyvmwhBSuBzvLZiTUIG2m98OScTgwOUarm7blmFgU7k9n1unEiBC13yO/eizEIHWq+8+OWT' +
                          'AsOUqzn77dmFwUvgs/z2JE7Bhxquuztm0wNDVKp5e+6UwgHb8DwzW4eCC2Iz/DaiToFG2q/8OScTQ0PVrPt8NqMOwgbab3w5KFNDAxPqOPwtGMcBjiS1/HMeCwGI3fH8d+PQAoUXrTp66hVFApGnt/yvmwhBSuBzvLZiTYIG2m98OWdTQ0NUqjl77lmFgU7k9j1unEjBC14yO/eizEIHWq+8+OU' +
                          'TgsOUqvm77dmFgUvgM/y2ZE7Bhxquuztm0wNDVOq5e+6UwgHb8Dw1GweBCyIzv7ZijYHG2q/8OScTQ0PVrPt8NqLOwgbab3w5KFNDAxPqOPwtGMcBjiS1/HMeCwGI3fH8d+PQAoUXrTp66hVFApGnt/yv2wiBSuBzvLaiTcIHGm+8uSeTQ0NUqnm7blmFQU7k9j1unEiBC14yO/eizAIG2m98uOUTwsOUqzm7rZl' +
                          'FwUvgs/y2ZE6Bxtpue3tm00NDVOp5e+7UwgIbsDu1GweBCuIzv7ZiTYHG2q/8OScTQ0PVrPt8NqLOwgbab3w5KFNDAxPqOPwtGMcBjiS1/HMeCwGI3fH8d+PQAoUXrTp66hVFApGnt/yv2wiBSuBzvLaiTcIHGm+8uSeTQ0NUqnm7blmFQU7k9j1unEiBC14yO/eizAIG2m98uOUTwsOUqzm7rZlFwUvgs/y2ZE6' +
                          'Bxtpue3tm00NDVOp5e+7UwgIZsLs1G4eBC2Izf/ZiTUHG2q+8OScTA0OVbLs8NiLOggaaL3v5KJNDAxPqOLws2MdBjiS1/HMeCwFI3fH8d+PQAoUXrTp66hVFApGnt/yv2wiBSuBzvLaiTcIHGm+8uSeTQ0NUqnm7blmFQU7k9j1unEiBC14yO/eizAIG2m98uOUTwsOUqzm7rZlFwUvgs/y2ZE6Bxtpue3tm00NDVOp' +
                          '5e+7UwgIZsLs1G4eBC2Izf/ZiTUHG2q+8OScTA0OVbLs8NiLOggaaL3v5KJNDAxPqOLws2MdBjiS1/HMeCwFI3fH8d+PQAoUXrTp66hVFApGnt/yv2wiBSuBzvLaiTcIHGm+8uSeTQ0NUqnm7blmFQU7k9j1unEiBC14yO/eizAIG2m98uOUTwsOUqzm7rZlFwU=');
            audio.volume = 0.3;
            audio.play().catch(() => {});
        } catch (e) {
            // Ignore audio errors
        }
    }
    
    // Toggle sound
    function toggleSound(enabled) {
        soundEnabled = enabled;
        localStorage.setItem('pulse-alert-sound', enabled ? 'true' : 'false');
    }

    // Utility functions that replace browser dialogs
    function alert(message, options) {
        return showToast(message, 'alert', 8000, options);
    }

    function success(message, duration, options) {
        return showToast(message, 'success', duration || 4000, options);
    }

    function error(message, duration, options) {
        return showToast(message, 'error', duration || 7000, options);
    }

    function warning(message, duration, options) {
        return showToast(message, 'warning', duration || 5000, options);
    }
    
    function info(message, duration, options) {
        return showToast(message, 'info', duration || 5000, options);
    }

    function confirm(message, onConfirm, onCancel = null) {
        return showConfirmToast(message, onConfirm, onCancel);
    }

    return {
        init,
        showToast,
        dismissToast,
        handleConfirm,
        alert,
        success,
        error,
        warning,
        confirm
    };
})();

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.toast.init);
} else {
    PulseApp.ui.toast.init();
}