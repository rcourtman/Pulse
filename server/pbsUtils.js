/**
 * Detects if a task failure is stale and should be hidden from the UI
 * @param {Object} task - PBS task object
 * @returns {boolean} - True if this is a stale task that should be filtered
 */
function isStaleTaskFailure(task) {
    const taskType = task.worker_type || task.type;
    const status = task.status || '';
    const endTime = task.endtime || task.endTime || 0;
    const taskId = task.id || '';
    const upid = task.upid || '';
    const guest = task.guest || '';
    
    // Handle verification tasks
    if (taskType === 'verificationjob' && status !== 'OK' && (status.includes('ERROR') || status.includes('verification failed'))) {
        // Special handling for the known orphaned verification job
        // Check in multiple fields where the job ID might appear
        if (taskId.includes('v-3fb332a6-ba43') || 
            upid.includes('v-3fb332a6-ba43') || 
            upid.includes('v\\x2d3fb332a6\\x2dba43') ||
            guest.includes('v-3fb332a6-ba43')) {
            return true;
        }
        
        // Consider verification failures older than 14 days as potentially stale
        const fourteenDaysAgo = (Date.now() / 1000) - (14 * 24 * 60 * 60);
        const isOldFailure = endTime && endTime < fourteenDaysAgo;
        
        // Check if the error message indicates missing/deleted snapshots
        const hasStaleErrorSignature = status.includes('verification failed') || 
                                       status.includes('backup not found') ||
                                       status.includes('group not found') ||
                                       status.includes('missing chunks');
        
        return isOldFailure && hasStaleErrorSignature;
    }
    
    // Handle old GC warnings - check multiple fields
    const isGCTask = taskType === 'garbage_collection' || 
                     (taskId && taskId.includes('GC main')) ||
                     (guest && guest.includes('GC main'));
                     
    if (isGCTask && (status.includes('WARNINGS') || status.includes('ERROR WARNINGS'))) {
        const mayStart = new Date('2025-05-01').getTime() / 1000;
        const mayEnd = new Date('2025-05-31').getTime() / 1000;
        const isFromMay2025 = endTime >= mayStart && endTime <= mayEnd;
        
        if (isFromMay2025) {
            return true; // Filter out May 2025 GC warnings
        }
        
        // Also hide GC warnings older than 30 days
        const thirtyDaysAgo = (Date.now() / 1000) - (30 * 24 * 60 * 60);
        return endTime && endTime < thirtyDaysAgo;
    }
    
    return false;
}

/**
 * Processes a list of raw PBS tasks into structured summaries and recent task lists.
 * @param {Array} allTasks - Array of raw task objects from the PBS API.
 * @returns {Object} - Object containing structured task data (backupTasks, verificationTasks, etc.).
 */
