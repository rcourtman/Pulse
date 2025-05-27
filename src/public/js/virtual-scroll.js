// Virtual scrolling implementation for large datasets
PulseApp.virtualScroll = (() => {
    const ITEM_HEIGHT = 40; // Height of each row in pixels
    const BUFFER_SIZE = 5; // Number of items to render outside viewport
    const SCROLL_DEBOUNCE = 10; // Debounce scroll events
    
    class VirtualScroller {
        constructor(container, items, rowRenderer) {
            this.container = container;
            this.items = items;
            this.rowRenderer = rowRenderer;
            this.itemHeight = ITEM_HEIGHT;
            this.bufferSize = BUFFER_SIZE;
            
            this.scrollTop = 0;
            this.containerHeight = 0;
            this.visibleStart = 0;
            this.visibleEnd = 0;
            
            this.scrollHandler = null;
            this.resizeObserver = null;
            
            this.init();
        }
        
        init() {
            // Create scroll container structure
            this.viewport = document.createElement('div');
            this.viewport.className = 'virtual-scroll-viewport';
            this.viewport.style.height = '100%';
            this.viewport.style.overflowY = 'auto';
            this.viewport.style.position = 'relative';
            
            this.spacer = document.createElement('div');
            this.spacer.className = 'virtual-scroll-spacer';
            this.spacer.style.height = `${this.items.length * this.itemHeight}px`;
            
            this.content = document.createElement('div');
            this.content.className = 'virtual-scroll-content';
            this.content.style.position = 'absolute';
            this.content.style.top = '0';
            this.content.style.left = '0';
            this.content.style.right = '0';
            
            this.viewport.appendChild(this.spacer);
            this.viewport.appendChild(this.content);
            
            // Clear container and add viewport
            this.container.innerHTML = '';
            this.container.appendChild(this.viewport);
            
            // Set up event listeners
            this.scrollHandler = this.debounce(() => this.handleScroll(), SCROLL_DEBOUNCE);
            this.viewport.addEventListener('scroll', this.scrollHandler);
            
            // Set up resize observer
            this.resizeObserver = new ResizeObserver(() => this.handleResize());
            this.resizeObserver.observe(this.viewport);
            
            // Initial render
            this.handleResize();
        }
        
        handleScroll() {
            this.scrollTop = this.viewport.scrollTop;
            this.updateVisibleRange();
            this.render();
        }
        
        handleResize() {
            this.containerHeight = this.viewport.clientHeight;
            this.updateVisibleRange();
            this.render();
        }
        
        updateVisibleRange() {
            const scrollTop = this.scrollTop;
            const visibleItems = Math.ceil(this.containerHeight / this.itemHeight);
            
            this.visibleStart = Math.max(0, Math.floor(scrollTop / this.itemHeight) - this.bufferSize);
            this.visibleEnd = Math.min(
                this.items.length,
                Math.ceil((scrollTop + this.containerHeight) / this.itemHeight) + this.bufferSize
            );
        }
        
        render() {
            const fragment = document.createDocumentFragment();
            const visibleItems = this.items.slice(this.visibleStart, this.visibleEnd);
            
            // Create rows for visible items
            visibleItems.forEach((item, index) => {
                const actualIndex = this.visibleStart + index;
                const row = this.rowRenderer(item, actualIndex);
                
                if (row) {
                    // Position the row
                    row.style.position = 'absolute';
                    row.style.top = `${actualIndex * this.itemHeight}px`;
                    row.style.left = '0';
                    row.style.right = '0';
                    row.style.height = `${this.itemHeight}px`;
                    fragment.appendChild(row);
                }
            });
            
            // Clear content and add new rows
            this.content.innerHTML = '';
            this.content.appendChild(fragment);
        }
        
        updateItems(newItems) {
            this.items = newItems;
            this.spacer.style.height = `${this.items.length * this.itemHeight}px`;
            this.updateVisibleRange();
            this.render();
        }
        
        scrollToItem(index) {
            const scrollTop = index * this.itemHeight;
            this.viewport.scrollTop = scrollTop;
        }
        
        destroy() {
            if (this.scrollHandler) {
                this.viewport.removeEventListener('scroll', this.scrollHandler);
            }
            if (this.resizeObserver) {
                this.resizeObserver.disconnect();
            }
            this.container.innerHTML = '';
        }
        
        debounce(func, wait) {
            let timeout;
            return function executedFunction(...args) {
                const later = () => {
                    clearTimeout(timeout);
                    func(...args);
                };
                clearTimeout(timeout);
                timeout = setTimeout(later, wait);
            };
        }
    }
    
    function createVirtualScroller(container, items, rowRenderer) {
        return new VirtualScroller(container, items, rowRenderer);
    }
    
    return {
        createVirtualScroller
    };
})();