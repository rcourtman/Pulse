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

    // Warn if unmapped task types found (useful for production troubleshooting)
    const unmappedTypes = new Set();
    allTasks.forEach(task => {
        const taskType = task.worker_type || task.type;
        if (!taskTypeMap[taskType]) {
            unmappedTypes.add(taskType);
        }
    });
    
    if (unmappedTypes.size > 0) {
        // console.warn('WARN: [pbsUtils] Unmapped PBS task types found:', Array.from(unmappedTypes));
    }

    const createDetailedTask = (task) => ({
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
        guestType: task.guestType
    });

    const sortTasksDesc = (a, b) => (b.startTime || 0) - (a.startTime || 0);
    
    const getRecentTasksList = (taskList, detailedTaskFn, sortFn, count = 20) => {
        if (!taskList) return [];
        const nowSec = Date.now() / 1000;
        const thirtyDays = 30 * 24 * 60 * 60;
        const recent = taskList.filter(task => {
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

            // Fixed status handling - be more specific about what counts as failed
            const status = task.status || 'NO_STATUS';
            const isRunning = status.includes('running') || status.includes('queued');
            const isCompleted = status && !isRunning;

            if (isCompleted) {
                if (status === 'OK') {
                    category.ok++;
                    if (task.endtime && task.endtime > category.lastOk) category.lastOk = task.endtime;
                } else {
                    // Everything that's not OK or running is considered failed
                    // This includes: WARNING, WARNINGS:..., errors, connection errors, etc.
                    category.failed++;
                    if (task.endtime && task.endtime > category.lastFailed) category.lastFailed = task.endtime;
                }
            }
        }
    });

    return results;
}

module.exports = { processPbsTasks, categorizeAndCountTasks };