function processPbsTasks(allTasks) {
    // Ensure input is an array; return empty structure if not
    if (!Array.isArray(allTasks)) {
        console.warn('[PBS Utils] processPbsTasks received non-array input:', allTasks);
        return {
            backupTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0 } },
            verificationTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0 } },
            syncTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0 } },
            pruneTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0 } },
            aggregatedPbsTaskSummary: { total: 0, ok: 0, failed: 0 },
        };
    }

    const taskTypeMap = {
        backup: 'backup',
        verify: 'verify',
        verificationjob: 'verify',
        verify_group: 'verify',
        'verify-group': 'verify',
        verification: 'verify',
        sync: 'sync',
        syncjob: 'sync',
        'sync-job': 'sync',
        garbage_collection: 'pruneGc',
        'garbage-collection': 'pruneGc',
        prune: 'pruneGc',
        prunejob: 'pruneGc',
        'prune-job': 'pruneGc',
        gc: 'pruneGc'
    };

    const taskResults = categorizeAndCountTasks(allTasks, taskTypeMap);

    const unmappedTypes = new Set();
    allTasks.forEach(task => {
        const taskType = task.worker_type || task.type;
        if (!taskTypeMap[taskType]) {
            unmappedTypes.add(taskType);
        }
    });
    
    if (unmappedTypes.size > 0) {
    }

    // Helper function to extract namespace from task data
    const extractNamespaceFromTask = (task) => {
        // PRIORITY 1: If namespace is already set (from dataFetcher enhancement), use it
        if (task.namespace !== undefined && task.namespace !== null) {
            return task.namespace;
        }
        
        // PRIORITY 2: Extract from worker_id for raw PBS tasks
        const workerId = task.worker_id || task.id || '';
        
        // Try to extract namespace from worker_id patterns for regular PBS tasks
        // Pattern 1: Standard PBS format with ns= prefix
        // Examples: backup:ns=namespace:vm/101, verify:ns=namespace:ct/100
        const nsMatch = workerId.match(/ns=([^:]+)/);
        if (nsMatch && nsMatch[1]) {
            return nsMatch[1];
        }
        
        // Pattern 2: Alternative format where namespace is between datastore and type
        // Examples: main:namespace:vm/101, datastore1:namespace:ct/100
        // This handles cases where PBS uses datastore:namespace:type/id format
        const parts = workerId.split(':');
        if (parts.length >= 3) {
            const datastorePattern = /^[a-zA-Z0-9_-]+$/;
            const typePattern = /^(vm|ct)\/\d+$/;
            
            if (datastorePattern.test(parts[0]) && typePattern.test(parts[2])) {
                // The namespace is in the middle
                const namespace = parts[1];
                // Don't return empty string as namespace
                if (namespace && namespace !== '') {
                    return namespace;
                }
            }
        }
        
        // Default to 'root' namespace if no explicit namespace found
        return 'root';
    };

    const createDetailedTask = (task) => {
        const detailedTask = {
            upid: task.upid,
            node: task.node,
            type: task.worker_type || task.type,
            id: task.worker_id || task.id || task.guest, // Include guest as fallback for ID
            status: task.status,
            startTime: task.starttime,
            endTime: task.endtime,
            duration: task.endtime && task.starttime ? task.endtime - task.starttime : null,
            user: task.user,
            exitCode: task.exitcode,
            exitStatus: task.exitstatus,
            saved: task.saved || false,
            guest: task.guest || task.worker_id,
            pbsBackupRun: task.pbsBackupRun,
            // Add guest identification fields
            guestId: task.guestId,
            guestType: task.guestType,
            // Add namespace information
            namespace: extractNamespaceFromTask(task)
        };
        
        
        return detailedTask;
    };

    const sortTasksDesc = (a, b) => (b.startTime || 0) - (a.startTime || 0);
    
    const getRecentTasksList = (taskList, detailedTaskFn, sortFn, count = 500) => {
        if (!taskList) return [];
        const nowSec = Date.now() / 1000;
        const thirtyDays = 30 * 24 * 60 * 60;
        
        const recent = taskList.filter(task => {
            // Exclude stale tasks from UI display
            if (isStaleTaskFailure(task)) {
                return false;
            }
            
            // Include tasks without a starttime and tasks within last 30 days
            if (task.starttime == null) return true;
            return (nowSec - task.starttime) <= thirtyDays;
        });
        return recent.map(detailedTaskFn).sort(sortFn).slice(0, count);
    };

    const recentBackupTasks = getRecentTasksList(taskResults.backup.list, createDetailedTask, sortTasksDesc);
    const recentVerifyTasks = getRecentTasksList(taskResults.verify.list, createDetailedTask, sortTasksDesc);
    const recentSyncTasks = getRecentTasksList(taskResults.sync.list, createDetailedTask, sortTasksDesc);
    const recentPruneGcTasks = getRecentTasksList(taskResults.pruneGc.list, createDetailedTask, sortTasksDesc);
    
    // Helper function to create the summary object
    const createSummary = (category) => ({ 
        ok: category.ok,
        failed: category.failed,
        total: category.ok + category.failed,
        lastOk: category.lastOk || null, // Use null if 0
        lastFailed: category.lastFailed || null // Use null if 0
    });

    return {
        backupTasks: {
            recentTasks: recentBackupTasks,
            summary: createSummary(taskResults.backup)
        },
        verificationTasks: {
            recentTasks: recentVerifyTasks,
            summary: createSummary(taskResults.verify)
        },
        syncTasks: {
            recentTasks: recentSyncTasks,
            summary: createSummary(taskResults.sync)
        },
        pruneTasks: {
            recentTasks: recentPruneGcTasks,
            summary: createSummary(taskResults.pruneGc)
        }
    };
}

function categorizeAndCountTasks(allTasks, taskTypeMap) {
    const results = {
        backup: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 },
        verify: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 },
        sync: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 },
        pruneGc: { list: [], ok: 0, failed: 0, lastOk: 0, lastFailed: 0 }
    };

    if (!allTasks || !Array.isArray(allTasks)) {
        return results;
    }

    allTasks.forEach(task => {
        const taskType = task.worker_type || task.type;
        const categoryKey = taskTypeMap[taskType];

        if (categoryKey) {
            const category = results[categoryKey];
            category.list.push(task);

            // Skip counting stale tasks
            if (isStaleTaskFailure(task)) {
                return; // Skip counting this task in summary
            }

            // Fixed status handling - be more specific about what counts as failed
            const status = task.status || 'NO_STATUS';
            const isRunning = status.includes('running') || status.includes('queued');
            const isCompleted = status && !isRunning;

            if (isCompleted) {
                if (status === 'OK') {
                    category.ok++;
                    if (task.endtime && task.endtime > category.lastOk) category.lastOk = task.endtime;
                } else {
                    // Everything else that's not OK or running is considered failed
                    // This includes: WARNING, WARNINGS:..., errors, connection errors, etc.
                    category.failed++;
                    if (task.endtime && task.endtime > category.lastFailed) category.lastFailed = task.endtime;
                }
            }
        }
    });

    return results;
}

module.exports = { processPbsTasks, categorizeAndCountTasks, isStaleTaskFailure };
