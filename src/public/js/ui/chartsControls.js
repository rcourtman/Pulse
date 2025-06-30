// Charts Controls Module
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.chartsControls = (() => {
    let chartsModeControls = null;
    let timeRangeRadios = null;
    let timeRangeSelect = null;
    let chartsToggle = null;

    function init() {
        // Get DOM elements
        chartsModeControls = document.getElementById('charts-mode-controls');
        timeRangeSelect = document.getElementById('time-range-select');
        chartsToggle = document.getElementById('toggle-charts-checkbox');
        
        if (!chartsModeControls || !timeRangeSelect) {
            console.warn('[ChartsControls] Required elements not found');
            return;
        }

        // Setup time range radio listeners
        setupTimeRangeRadios();
        
        // Listen for charts toggle changes
        if (chartsToggle) {
            chartsToggle.addEventListener('change', handleChartsToggleChange);
        }
    }

    function setupTimeRangeRadios() {
        timeRangeRadios = document.querySelectorAll('input[name="time-range"]');
        
        timeRangeRadios.forEach(radio => {
            radio.addEventListener('change', (e) => {
                if (e.target.checked) {
                    const timeRange = e.target.value;
                    updateTimeRangeRadio(timeRange); // Update styling immediately
                    setTimeRange(timeRange);
                }
            });
        });
        
        // Try to restore saved time range on init
        let savedTimeRange = null;
        try {
            savedTimeRange = localStorage.getItem('pulseChartTimeRange');
        } catch (e) {
            // Ignore localStorage errors
        }
        
        // If we have a saved time range and it's valid, use it
        if (savedTimeRange && document.querySelector(`input[name="time-range"][value="${savedTimeRange}"]`)) {
            const savedRadio = document.querySelector(`input[name="time-range"][value="${savedTimeRange}"]`);
            if (savedRadio) {
                savedRadio.checked = true;
                updateTimeRangeRadio(savedTimeRange);
                if (timeRangeSelect) {
                    timeRangeSelect.value = savedTimeRange;
                }
            }
        } else {
            // Otherwise set initial styling for the default selected radio
            const checkedRadio = document.querySelector('input[name="time-range"]:checked');
            if (checkedRadio) {
                updateTimeRangeRadio(checkedRadio.value);
            }
        }
    }

    function setTimeRange(timeRange) {
        // Update the hidden select element
        if (timeRangeSelect) {
            timeRangeSelect.value = timeRange;
            
            // Save to localStorage
            try {
                localStorage.setItem('pulseChartTimeRange', timeRange);
            } catch (e) {
                // Ignore localStorage errors
            }
            
            // Trigger change event for compatibility
            const event = new Event('change', { bubbles: true });
            timeRangeSelect.dispatchEvent(event);
        }
        
        // Update radio button to match
        updateTimeRangeRadio(timeRange);
        
        // Don't fetch data here - let dashboard.js handle it via the select change event
        // This prevents duplicate API calls
    }
    
    function updateTimeRangeRadio(timeRange) {
        const isDarkMode = document.documentElement.classList.contains('dark');
        
        // First, remove active styling from all labels
        document.querySelectorAll('label[data-time-range]').forEach(label => {
            // Remove all possible active classes
            label.classList.remove('bg-blue-100', 'text-blue-700', 'text-blue-300', 'bg-gray-100', 'text-blue-600', 'bg-gray-700', 'text-blue-400', 'bg-blue-900');
            label.className = label.className.replace(/bg-blue-800\/\d+/g, '');
            
            if (isDarkMode) {
                label.classList.add('bg-gray-800');
            } else {
                label.classList.add('bg-white');
            }
        });
        
        // Find and check the matching radio button
        const radio = document.querySelector(`input[name="time-range"][value="${timeRange}"]`);
        if (radio) {
            radio.checked = true;
            
            // Add active styling to the corresponding label
            const label = document.querySelector(`label[data-time-range="${timeRange}"]`);
            if (label) {
                label.classList.remove('bg-white', 'bg-gray-800');
                if (isDarkMode) {
                    // For dark mode, use a more subtle background
                    label.classList.add('bg-gray-700', 'text-blue-400');
                } else {
                    label.classList.add('bg-gray-100', 'text-blue-600');
                }
            }
            
        } else {
            console.warn(`[ChartsControls] Could not find radio button for time range: ${timeRange}`);
        }
    }

    function handleChartsToggleChange() {
        if (chartsToggle && chartsToggle.checked) {
            showChartsControls();
        } else {
            hideChartsControls();
        }
    }

    function showChartsControls() {
        if (chartsModeControls) {
            chartsModeControls.classList.remove('hidden');
            
            // Hide other control panels
            const thresholdControls = document.getElementById('threshold-filter-controls');
            const alertControls = document.getElementById('alert-mode-controls');
            
            if (thresholdControls) thresholdControls.classList.add('hidden');
            if (alertControls) alertControls.classList.add('hidden');
            
            // Try to restore saved time range from localStorage
            let savedTimeRange = null;
            try {
                savedTimeRange = localStorage.getItem('pulseChartTimeRange');
            } catch (e) {
                // Ignore localStorage errors
            }
            
            // If we have a saved time range and it's valid, use it
            if (savedTimeRange && document.querySelector(`input[name="time-range"][value="${savedTimeRange}"]`)) {
                setTimeRange(savedTimeRange);
            } else {
                // Otherwise sync radio button with current time range
                if (timeRangeSelect) {
                    updateTimeRangeRadio(timeRangeSelect.value);
                }
            }
            
            // Update time range availability when showing charts controls
            if (PulseApp.charts && PulseApp.charts.fetchChartData) {
                // Fetch fresh data to get the stats including oldest timestamp
                PulseApp.charts.fetchChartData();
            }
        }
    }

    function hideChartsControls() {
        if (chartsModeControls) {
            chartsModeControls.classList.add('hidden');
        }
    }

    return {
        init,
        showChartsControls,
        hideChartsControls,
        setTimeRange,
        updateTimeRangeRadio
    };
})();