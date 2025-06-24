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
                    setTimeRange(timeRange);
                }
            });
        });
    }

    function setTimeRange(timeRange) {
        // Update the hidden select element
        if (timeRangeSelect) {
            timeRangeSelect.value = timeRange;
            
            // Trigger change event for compatibility
            const event = new Event('change', { bubbles: true });
            timeRangeSelect.dispatchEvent(event);
        }
        
        // Fetch new chart data
        if (PulseApp.charts && PulseApp.charts.getChartData) {
            PulseApp.charts.getChartData().then(() => {
                PulseApp.charts.updateAllCharts(true);
            });
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
        setTimeRange
    };
})();