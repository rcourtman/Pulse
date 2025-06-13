PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.toast = (() => {
    let toastContainer = null;
    const MAX_TOASTS = 5;

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

    function showToast(message, type = 'info', duration = 5000) {
        if (!toastContainer) {
            createToastContainer();
        }

        const toastId = `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
        const toast = document.createElement('div');
        toast.id = toastId;
        toast.className = 'pointer-events-auto transform transition-all duration-300 ease-out opacity-0 translate-x-full scale-95';

        const typeClasses = getTypeClasses(type);
        const icon = getTypeIcon(type);

        toast.innerHTML = `
            <div class="${typeClasses.bg} ${typeClasses.border} ${typeClasses.text} shadow-lg rounded-lg border overflow-hidden backdrop-blur-sm">
                <div class="p-4">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            ${icon}
                        </div>
                        <div class="ml-3 flex-1">
                            <p class="text-sm font-medium">${message}</p>
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
            </div>
        `;

        toastContainer.appendChild(toast);

        // Animate in
        requestAnimationFrame(() => {
            toast.classList.remove('opacity-0', 'translate-x-full', 'scale-95');
            toast.classList.add('opacity-100', 'translate-x-0', 'scale-100');
        });

        // Auto remove after duration
        if (duration > 0) {
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
                    text: 'text-green-800 dark:text-green-200'
                };
            case 'error':
                return {
                    bg: 'bg-red-50 dark:bg-red-900/20',
                    border: 'border-red-200 dark:border-red-800',
                    text: 'text-red-800 dark:text-red-200'
                };
            case 'warning':
                return {
                    bg: 'bg-yellow-50 dark:bg-yellow-900/20',
                    border: 'border-yellow-200 dark:border-yellow-800',
                    text: 'text-yellow-800 dark:text-yellow-200'
                };
            case 'info':
            default:
                return {
                    bg: 'bg-blue-50 dark:bg-blue-900/20',
                    border: 'border-blue-200 dark:border-blue-800',
                    text: 'text-blue-800 dark:text-blue-200'
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

    // Utility functions that replace browser dialogs
    function alert(message) {
        return showToast(message, 'info', 6000);
    }

    function success(message) {
        return showToast(message, 'success', 4000);
    }

    function error(message) {
        return showToast(message, 'error', 7000);
    }

    function warning(message) {
        return showToast(message, 'warning', 5000);
    }

    function info(message) {
        return showToast(message, 'info', 4000);
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
        info,
        confirm
    };
})();

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', PulseApp.ui.toast.init);
} else {
    PulseApp.ui.toast.init();
}