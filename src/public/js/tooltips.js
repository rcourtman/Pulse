PulseApp.tooltips = (() => {
    let tooltipElement = null;
    let sliderValueTooltip = null;

    function init() {
        tooltipElement = document.getElementById('custom-tooltip');
        sliderValueTooltip = document.getElementById('slider-value-tooltip');

        if (!tooltipElement) {
            console.warn('Element #custom-tooltip not found - tooltips will not work.');
            return; // Don't attach listeners if the element is missing
        }
        if (!sliderValueTooltip) {
            console.warn('Element #slider-value-tooltip not found - slider values will not display on drag.');
            // Continue initialization for general tooltips even if slider tooltip is missing
        }

        tooltipElement.classList.remove('duration-100');
        tooltipElement.classList.add('duration-50');

        document.body.addEventListener('mouseover', handleMouseOver);
        document.body.addEventListener('mouseout', handleMouseOut);
        document.body.addEventListener('mousemove', handleMouseMove);

        // Hide tooltip when mouse leaves the document
        document.addEventListener('mouseleave', hideTooltip);
        
        // Hide tooltip on scroll to prevent stuck tooltips
        window.addEventListener('scroll', hideTooltip, true);

        document.addEventListener('mouseup', hideSliderTooltip);
        document.addEventListener('touchend', hideSliderTooltip);
    }

    function handleMouseOver(event) {
        // Don't handle if programmatic tooltip is active
        if (tooltipElement && tooltipElement.getAttribute('data-programmatic') === 'true') {
            return;
        }
        
        const target = event.target.closest('[data-tooltip], .metric-tooltip-trigger, .storage-tooltip-trigger, .truncate');
        if (target) {
            let tooltipText = target.getAttribute('data-tooltip');
            
            // Auto-generate tooltip for truncated text
            if (!tooltipText && target.classList.contains('truncate')) {
                const fullText = target.textContent.trim();
                const title = target.getAttribute('title');
                // Only show tooltip if text is actually truncated
                if ((title && title !== fullText) || target.scrollWidth > target.clientWidth) {
                    tooltipText = title || fullText;
                }
            }
            
            if (tooltipText && tooltipElement) {
                tooltipElement.textContent = tooltipText;
                positionTooltip(event);
                tooltipElement.classList.remove('hidden', 'opacity-0');
                tooltipElement.classList.add('opacity-100');
            }
        }
    }

    function handleMouseOut(event) {
        if (tooltipElement && tooltipElement.getAttribute('data-programmatic') === 'true') {
            return;
        }
        
        const target = event.target.closest('[data-tooltip], .metric-tooltip-trigger, .storage-tooltip-trigger, .truncate');
        if (!target) return;
        
        // Check if we're actually leaving the tooltip trigger element
        const relatedTarget = event.relatedTarget;
        if (relatedTarget && target.contains(relatedTarget)) {
            return; // We're still within the same tooltip trigger
        }
        
        if (tooltipElement) {
            tooltipElement.classList.add('hidden', 'opacity-0');
            tooltipElement.classList.remove('opacity-100');
        }
    }

    function handleMouseMove(event) {
        if (tooltipElement && !tooltipElement.classList.contains('hidden')) {
            const target = event.target.closest('[data-tooltip], .metric-tooltip-trigger, .storage-tooltip-trigger, .truncate');
            if (target) {
                positionTooltip(event);
            }
            // Don't hide on mousemove - let mouseout handle it properly
        }
    }

    function positionTooltip(event) {
        if (!tooltipElement) return;
        const offsetX = 10;
        const offsetY = 15;
        
        // Update only position, don't touch other styles
        tooltipElement.style.left = `${event.clientX + offsetX}px`;
        tooltipElement.style.top = `${event.clientY + offsetY}px`;
        tooltipElement.style.position = 'fixed';
        tooltipElement.style.zIndex = '9999';
    }

    function updateSliderTooltip(sliderElement) {
        if (!sliderValueTooltip || !sliderElement) return;

        // Check if this is a threshold slider or alert slider
        let displayText;
        let isThresholdSlider = false;
        let isAlertSlider = false;
        
        if (sliderElement.id.startsWith('threshold-slider-')) {
            const type = sliderElement.id.replace('threshold-slider-', '');
            if (PulseApp.state.getThresholdState()[type]) {
                isThresholdSlider = true;
                const numericValue = parseInt(sliderElement.value);
                displayText = `${numericValue}%`;
            }
        } else if (sliderElement.id.startsWith('alert-slider-') || sliderElement.id.startsWith('global-alert-') || sliderElement.id.startsWith('node-alert-slider-') || sliderElement.id.startsWith('global-node-alert-')) {
            isAlertSlider = true;
            const numericValue = parseInt(sliderElement.value);
            displayText = `${numericValue}%`;
        }
        
        // If neither threshold nor alert slider, don't show tooltip
        if (!isThresholdSlider && !isAlertSlider) return;

        const rect = sliderElement.getBoundingClientRect();
        const min = parseFloat(sliderElement.min);
        const max = parseFloat(sliderElement.max);
        const value = parseFloat(sliderElement.value);

        const percent = (max > min) ? (value - min) / (max - min) : 0;

        const thumbWidthEstimate = 16;
        let thumbX = rect.left + (percent * (rect.width - thumbWidthEstimate)) + (thumbWidthEstimate / 2);

        // Cancel any pending hide timeout when showing tooltip
        if (hideTooltipTimeout) {
            clearTimeout(hideTooltipTimeout);
            hideTooltipTimeout = null;
        }
        
        sliderValueTooltip.textContent = displayText;
        sliderValueTooltip.classList.remove('hidden');
        const tooltipRect = sliderValueTooltip.getBoundingClientRect();

        const posX = thumbX - (tooltipRect.width / 2);
        
        const isInTableHeader = sliderElement.closest('thead') !== null;
        const isGlobalAlertSlider = sliderElement.id && sliderElement.id.startsWith('global-alert-');
        let posY;
        
        
        if (isInTableHeader || isGlobalAlertSlider) {
            // For sliders in table header or global alert sliders, position tooltip above the slider but with more space
            posY = rect.top + window.pageYOffset - tooltipRect.height - 10;
        } else {
            // For regular sliders, position tooltip above the slider
            posY = rect.top + window.pageYOffset - tooltipRect.height - 5;
        }

        sliderValueTooltip.style.left = `${posX}px`;
        sliderValueTooltip.style.top = `${posY}px`;
    }

    let hideTooltipTimeout = null;

    function hideSliderTooltip() {
        // Clear any existing timeout
        if (hideTooltipTimeout) {
            clearTimeout(hideTooltipTimeout);
        }
        
        // Hide tooltip after a brief delay to allow user to see final value
        hideTooltipTimeout = setTimeout(() => {
            if (sliderValueTooltip) {
                sliderValueTooltip.classList.add('hidden');
            }
        }, 800); // 800ms delay
    }

    function hideSliderTooltipImmediately() {
        // Clear any existing timeout
        if (hideTooltipTimeout) {
            clearTimeout(hideTooltipTimeout);
            hideTooltipTimeout = null;
        }
        
        // Hide tooltip immediately and reset its position
        if (sliderValueTooltip) {
            sliderValueTooltip.classList.add('hidden');
            // Reset position to prevent positioning issues
            sliderValueTooltip.style.left = '0px';
            sliderValueTooltip.style.top = '0px';
        }
    }

    function showTooltip(event, content) {
        if (!tooltipElement) {
            console.error('[Tooltip] tooltipElement is null');
            return;
        }
        
        // Mark as programmatic tooltip to prevent global handlers from interfering
        tooltipElement.setAttribute('data-programmatic', 'true');
        
        // Remove hiding classes FIRST
        tooltipElement.classList.remove('hidden', 'opacity-0');
        tooltipElement.classList.add('opacity-100');
        
        // Force visibility with !important to override any inherited styles
        tooltipElement.style.setProperty('display', 'block', 'important');
        tooltipElement.style.setProperty('visibility', 'visible', 'important');
        tooltipElement.style.setProperty('opacity', '1', 'important');
        tooltipElement.style.setProperty('z-index', '9999', 'important');
        
        // Set content and position AFTER making visible
        tooltipElement.innerHTML = content;
        positionTooltip(event);
        
    }

    function hideTooltip() {
        if (tooltipElement) {
            tooltipElement.classList.add('hidden', 'opacity-0');
            tooltipElement.classList.remove('opacity-100');
            // Remove all inline styles to let CSS classes handle visibility
            tooltipElement.style.removeProperty('display');
            tooltipElement.style.removeProperty('visibility');
            tooltipElement.style.removeProperty('opacity');
            tooltipElement.style.removeProperty('z-index');
            tooltipElement.removeAttribute('data-programmatic');
        }
    }


    return {
        init,
        updateSliderTooltip,
        hideSliderTooltip,
        hideSliderTooltipImmediately,
        showTooltip,
        hideTooltip
    };
})(); 