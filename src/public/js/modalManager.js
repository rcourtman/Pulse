/**
 * Modal Manager - Centralized modal handling utilities
 * Reduces code duplication across modal implementations
 */
(function() {
    'use strict';

    const modalManager = {
        activeModals: new Set(),

        /**
         * Create a modal with standard behavior (click outside to close, escape key)
         * @param {Object} options - Modal configuration
         * @returns {HTMLElement} The created modal element
         */
        createModal: function(options = {}) {
            const {
                id = 'modal-' + Date.now(),
                content = '',
                className = '',
                closeOnClickOutside = true,
                closeOnEscape = true,
                onClose = null
            } = options;

            const modal = document.createElement('div');
            modal.id = id;
            modal.className = `fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center ${className}`;
            modal.innerHTML = content;

            if (closeOnClickOutside) {
                modal.addEventListener('click', (e) => {
                    if (e.target === modal) {
                        this.closeModal(modal);
                    }
                });
            }

            if (closeOnEscape) {
                this.activeModals.add({
                    element: modal,
                    onClose: onClose
                });
            }

            return modal;
        },

        /**
         * Open a modal
         * @param {HTMLElement|string} modal - Modal element or selector
         */
        openModal: function(modal) {
            if (typeof modal === 'string') {
                modal = document.querySelector(modal);
            }
            
            if (!modal) return;

            modal.classList.remove('hidden');
            document.body.style.overflow = 'hidden';
            
            // Track that we set overflow hidden
            document.body.dataset.modalOpen = 'true';
            
            // Focus management
            const focusable = modal.querySelector('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
            if (focusable) {
                focusable.focus();
            }
        },

        /**
         * Close a modal
         * @param {HTMLElement|string} modal - Modal element or selector
         */
        closeModal: function(modal) {
            if (typeof modal === 'string') {
                modal = document.querySelector(modal);
            }
            
            if (!modal) return;

            // First hide the modal
            modal.classList.add('hidden');
            
            // Force a reflow to ensure the hidden class is applied
            modal.offsetHeight;
            
            // Then check if any other modals are still open
            // Use a more specific selector that excludes loading overlay and other non-modal elements
            const openModals = document.querySelectorAll('.fixed.inset-0.z-50:not(.hidden):not(#loading-overlay)');
            const actualModals = Array.from(openModals).filter(el => {
                // Filter out elements that aren't actually modals
                return el.id.includes('modal') || el.classList.toString().includes('modal');
            });
            
            // Only reset overflow if no other actual modals are open
            if (actualModals.length === 0) {
                document.body.style.overflow = '';
                document.body.style.overflowY = '';
                document.body.style.position = '';
                document.documentElement.style.overflow = '';
                delete document.body.dataset.modalOpen;
            }

            // Execute onClose callback if exists
            const modalData = Array.from(this.activeModals).find(m => m.element === modal);
            if (modalData && modalData.onClose) {
                modalData.onClose();
            }

            // Remove from active modals
            this.activeModals = new Set(Array.from(this.activeModals).filter(m => m.element !== modal));
        },

        /**
         * Close all open modals
         */
        closeAllModals: function() {
            const modals = document.querySelectorAll('.fixed.z-50:not(.hidden)');
            modals.forEach(modal => this.closeModal(modal));
        },

        /**
         * Setup standard modal behaviors for existing modal
         * @param {HTMLElement|string} modal - Modal element or selector
         * @param {Object} options - Configuration options
         */
        setupModal: function(modal, options = {}) {
            if (typeof modal === 'string') {
                modal = document.querySelector(modal);
            }
            
            if (!modal) return;

            const {
                closeButton = null,
                closeOnClickOutside = true,
                closeOnEscape = true,
                onClose = null
            } = options;

            if (closeButton) {
                const btn = typeof closeButton === 'string' ? modal.querySelector(closeButton) : closeButton;
                if (btn) {
                    btn.addEventListener('click', () => this.closeModal(modal));
                }
            }

            if (closeOnClickOutside) {
                modal.addEventListener('click', (e) => {
                    if (e.target === modal) {
                        this.closeModal(modal);
                    }
                });
            }

            if (closeOnEscape || onClose) {
                this.activeModals.add({
                    element: modal,
                    onClose: onClose
                });
            }
        },

        /**
         * Initialize global escape key handler
         */
        init: function() {
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') {
                    const openModals = Array.from(document.querySelectorAll('.fixed.z-50:not(.hidden)'));
                    if (openModals.length > 0) {
                        // Close the topmost modal
                        const topModal = openModals[openModals.length - 1];
                        const modalData = Array.from(this.activeModals).find(m => m.element === topModal);
                        
                        if (!modalData || modalData.element === topModal) {
                            this.closeModal(topModal);
                        }
                    }
                }
            });
            
        }
    };

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => modalManager.init());
    } else {
        modalManager.init();
    }

    // Export to global scope
    window.PulseApp = window.PulseApp || {};
    window.PulseApp.modalManager = modalManager;
})();