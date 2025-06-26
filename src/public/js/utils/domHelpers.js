/**
 * DOM manipulation helper functions to reduce code duplication
 */

/**
 * Create an element with className and optional attributes
 * @param {string} tag - HTML tag name
 * @param {string} className - CSS class(es) to apply
 * @param {Object} attrs - Optional attributes to set
 * @returns {HTMLElement}
 */
function createElement(tag, className = '', attrs = {}) {
    const element = document.createElement(tag);
    if (className) {
        element.className = className;
    }
    Object.entries(attrs).forEach(([key, value]) => {
        if (key === 'text') {
            element.textContent = value;
        } else if (key === 'html') {
            element.innerHTML = value;
        } else {
            element.setAttribute(key, value);
        }
    });
    return element;
}

/**
 * Create a button element with common setup
 * @param {string} text - Button text
 * @param {string} className - CSS classes
 * @param {Function} onClick - Click handler
 * @returns {HTMLElement}
 */
function createButton(text, className, onClick) {
    const button = createElement('button', className, { text });
    if (onClick) {
        button.addEventListener('click', onClick);
    }
    return button;
}

/**
 * Create an icon element
 * @param {string} iconClass - Icon class (e.g., 'fa-times')
 * @param {string} additionalClass - Additional CSS classes
 * @returns {HTMLElement}
 */
function createIcon(iconClass, additionalClass = '') {
    return createElement('i', `fas ${iconClass} ${additionalClass}`.trim());
}

/**
 * Create a table row with cells
 * @param {Array} cells - Array of cell contents (strings or elements)
 * @param {string} className - Optional row class
 * @returns {HTMLElement}
 */
function createTableRow(cells, className = '') {
    const row = createElement('tr', className);
    cells.forEach(cell => {
        const td = createElement('td', 'pb-2 px-4');
        if (typeof cell === 'string') {
            td.textContent = cell;
        } else if (cell instanceof HTMLElement) {
            td.appendChild(cell);
        } else if (cell && typeof cell === 'object') {
            // Handle objects with html property
            td.innerHTML = cell.html || '';
            if (cell.className) {
                td.className = cell.className;
            }
        }
        row.appendChild(td);
    });
    return row;
}

/**
 * Toggle element visibility
 * @param {HTMLElement|string} element - Element or selector
 * @param {boolean} show - Show/hide flag
 */
function toggleVisibility(element, show) {
    const el = typeof element === 'string' ? document.querySelector(element) : element;
    if (el) {
        el.style.display = show ? '' : 'none';
    }
}

/**
 * Add loading state to element
 * @param {HTMLElement} element - Target element
 * @param {boolean} loading - Loading state
 */
function setLoadingState(element, loading) {
    if (loading) {
        element.classList.add('opacity-50', 'pointer-events-none');
        element.setAttribute('data-loading', 'true');
    } else {
        element.classList.remove('opacity-50', 'pointer-events-none');
        element.removeAttribute('data-loading');
    }
}

// Export for use in other files
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        createElement,
        createButton,
        createIcon,
        createTableRow,
        toggleVisibility,
        setLoadingState
    };
}