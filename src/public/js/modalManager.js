// Modal Manager - Handles modal dialogs throughout the application
PulseApp.modalManager = (() => {
    const modalRegistry = new Map();

    function setupModal(modalElement, options = {}) {
        // Handle both ID strings and DOM elements
        const modal = typeof modalElement === 'string' 
            ? document.querySelector(modalElement)
            : modalElement;
            
        if (!modal) {
            // Modal not found - this is OK, not all modals exist on all pages
            return;
        }

        // Store modal configuration
        modalRegistry.set(modal, options);

        // Setup close button if provided
        if (options.closeButton) {
            const closeBtn = typeof options.closeButton === 'string'
                ? modal.querySelector(options.closeButton)
                : options.closeButton;
                
            if (closeBtn) {
                closeBtn.addEventListener('click', () => closeModal(modal));
            }
        }

        // Setup click outside to close
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                closeModal(modal);
            }
        });

        // Setup escape key to close
        const escapeHandler = (e) => {
            if (e.key === 'Escape' && !modal.classList.contains('hidden')) {
                closeModal(modal);
            }
        };
        document.addEventListener('keydown', escapeHandler);
        
        // Store the handler so we can remove it later
        options._escapeHandler = escapeHandler;
    }

    function openModal(modalElement) {
        const modal = typeof modalElement === 'string' 
            ? document.querySelector(modalElement)
            : modalElement;
            
        if (modal) {
            modal.classList.remove('hidden');
            document.body.style.overflow = 'hidden'; // Prevent background scrolling
        }
    }

    function closeModal(modalElement) {
        const modal = typeof modalElement === 'string' 
            ? document.querySelector(modalElement)
            : modalElement;
            
        if (modal) {
            modal.classList.add('hidden');
            document.body.style.overflow = ''; // Restore scrolling
            
            // Call onClose callback if provided
            const options = modalRegistry.get(modal);
            if (options && options.onClose) {
                options.onClose();
            }
        }
    }

    return {
        setupModal,
        openModal,
        closeModal
    };
})();