PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.pbs2 = (() => {
    // State management
    let state = {
        pbsData: [],
        selectedTimeRange: '24h',
        selectedInstance: 0,
        viewMode: 'timeline', // timeline, matrix, flow
        selectedVm: null,
        hoveredTask: null,
        timelineData: new Map(),
        vmBackupPatterns: new Map(),
        insights: []
    };

    // Time range constants
    const TIME_RANGES = {
        '1h': { ms: 60 * 60 * 1000, label: '1 Hour', slots: 12 },
        '6h': { ms: 6 * 60 * 60 * 1000, label: '6 Hours', slots: 24 },
        '24h': { ms: 24 * 60 * 60 * 1000, label: '24 Hours', slots: 48 },
        '7d': { ms: 7 * 24 * 60 * 60 * 1000, label: '7 Days', slots: 28 }
    };

    // Initialize
    function init() {
        console.log('[PBS2] Initializing next-gen PBS dashboard');
        const container = document.getElementById('pbs2-instances-container');
        if (!container) {
            console.error('[PBS2] Container not found');
            return;
        }

        // Get initial data
        const pbsData = PulseApp.state?.get?.('pbsDataArray') || PulseApp.state?.pbs || [];
        state.pbsData = pbsData;
        
        // Process data for insights
        processDataForInsights();
        
        // Render
        render(container);
    }

    // Main render
    function render(container) {
        container.innerHTML = '';
        container.className = 'pbs2-container';
        
        if (!state.pbsData || state.pbsData.length === 0) {
            renderEmptyState(container);
            return;
        }

        // Create main layout
        const layout = document.createElement('div');
        layout.className = 'pbs2-layout';
        
        // Critical alerts banner (if any)
        const alerts = getCriticalAlerts();
        if (alerts.length > 0) {
            layout.appendChild(createAlertsBanner(alerts));
        }
        
        // Main dashboard grid
        const dashboard = document.createElement('div');
        dashboard.className = 'pbs2-dashboard';
        
        // Left panel - Timeline/Matrix/Flow view
        const mainView = document.createElement('div');
        mainView.className = 'pbs2-main-view';
        mainView.appendChild(createViewControls());
        mainView.appendChild(createMainVisualization());
        dashboard.appendChild(mainView);
        
        // Right panel - Context & Insights
        const sidePanel = document.createElement('div');
        sidePanel.className = 'pbs2-side-panel';
        sidePanel.appendChild(createInsightsPanel());
        sidePanel.appendChild(createVmDetailsPanel());
        dashboard.appendChild(sidePanel);
        
        layout.appendChild(dashboard);
        container.appendChild(layout);
        
        // Add styles
        injectStyles();
    }

    // Create view controls
    function createViewControls() {
        const controls = document.createElement('div');
        controls.className = 'pbs2-controls';
        
        controls.innerHTML = `
            <div class="controls-left">
                <div class="view-switcher">
                    <button class="view-btn ${state.viewMode === 'timeline' ? 'active' : ''}" data-view="timeline">
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="3" y="4" width="18" height="4" rx="1"/>
                            <rect x="3" y="10" width="14" height="4" rx="1"/>
                            <rect x="3" y="16" width="10" height="4" rx="1"/>
                        </svg>
                        Timeline
                    </button>
                    <button class="view-btn ${state.viewMode === 'matrix' ? 'active' : ''}" data-view="matrix">
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="3" y="3" width="6" height="6" rx="1"/>
                            <rect x="11" y="3" width="6" height="6" rx="1"/>
                            <rect x="3" y="11" width="6" height="6" rx="1"/>
                            <rect x="11" y="11" width="6" height="6" rx="1"/>
                        </svg>
                        Matrix
                    </button>
                    <button class="view-btn ${state.viewMode === 'flow' ? 'active' : ''}" data-view="flow">
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M5 12h14M12 5l7 7-7 7"/>
                        </svg>
                        Flow
                    </button>
                </div>
                
                <div class="time-selector">
                    ${Object.entries(TIME_RANGES).map(([key, range]) => `
                        <button class="time-btn ${state.selectedTimeRange === key ? 'active' : ''}" data-time="${key}">
                            ${range.label}
                        </button>
                    `).join('')}
                </div>
            </div>
            
            <div class="controls-right">
                <div class="live-indicator">
                    <span class="pulse-dot"></span>
                    <span>Live</span>
                </div>
                
                ${state.pbsData.length > 1 ? `
                    <select class="instance-selector">
                        ${state.pbsData.map((instance, idx) => `
                            <option value="${idx}" ${idx === state.selectedInstance ? 'selected' : ''}>
                                ${instance.pbsInstanceName || `PBS ${idx + 1}`}
                            </option>
                        `).join('')}
                    </select>
                ` : ''}
            </div>
        `;
        
        // Event listeners
        controls.querySelectorAll('.view-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                state.viewMode = btn.dataset.view;
                document.querySelectorAll('.view-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                updateMainVisualization();
            });
        });
        
        controls.querySelectorAll('.time-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                state.selectedTimeRange = btn.dataset.time;
                document.querySelectorAll('.time-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                updateMainVisualization();
            });
        });
        
        const instanceSelector = controls.querySelector('.instance-selector');
        if (instanceSelector) {
            instanceSelector.addEventListener('change', (e) => {
                state.selectedInstance = parseInt(e.target.value);
                processDataForInsights();
                updateMainVisualization();
            });
        }
        
        return controls;
    }

    // Create main visualization
    function createMainVisualization() {
        const container = document.createElement('div');
        container.className = 'pbs2-visualization';
        
        switch (state.viewMode) {
            case 'timeline':
                container.appendChild(createTimelineView());
                break;
            case 'matrix':
                container.appendChild(createMatrixView());
                break;
            case 'flow':
                container.appendChild(createFlowView());
                break;
        }
        
        return container;
    }

    // Timeline view - shows backup activity over time
    function createTimelineView() {
        const timeline = document.createElement('div');
        timeline.className = 'timeline-view';
        
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return timeline;
        
        // Process tasks into timeline slots
        const range = TIME_RANGES[state.selectedTimeRange];
        const now = Date.now();
        const startTime = now - range.ms;
        const slotDuration = range.ms / range.slots;
        
        // Group tasks by VM and time slot
        const vmTimelines = new Map();
        const allVms = new Set();
        
        ['backupTasks', 'verificationTasks'].forEach(taskType => {
            if (instance[taskType]?.recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    const vm = parseVmFromTask(task);
                    if (!vm) return;
                    
                    allVms.add(vm);
                    if (!vmTimelines.has(vm)) {
                        vmTimelines.set(vm, Array(range.slots).fill(null).map(() => []));
                    }
                    
                    const taskStartMs = (task.startTime || 0) * 1000;
                    if (taskStartMs >= startTime) {
                        const slot = Math.floor((taskStartMs - startTime) / slotDuration);
                        if (slot >= 0 && slot < range.slots) {
                            vmTimelines.get(vm)[slot].push({
                                ...task,
                                type: taskType === 'backupTasks' ? 'backup' : 'verify'
                            });
                        }
                    }
                });
            }
        });
        
        // Create timeline header
        const header = document.createElement('div');
        header.className = 'timeline-header';
        
        // Time labels
        const timeLabels = document.createElement('div');
        timeLabels.className = 'time-labels';
        for (let i = 0; i < range.slots; i++) {
            const label = document.createElement('div');
            label.className = 'time-label';
            if (i % Math.floor(range.slots / 8) === 0) {
                const time = new Date(startTime + i * slotDuration);
                label.textContent = formatTimeLabel(time, state.selectedTimeRange);
            }
            timeLabels.appendChild(label);
        }
        header.appendChild(timeLabels);
        timeline.appendChild(header);
        
        // VM rows
        const rows = document.createElement('div');
        rows.className = 'timeline-rows';
        
        Array.from(allVms).sort().forEach(vm => {
            const row = document.createElement('div');
            row.className = 'vm-row';
            
            // VM label
            const label = document.createElement('div');
            label.className = 'vm-label';
            label.textContent = vm;
            label.title = vm;
            row.appendChild(label);
            
            // Timeline slots
            const slots = document.createElement('div');
            slots.className = 'timeline-slots';
            
            const vmSlots = vmTimelines.get(vm) || Array(range.slots).fill(null).map(() => []);
            vmSlots.forEach((tasks, idx) => {
                const slot = document.createElement('div');
                slot.className = 'time-slot';
                
                if (tasks.length > 0) {
                    const primaryTask = tasks[0];
                    const status = getTaskStatus(primaryTask);
                    slot.classList.add(`status-${status}`, `type-${primaryTask.type}`);
                    
                    if (tasks.length > 1) {
                        slot.classList.add('multiple-tasks');
                        slot.setAttribute('data-count', tasks.length);
                    }
                    
                    // Hover tooltip
                    slot.addEventListener('mouseenter', (e) => {
                        showTaskTooltip(e, tasks);
                    });
                    slot.addEventListener('mouseleave', hideTooltip);
                    
                    // Click to select
                    slot.addEventListener('click', () => {
                        state.selectedVm = vm;
                        updateVmDetails();
                    });
                }
                
                slots.appendChild(slot);
            });
            
            row.appendChild(slots);
            rows.appendChild(row);
        });
        
        timeline.appendChild(rows);
        
        // Add current time indicator
        const currentTimeOffset = (now - startTime) / range.ms;
        if (currentTimeOffset >= 0 && currentTimeOffset <= 1) {
            const indicator = document.createElement('div');
            indicator.className = 'current-time-indicator';
            indicator.style.left = `${currentTimeOffset * 100}%`;
            timeline.appendChild(indicator);
        }
        
        return timeline;
    }

    // Matrix view - shows VM backup health at a glance
    function createMatrixView() {
        const matrix = document.createElement('div');
        matrix.className = 'matrix-view';
        
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return matrix;
        
        // Calculate VM health metrics
        const vmMetrics = calculateVmMetrics(instance);
        
        // Sort VMs by health score
        const sortedVms = Array.from(vmMetrics.entries())
            .sort((a, b) => a[1].healthScore - b[1].healthScore);
        
        // Create grid
        const grid = document.createElement('div');
        grid.className = 'vm-grid';
        
        sortedVms.forEach(([vm, metrics]) => {
            const cell = document.createElement('div');
            cell.className = 'vm-cell';
            cell.classList.add(`health-${getHealthLevel(metrics.healthScore)}`);
            
            // Background gradient based on success rate
            const gradient = `linear-gradient(135deg, 
                ${getHealthColor(metrics.successRate)} 0%, 
                ${getHealthColor(metrics.successRate * 0.8)} 100%)`;
            cell.style.background = gradient;
            
            cell.innerHTML = `
                <div class="vm-name">${vm}</div>
                <div class="vm-stats">
                    <div class="stat">
                        <span class="value">${metrics.successRate}%</span>
                        <span class="label">Success</span>
                    </div>
                    <div class="stat">
                        <span class="value">${metrics.lastBackup}</span>
                        <span class="label">Last Backup</span>
                    </div>
                </div>
                <div class="vm-mini-chart">
                    ${createSparkline(metrics.recentHistory)}
                </div>
            `;
            
            // Hover effect
            cell.addEventListener('mouseenter', () => {
                cell.classList.add('hover');
                showVmMetricsTooltip(cell, vm, metrics);
            });
            cell.addEventListener('mouseleave', () => {
                cell.classList.remove('hover');
                hideTooltip();
            });
            
            // Click to view details
            cell.addEventListener('click', () => {
                state.selectedVm = vm;
                updateVmDetails();
            });
            
            grid.appendChild(cell);
        });
        
        matrix.appendChild(grid);
        return matrix;
    }

    // Flow view - shows backup job dependencies and patterns
    function createFlowView() {
        const flow = document.createElement('div');
        flow.className = 'flow-view';
        
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return flow;
        
        // Analyze backup patterns
        const patterns = analyzeBackupPatterns(instance);
        
        // Create flow visualization
        const canvas = document.createElement('canvas');
        canvas.className = 'flow-canvas';
        flow.appendChild(canvas);
        
        // Set canvas size
        const rect = flow.getBoundingClientRect();
        canvas.width = rect.width || 800;
        canvas.height = 600;
        
        const ctx = canvas.getContext('2d');
        
        // Draw flow diagram
        drawBackupFlow(ctx, patterns, canvas.width, canvas.height);
        
        // Add legend
        const legend = document.createElement('div');
        legend.className = 'flow-legend';
        legend.innerHTML = `
            <div class="legend-item">
                <div class="legend-color" style="background: #3b82f6"></div>
                <span>Backup Job</span>
            </div>
            <div class="legend-item">
                <div class="legend-color" style="background: #10b981"></div>
                <span>Verification</span>
            </div>
            <div class="legend-item">
                <div class="legend-color" style="background: #f59e0b"></div>
                <span>Running</span>
            </div>
            <div class="legend-item">
                <div class="legend-color" style="background: #ef4444"></div>
                <span>Failed</span>
            </div>
        `;
        flow.appendChild(legend);
        
        return flow;
    }

    // Create insights panel
    function createInsightsPanel() {
        const panel = document.createElement('div');
        panel.className = 'insights-panel';
        
        panel.innerHTML = `
            <h3 class="panel-title">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"/>
                </svg>
                Insights & Recommendations
            </h3>
        `;
        
        const insightsList = document.createElement('div');
        insightsList.className = 'insights-list';
        
        state.insights.forEach(insight => {
            const item = document.createElement('div');
            item.className = `insight-item priority-${insight.priority}`;
            
            item.innerHTML = `
                <div class="insight-icon">${getInsightIcon(insight.type)}</div>
                <div class="insight-content">
                    <div class="insight-title">${insight.title}</div>
                    <div class="insight-description">${insight.description}</div>
                    ${insight.action ? `
                        <button class="insight-action">${insight.action}</button>
                    ` : ''}
                </div>
            `;
            
            insightsList.appendChild(item);
        });
        
        panel.appendChild(insightsList);
        return panel;
    }

    // Create VM details panel
    function createVmDetailsPanel() {
        const panel = document.createElement('div');
        panel.className = 'vm-details-panel';
        
        if (!state.selectedVm) {
            panel.innerHTML = `
                <div class="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1">
                        <rect x="3" y="4" width="18" height="18" rx="2" ry="2"/>
                        <line x1="9" y1="9" x2="15" y2="9"/>
                        <line x1="9" y1="13" x2="15" y2="13"/>
                        <line x1="9" y1="17" x2="13" y2="17"/>
                    </svg>
                    <p>Select a VM to view details</p>
                </div>
            `;
            return panel;
        }
        
        const instance = state.pbsData[state.selectedInstance];
        const vmData = getVmData(instance, state.selectedVm);
        
        panel.innerHTML = `
            <div class="vm-header">
                <h3>${state.selectedVm}</h3>
                <button class="close-btn" onclick="PulseApp.ui.pbs2.clearSelection()">×</button>
            </div>
            
            <div class="vm-metrics">
                <div class="metric">
                    <span class="metric-value">${vmData.totalBackups}</span>
                    <span class="metric-label">Total Backups</span>
                </div>
                <div class="metric">
                    <span class="metric-value ${vmData.successRate < 95 ? 'warning' : ''}">${vmData.successRate}%</span>
                    <span class="metric-label">Success Rate</span>
                </div>
                <div class="metric">
                    <span class="metric-value">${vmData.avgDuration}</span>
                    <span class="metric-label">Avg Duration</span>
                </div>
            </div>
            
            <div class="vm-chart">
                <h4>Backup History</h4>
                <canvas id="vm-backup-chart" width="300" height="150"></canvas>
            </div>
            
            <div class="vm-recent-tasks">
                <h4>Recent Tasks</h4>
                <div class="task-list">
                    ${vmData.recentTasks.map(task => `
                        <div class="task-item status-${getTaskStatus(task)}">
                            <span class="task-type">${task.type}</span>
                            <span class="task-time">${formatTaskTime(task)}</span>
                            <span class="task-status">${getTaskStatusLabel(task)}</span>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
        
        // Draw chart
        setTimeout(() => {
            const canvas = document.getElementById('vm-backup-chart');
            if (canvas) {
                drawVmBackupChart(canvas, vmData);
            }
        }, 100);
        
        return panel;
    }

    // Process data for insights
    function processDataForInsights() {
        state.insights = [];
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return;
        
        // Analyze patterns and generate insights
        const vmMetrics = calculateVmMetrics(instance);
        
        // Check for VMs with poor success rates
        vmMetrics.forEach((metrics, vm) => {
            if (metrics.successRate < 90) {
                state.insights.push({
                    type: 'warning',
                    priority: 'high',
                    title: `${vm} has low backup success rate`,
                    description: `Only ${metrics.successRate}% of backups succeeded in the last 7 days. Consider investigating the root cause.`,
                    action: 'View Details'
                });
            }
        });
        
        // Check for VMs without recent backups
        const now = Date.now() / 1000;
        vmMetrics.forEach((metrics, vm) => {
            if (metrics.lastBackupTime && (now - metrics.lastBackupTime) > 86400) {
                const hours = Math.floor((now - metrics.lastBackupTime) / 3600);
                state.insights.push({
                    type: 'alert',
                    priority: 'medium',
                    title: `${vm} backup is overdue`,
                    description: `Last backup was ${hours} hours ago. Expected backup frequency is 24 hours.`,
                    action: 'Schedule Backup'
                });
            }
        });
        
        // Check for backup performance trends
        const avgDurations = Array.from(vmMetrics.values()).map(m => m.avgDurationSeconds).filter(d => d > 0);
        if (avgDurations.length > 0) {
            const overallAvg = avgDurations.reduce((a, b) => a + b, 0) / avgDurations.length;
            const slowVms = Array.from(vmMetrics.entries())
                .filter(([_, m]) => m.avgDurationSeconds > overallAvg * 1.5)
                .map(([vm, _]) => vm);
            
            if (slowVms.length > 0) {
                state.insights.push({
                    type: 'info',
                    priority: 'low',
                    title: 'Slow backup performance detected',
                    description: `VMs ${slowVms.slice(0, 3).join(', ')} take 50% longer than average to backup. Consider optimization.`,
                    action: 'Analyze'
                });
            }
        }
        
        // Add positive insights
        const perfectVms = Array.from(vmMetrics.entries())
            .filter(([_, m]) => m.successRate === 100)
            .map(([vm, _]) => vm);
        
        if (perfectVms.length > 0) {
            state.insights.push({
                type: 'success',
                priority: 'low',
                title: `${perfectVms.length} VMs with perfect backup record`,
                description: `${perfectVms.slice(0, 5).join(', ')} have 100% success rate.`
            });
        }
    }

    // Utility functions
    function parseVmFromTask(task) {
        if (!task.upid) return null;
        const parts = task.upid.split(':');
        if (parts.length >= 3 && parts[2]) {
            return parts[2].replace('vm/', 'VM ');
        }
        return null;
    }

    function getTaskStatus(task) {
        if (!task.status) return 'running';
        if (task.status === 'OK') return 'success';
        return 'failed';
    }

    function formatTimeLabel(date, range) {
        if (range === '1h' || range === '6h') {
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        } else if (range === '24h') {
            return date.toLocaleTimeString([], { hour: '2-digit' });
        } else {
            return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
        }
    }

    function calculateVmMetrics(instance) {
        const metrics = new Map();
        const now = Date.now() / 1000;
        const cutoff = now - (7 * 24 * 60 * 60); // 7 days
        
        ['backupTasks', 'verificationTasks'].forEach(taskType => {
            if (instance[taskType]?.recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    const vm = parseVmFromTask(task);
                    if (!vm || !task.startTime || task.startTime < cutoff) return;
                    
                    if (!metrics.has(vm)) {
                        metrics.set(vm, {
                            totalTasks: 0,
                            successfulTasks: 0,
                            failedTasks: 0,
                            lastBackupTime: 0,
                            totalDuration: 0,
                            recentHistory: [],
                            healthScore: 100
                        });
                    }
                    
                    const vmMetrics = metrics.get(vm);
                    vmMetrics.totalTasks++;
                    
                    if (task.status === 'OK') {
                        vmMetrics.successfulTasks++;
                    } else if (task.status && task.status !== 'Running') {
                        vmMetrics.failedTasks++;
                    }
                    
                    if (taskType === 'backupTasks' && task.startTime > vmMetrics.lastBackupTime) {
                        vmMetrics.lastBackupTime = task.startTime;
                    }
                    
                    if (task.endTime && task.startTime) {
                        vmMetrics.totalDuration += (task.endTime - task.startTime);
                    }
                    
                    vmMetrics.recentHistory.push({
                        time: task.startTime,
                        success: task.status === 'OK'
                    });
                });
            }
        });
        
        // Calculate derived metrics
        metrics.forEach((vmMetrics, vm) => {
            vmMetrics.successRate = vmMetrics.totalTasks > 0 
                ? Math.round((vmMetrics.successfulTasks / vmMetrics.totalTasks) * 100)
                : 100;
            
            vmMetrics.avgDurationSeconds = vmMetrics.totalTasks > 0
                ? Math.round(vmMetrics.totalDuration / vmMetrics.totalTasks)
                : 0;
            
            vmMetrics.avgDuration = formatDuration(vmMetrics.avgDurationSeconds);
            
            vmMetrics.lastBackup = vmMetrics.lastBackupTime > 0
                ? formatRelativeTime(now - vmMetrics.lastBackupTime)
                : 'Never';
            
            // Calculate health score
            let health = 100;
            health -= (100 - vmMetrics.successRate) * 0.5; // Success rate weight
            if (vmMetrics.lastBackupTime > 0) {
                const hoursSinceBackup = (now - vmMetrics.lastBackupTime) / 3600;
                if (hoursSinceBackup > 48) health -= 20;
                else if (hoursSinceBackup > 24) health -= 10;
            }
            vmMetrics.healthScore = Math.max(0, Math.round(health));
            
            // Sort history by time
            vmMetrics.recentHistory.sort((a, b) => a.time - b.time);
        });
        
        return metrics;
    }

    function getHealthLevel(score) {
        if (score >= 95) return 'excellent';
        if (score >= 80) return 'good';
        if (score >= 60) return 'warning';
        return 'critical';
    }

    function getHealthColor(score) {
        if (score >= 95) return '#10b981';
        if (score >= 80) return '#3b82f6';
        if (score >= 60) return '#f59e0b';
        return '#ef4444';
    }

    function createSparkline(history) {
        if (!history || history.length === 0) return '';
        
        const width = 100;
        const height = 30;
        const points = history.slice(-20).map((item, idx) => {
            const x = (idx / (history.length - 1)) * width;
            const y = item.success ? 5 : height - 5;
            return `${x},${y}`;
        }).join(' ');
        
        return `
            <svg width="${width}" height="${height}" class="sparkline">
                <polyline points="${points}" fill="none" stroke="currentColor" stroke-width="2"/>
            </svg>
        `;
    }

    function formatDuration(seconds) {
        if (seconds < 60) return `${seconds}s`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
        return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
    }

    function formatRelativeTime(seconds) {
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
        return `${Math.floor(seconds / 86400)}d ago`;
    }

    function getInsightIcon(type) {
        const icons = {
            warning: '⚠️',
            alert: '🔔',
            info: 'ℹ️',
            success: '✅'
        };
        return icons[type] || '📌';
    }

    // Update functions
    function updateMainVisualization() {
        const container = document.querySelector('.pbs2-visualization');
        if (!container) return;
        
        container.innerHTML = '';
        
        switch (state.viewMode) {
            case 'timeline':
                container.appendChild(createTimelineView());
                break;
            case 'matrix':
                container.appendChild(createMatrixView());
                break;
            case 'flow':
                container.appendChild(createFlowView());
                break;
        }
    }

    function updateVmDetails() {
        const panel = document.querySelector('.vm-details-panel');
        if (!panel) return;
        
        const newPanel = createVmDetailsPanel();
        panel.replaceWith(newPanel);
    }

    // Main update function
    function updatePbsInfo(pbsArray) {
        console.log('[PBS2] Received update with data:', pbsArray);
        state.pbsData = pbsArray || [];
        
        processDataForInsights();
        
        const container = document.getElementById('pbs2-instances-container');
        if (container) {
            render(container);
        }
    }

    // Inject custom styles
    function injectStyles() {
        const styleId = 'pbs2-styles';
        if (document.getElementById(styleId)) return;
        
        const style = document.createElement('style');
        style.id = styleId;
        style.textContent = `
            .pbs2-container {
                height: 100%;
                display: flex;
                flex-direction: column;
                background: var(--bg-primary);
                color: var(--text-primary);
                --bg-primary: #ffffff;
                --bg-secondary: #f9fafb;
                --bg-tertiary: #f3f4f6;
                --text-primary: #111827;
                --text-secondary: #6b7280;
                --border-color: #e5e7eb;
                --success-color: #10b981;
                --warning-color: #f59e0b;
                --error-color: #ef4444;
                --info-color: #3b82f6;
            }
            
            .dark .pbs2-container {
                --bg-primary: #1f2937;
                --bg-secondary: #111827;
                --bg-tertiary: #374151;
                --text-primary: #f9fafb;
                --text-secondary: #9ca3af;
                --border-color: #374151;
            }
            
            .pbs2-dashboard {
                display: grid;
                grid-template-columns: 1fr 400px;
                gap: 1rem;
                height: calc(100vh - 200px);
                padding: 1rem;
            }
            
            .pbs2-main-view {
                display: flex;
                flex-direction: column;
                background: var(--bg-secondary);
                border-radius: 0.5rem;
                overflow: hidden;
            }
            
            .pbs2-controls {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 1rem;
                background: var(--bg-primary);
                border-bottom: 1px solid var(--border-color);
            }
            
            .controls-left {
                display: flex;
                gap: 1rem;
            }
            
            .view-switcher {
                display: flex;
                gap: 0.5rem;
                background: var(--bg-tertiary);
                padding: 0.25rem;
                border-radius: 0.375rem;
            }
            
            .view-btn {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                padding: 0.5rem 1rem;
                background: transparent;
                border: none;
                border-radius: 0.25rem;
                color: var(--text-secondary);
                cursor: pointer;
                transition: all 0.2s;
            }
            
            .view-btn:hover {
                color: var(--text-primary);
            }
            
            .view-btn.active {
                background: var(--bg-primary);
                color: var(--info-color);
                box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            }
            
            .time-selector {
                display: flex;
                gap: 0.25rem;
            }
            
            .time-btn {
                padding: 0.5rem 1rem;
                background: transparent;
                border: 1px solid var(--border-color);
                border-radius: 0.25rem;
                color: var(--text-secondary);
                cursor: pointer;
                transition: all 0.2s;
            }
            
            .time-btn:hover {
                border-color: var(--info-color);
                color: var(--info-color);
            }
            
            .time-btn.active {
                background: var(--info-color);
                color: white;
                border-color: var(--info-color);
            }
            
            .live-indicator {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                color: var(--success-color);
                font-size: 0.875rem;
            }
            
            .pulse-dot {
                width: 8px;
                height: 8px;
                background: var(--success-color);
                border-radius: 50%;
                animation: pulse 2s infinite;
            }
            
            @keyframes pulse {
                0%, 100% { opacity: 1; transform: scale(1); }
                50% { opacity: 0.5; transform: scale(1.2); }
            }
            
            .pbs2-visualization {
                flex: 1;
                overflow: auto;
                padding: 1rem;
            }
            
            /* Timeline View */
            .timeline-view {
                position: relative;
                min-width: 800px;
            }
            
            .timeline-header {
                position: sticky;
                top: 0;
                background: var(--bg-secondary);
                z-index: 10;
                padding-bottom: 0.5rem;
            }
            
            .time-labels {
                display: flex;
                margin-left: 120px;
                position: relative;
                height: 30px;
            }
            
            .time-label {
                flex: 1;
                font-size: 0.75rem;
                color: var(--text-secondary);
                text-align: center;
            }
            
            .timeline-rows {
                display: flex;
                flex-direction: column;
                gap: 0.25rem;
            }
            
            .vm-row {
                display: flex;
                align-items: center;
                min-height: 40px;
            }
            
            .vm-label {
                width: 120px;
                padding-right: 1rem;
                font-size: 0.875rem;
                color: var(--text-primary);
                text-overflow: ellipsis;
                overflow: hidden;
                white-space: nowrap;
            }
            
            .timeline-slots {
                flex: 1;
                display: flex;
                gap: 1px;
                background: var(--border-color);
                padding: 1px;
                border-radius: 0.25rem;
            }
            
            .time-slot {
                flex: 1;
                height: 36px;
                background: var(--bg-primary);
                border-radius: 0.125rem;
                cursor: pointer;
                transition: all 0.2s;
                position: relative;
            }
            
            .time-slot.status-success.type-backup {
                background: var(--success-color);
                opacity: 0.8;
            }
            
            .time-slot.status-success.type-verify {
                background: var(--info-color);
                opacity: 0.8;
            }
            
            .time-slot.status-failed {
                background: var(--error-color);
                opacity: 0.8;
            }
            
            .time-slot.status-running {
                background: var(--warning-color);
                opacity: 0.8;
                animation: pulse 2s infinite;
            }
            
            .time-slot:hover {
                opacity: 1;
                transform: scale(1.05);
                z-index: 5;
            }
            
            .time-slot.multiple-tasks::after {
                content: attr(data-count);
                position: absolute;
                top: 2px;
                right: 2px;
                font-size: 0.625rem;
                background: rgba(0,0,0,0.5);
                color: white;
                padding: 0 4px;
                border-radius: 2px;
            }
            
            .current-time-indicator {
                position: absolute;
                top: 0;
                bottom: 0;
                width: 2px;
                background: var(--error-color);
                z-index: 15;
                pointer-events: none;
            }
            
            .current-time-indicator::before {
                content: '';
                position: absolute;
                top: -4px;
                left: -4px;
                width: 10px;
                height: 10px;
                background: var(--error-color);
                border-radius: 50%;
            }
            
            /* Matrix View */
            .matrix-view {
                padding: 1rem;
            }
            
            .vm-grid {
                display: grid;
                grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
                gap: 1rem;
            }
            
            .vm-cell {
                padding: 1.5rem;
                border-radius: 0.5rem;
                color: white;
                cursor: pointer;
                transition: all 0.3s;
                position: relative;
                overflow: hidden;
            }
            
            .vm-cell:hover {
                transform: translateY(-2px);
                box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            }
            
            .vm-name {
                font-weight: 600;
                margin-bottom: 0.5rem;
            }
            
            .vm-stats {
                display: flex;
                gap: 1rem;
                margin-bottom: 0.5rem;
            }
            
            .stat {
                display: flex;
                flex-direction: column;
            }
            
            .stat .value {
                font-size: 1.25rem;
                font-weight: 600;
            }
            
            .stat .label {
                font-size: 0.75rem;
                opacity: 0.8;
            }
            
            .vm-mini-chart {
                margin-top: 0.5rem;
                opacity: 0.8;
            }
            
            /* Side Panel */
            .pbs2-side-panel {
                display: flex;
                flex-direction: column;
                gap: 1rem;
            }
            
            .insights-panel,
            .vm-details-panel {
                background: var(--bg-secondary);
                border-radius: 0.5rem;
                padding: 1.5rem;
            }
            
            .panel-title {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                font-size: 1rem;
                font-weight: 600;
                margin-bottom: 1rem;
                color: var(--text-primary);
            }
            
            .insights-list {
                display: flex;
                flex-direction: column;
                gap: 0.75rem;
            }
            
            .insight-item {
                display: flex;
                gap: 0.75rem;
                padding: 0.75rem;
                background: var(--bg-primary);
                border-radius: 0.375rem;
                border-left: 3px solid;
                transition: all 0.2s;
            }
            
            .insight-item:hover {
                transform: translateX(2px);
            }
            
            .insight-item.priority-high {
                border-left-color: var(--error-color);
            }
            
            .insight-item.priority-medium {
                border-left-color: var(--warning-color);
            }
            
            .insight-item.priority-low {
                border-left-color: var(--info-color);
            }
            
            .insight-icon {
                font-size: 1.25rem;
                flex-shrink: 0;
            }
            
            .insight-content {
                flex: 1;
            }
            
            .insight-title {
                font-weight: 500;
                color: var(--text-primary);
                margin-bottom: 0.25rem;
            }
            
            .insight-description {
                font-size: 0.875rem;
                color: var(--text-secondary);
                line-height: 1.5;
            }
            
            .insight-action {
                margin-top: 0.5rem;
                padding: 0.25rem 0.75rem;
                background: var(--info-color);
                color: white;
                border: none;
                border-radius: 0.25rem;
                font-size: 0.875rem;
                cursor: pointer;
                transition: all 0.2s;
            }
            
            .insight-action:hover {
                background: var(--info-color);
                opacity: 0.9;
            }
            
            /* VM Details */
            .vm-details-panel {
                flex: 1;
                display: flex;
                flex-direction: column;
            }
            
            .empty-state {
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;
                padding: 3rem;
                color: var(--text-secondary);
                text-align: center;
            }
            
            .empty-state svg {
                margin-bottom: 1rem;
                opacity: 0.3;
            }
            
            .vm-header {
                display: flex;
                justify-content: space-between;
                align-items: center;
                margin-bottom: 1rem;
            }
            
            .vm-header h3 {
                font-size: 1.125rem;
                font-weight: 600;
                color: var(--text-primary);
            }
            
            .close-btn {
                width: 24px;
                height: 24px;
                background: none;
                border: none;
                color: var(--text-secondary);
                cursor: pointer;
                font-size: 1.25rem;
                line-height: 1;
                border-radius: 0.25rem;
                transition: all 0.2s;
            }
            
            .close-btn:hover {
                background: var(--bg-tertiary);
                color: var(--text-primary);
            }
            
            .vm-metrics {
                display: grid;
                grid-template-columns: repeat(3, 1fr);
                gap: 0.75rem;
                margin-bottom: 1.5rem;
            }
            
            .metric {
                text-align: center;
                padding: 0.75rem;
                background: var(--bg-primary);
                border-radius: 0.375rem;
            }
            
            .metric-value {
                display: block;
                font-size: 1.5rem;
                font-weight: 600;
                color: var(--text-primary);
            }
            
            .metric-value.warning {
                color: var(--warning-color);
            }
            
            .metric-label {
                display: block;
                font-size: 0.75rem;
                color: var(--text-secondary);
                margin-top: 0.25rem;
            }
            
            .vm-chart {
                margin-bottom: 1.5rem;
            }
            
            .vm-chart h4 {
                font-size: 0.875rem;
                font-weight: 500;
                color: var(--text-secondary);
                margin-bottom: 0.75rem;
            }
            
            .vm-recent-tasks h4 {
                font-size: 0.875rem;
                font-weight: 500;
                color: var(--text-secondary);
                margin-bottom: 0.75rem;
            }
            
            .task-list {
                display: flex;
                flex-direction: column;
                gap: 0.5rem;
            }
            
            .task-item {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 0.5rem;
                background: var(--bg-primary);
                border-radius: 0.25rem;
                font-size: 0.875rem;
            }
            
            .task-type {
                font-weight: 500;
                color: var(--text-primary);
            }
            
            .task-time {
                color: var(--text-secondary);
            }
            
            .task-status {
                padding: 0.125rem 0.5rem;
                border-radius: 0.25rem;
                font-size: 0.75rem;
                font-weight: 500;
            }
            
            .status-success .task-status {
                background: var(--success-color);
                color: white;
            }
            
            .status-failed .task-status {
                background: var(--error-color);
                color: white;
            }
            
            .status-running .task-status {
                background: var(--warning-color);
                color: white;
            }
            
            /* Tooltips */
            .pbs2-tooltip {
                position: fixed;
                background: var(--bg-primary);
                border: 1px solid var(--border-color);
                border-radius: 0.375rem;
                padding: 0.75rem;
                box-shadow: 0 4px 12px rgba(0,0,0,0.1);
                z-index: 1000;
                pointer-events: none;
                max-width: 300px;
            }
            
            .pbs2-tooltip .tooltip-title {
                font-weight: 500;
                margin-bottom: 0.5rem;
                color: var(--text-primary);
            }
            
            .pbs2-tooltip .tooltip-content {
                font-size: 0.875rem;
                color: var(--text-secondary);
            }
            
            /* Alerts Banner */
            .alerts-banner {
                background: var(--error-color);
                color: white;
                padding: 0.75rem 1rem;
                display: flex;
                align-items: center;
                gap: 0.5rem;
                border-radius: 0.5rem;
                margin-bottom: 1rem;
            }
            
            .alerts-banner svg {
                flex-shrink: 0;
            }
            
            .alerts-content {
                flex: 1;
            }
            
            /* Responsive */
            @media (max-width: 1200px) {
                .pbs2-dashboard {
                    grid-template-columns: 1fr;
                }
                
                .pbs2-side-panel {
                    flex-direction: row;
                }
                
                .insights-panel,
                .vm-details-panel {
                    flex: 1;
                }
            }
        `;
        
        document.head.appendChild(style);
    }

    // Helper functions for empty implementations
    function renderEmptyState(container) {
        container.innerHTML = `
            <div class="empty-state" style="height: 100%; display: flex; align-items: center; justify-content: center;">
                <div style="text-align: center; color: var(--text-secondary);">
                    <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" style="margin: 0 auto 1rem;">
                        <path d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/>
                    </svg>
                    <h3 style="font-size: 1.125rem; font-weight: 600; margin-bottom: 0.5rem;">No PBS Servers Connected</h3>
                    <p style="color: var(--text-secondary);">Configure Proxmox Backup Server integration to see backup status.</p>
                </div>
            </div>
        `;
    }

    function getCriticalAlerts() {
        const alerts = [];
        const instance = state.pbsData[state.selectedInstance];
        if (!instance) return alerts;
        
        // Check for recent failures
        let recentFailures = 0;
        ['backupTasks', 'verificationTasks'].forEach(taskType => {
            if (instance[taskType]?.recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    if (task.status && task.status !== 'OK' && task.status !== 'Running') {
                        const age = (Date.now() / 1000) - (task.startTime || 0);
                        if (age < 3600) { // Last hour
                            recentFailures++;
                        }
                    }
                });
            }
        });
        
        if (recentFailures > 0) {
            alerts.push({
                type: 'error',
                message: `${recentFailures} backup failures in the last hour`
            });
        }
        
        return alerts;
    }

    function createAlertsBanner(alerts) {
        const banner = document.createElement('div');
        banner.className = 'alerts-banner';
        
        banner.innerHTML = `
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
                <line x1="12" y1="9" x2="12" y2="13"/>
                <line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
            <div class="alerts-content">
                ${alerts.map(a => a.message).join(' • ')}
            </div>
        `;
        
        return banner;
    }

    function showTaskTooltip(event, tasks) {
        hideTooltip();
        
        const tooltip = document.createElement('div');
        tooltip.className = 'pbs2-tooltip';
        
        const content = tasks.map(task => {
            const vm = parseVmFromTask(task);
            const status = getTaskStatus(task);
            const time = task.startTime ? new Date(task.startTime * 1000).toLocaleTimeString() : 'Unknown';
            return `
                <div style="margin-bottom: 0.5rem;">
                    <div class="tooltip-title">${vm} - ${task.type}</div>
                    <div class="tooltip-content">
                        Status: ${status}<br>
                        Started: ${time}
                        ${task.endTime ? `<br>Duration: ${formatDuration(task.endTime - task.startTime)}` : ''}
                    </div>
                </div>
            `;
        }).join('');
        
        tooltip.innerHTML = content;
        document.body.appendChild(tooltip);
        
        // Position tooltip
        const rect = event.target.getBoundingClientRect();
        tooltip.style.left = `${rect.left}px`;
        tooltip.style.top = `${rect.bottom + 5}px`;
    }

    function showVmMetricsTooltip(element, vm, metrics) {
        hideTooltip();
        
        const tooltip = document.createElement('div');
        tooltip.className = 'pbs2-tooltip';
        
        tooltip.innerHTML = `
            <div class="tooltip-title">${vm}</div>
            <div class="tooltip-content">
                Total Tasks: ${metrics.totalTasks}<br>
                Successful: ${metrics.successfulTasks}<br>
                Failed: ${metrics.failedTasks}<br>
                Health Score: ${metrics.healthScore}/100
            </div>
        `;
        
        document.body.appendChild(tooltip);
        
        // Position tooltip
        const rect = element.getBoundingClientRect();
        tooltip.style.left = `${rect.left}px`;
        tooltip.style.top = `${rect.bottom + 5}px`;
    }

    function hideTooltip() {
        const tooltip = document.querySelector('.pbs2-tooltip');
        if (tooltip) {
            tooltip.remove();
        }
    }

    function analyzeBackupPatterns(instance) {
        // Simplified pattern analysis
        const patterns = {
            nodes: [],
            links: []
        };
        
        // Create nodes for each VM
        const vms = new Set();
        ['backupTasks'].forEach(taskType => {
            if (instance[taskType]?.recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    const vm = parseVmFromTask(task);
                    if (vm) vms.add(vm);
                });
            }
        });
        
        Array.from(vms).forEach((vm, idx) => {
            patterns.nodes.push({
                id: vm,
                label: vm,
                x: Math.cos(idx * 2 * Math.PI / vms.size) * 200 + 400,
                y: Math.sin(idx * 2 * Math.PI / vms.size) * 200 + 300
            });
        });
        
        return patterns;
    }

    function drawBackupFlow(ctx, patterns, width, height) {
        ctx.clearRect(0, 0, width, height);
        
        // Draw nodes
        patterns.nodes.forEach(node => {
            ctx.beginPath();
            ctx.arc(node.x, node.y, 30, 0, 2 * Math.PI);
            ctx.fillStyle = '#3b82f6';
            ctx.fill();
            
            ctx.fillStyle = '#ffffff';
            ctx.font = '12px sans-serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(node.label, node.x, node.y);
        });
    }

    function getVmData(instance, vm) {
        const tasks = [];
        let totalDuration = 0;
        let successCount = 0;
        
        ['backupTasks', 'verificationTasks'].forEach(taskType => {
            if (instance[taskType]?.recentTasks) {
                instance[taskType].recentTasks.forEach(task => {
                    if (parseVmFromTask(task) === vm) {
                        tasks.push({
                            ...task,
                            type: taskType === 'backupTasks' ? 'Backup' : 'Verify'
                        });
                        
                        if (task.status === 'OK') successCount++;
                        if (task.endTime && task.startTime) {
                            totalDuration += (task.endTime - task.startTime);
                        }
                    }
                });
            }
        });
        
        tasks.sort((a, b) => (b.startTime || 0) - (a.startTime || 0));
        
        return {
            totalBackups: tasks.length,
            successRate: tasks.length > 0 ? Math.round((successCount / tasks.length) * 100) : 0,
            avgDuration: tasks.length > 0 ? formatDuration(Math.round(totalDuration / tasks.length)) : 'N/A',
            recentTasks: tasks.slice(0, 10),
            allTasks: tasks
        };
    }

    function drawVmBackupChart(canvas, vmData) {
        const ctx = canvas.getContext('2d');
        const width = canvas.width;
        const height = canvas.height;
        
        ctx.clearRect(0, 0, width, height);
        
        // Simple success rate bar chart
        const tasks = vmData.allTasks.slice(0, 20).reverse();
        const barWidth = width / tasks.length;
        
        tasks.forEach((task, idx) => {
            const barHeight = height * 0.8;
            const x = idx * barWidth;
            const y = height - barHeight;
            
            ctx.fillStyle = task.status === 'OK' ? '#10b981' : '#ef4444';
            ctx.fillRect(x + 1, y, barWidth - 2, barHeight);
        });
    }

    function formatTaskTime(task) {
        if (!task.startTime) return 'Unknown';
        return new Date(task.startTime * 1000).toLocaleString();
    }

    function getTaskStatusLabel(task) {
        if (!task.status) return 'Running';
        return task.status;
    }

    function clearSelection() {
        state.selectedVm = null;
        updateVmDetails();
    }

    // Public API
    return {
        init,
        updatePbsInfo,
        clearSelection
    };
})();