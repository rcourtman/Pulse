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
        tooltipElement.style.left = `${event.pageX + offsetX}px`;
        tooltipElement.style.top = `${event.pageY + offsetY}px`;
    }

    function updateSliderTooltip(sliderElement) {
        if (!sliderValueTooltip || !sliderElement) return;

        const type = sliderElement.id.replace('threshold-slider-', '');
        if (!PulseApp.state.getThresholdState()[type]) return; // Only process actual threshold sliders

        const numericValue = parseInt(sliderElement.value);
        let displayText = `${numericValue}%`;

        const rect = sliderElement.getBoundingClientRect();
        const min = parseFloat(sliderElement.min);
        const max = parseFloat(sliderElement.max);
        const value = parseFloat(sliderElement.value);

        const percent = (max > min) ? (value - min) / (max - min) : 0;

        const thumbWidthEstimate = 16;
        let thumbX = rect.left + (percent * (rect.width - thumbWidthEstimate)) + (thumbWidthEstimate / 2);

        sliderValueTooltip.textContent = displayText;
        sliderValueTooltip.classList.remove('hidden');
        const tooltipRect = sliderValueTooltip.getBoundingClientRect();

        const posX = thumbX - (tooltipRect.width / 2);
        const posY = rect.top - tooltipRect.height - 5;

        sliderValueTooltip.style.left = `${posX}px`;
        sliderValueTooltip.style.top = `${posY}px`;
    }

    function hideSliderTooltip() {
        if (sliderValueTooltip) {
            sliderValueTooltip.classList.add('hidden');
        }
    }

    function showTooltip(event, content) {
        if (!tooltipElement) return;
        
        tooltipElement.innerHTML = content;
        positionTooltip(event);
        tooltipElement.classList.remove('hidden', 'opacity-0');
        tooltipElement.classList.add('opacity-100');
    }

    function hideTooltip() {
        if (tooltipElement) {
            tooltipElement.classList.add('hidden', 'opacity-0');
            tooltipElement.classList.remove('opacity-100');
        }
    }

    return {
        init,
        updateSliderTooltip,
        hideSliderTooltip,
        showTooltip,
        hideTooltip
    };
})(); 