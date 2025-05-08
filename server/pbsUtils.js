/**
 * Processes a list of raw PBS tasks into structured summaries and recent task lists.
 * @param {Array} allTasks - Array of raw task objects from the PBS API.
 * @returns {Object} - Object containing structured task data (backupTasks, verificationTasks, etc.).
 */
function processPbsTasks(allTasks) {
    if (!allTasks) return { // Return default structure if tasks are null
        backupTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
        verificationTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
        syncTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
        pruneTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } }
    };

    const taskTypeMap = {
        backup: 'backup',
        verify: 'verify',
        verificationjob: 'verify',
        verify_group: 'verify',
        sync: 'sync',
        garbage_collection: 'pruneGc',
        prune: 'pruneGc'
    };

    const taskResults = categorizeAndCountTasks(allTasks, taskTypeMap);

    const createDetailedTask = (task) => ({
        upid: task.upid,
        node: task.node,
        type: task.worker_type || task.type,
        id: task.worker_id || task.id,
        status: task.status,
        startTime: task.starttime,
        endTime: task.endtime,
        duration: task.endtime && task.starttime ? task.endtime - task.starttime : null,
    });

    const sortTasksDesc = (a, b) => (b.startTime || 0) - (a.startTime || 0);
    
    const getRecentTasksList = (taskList, detailedTaskFn, sortFn, count = 20) => {
        if (!taskList) return []; // Guard against undefined taskList
        return taskList.map(detailedTaskFn).sort(sortFn).slice(0, count);
    };

    const recentBackupTasks = getRecentTasksList(taskResults.backup.list, createDetailedTask, sortTasksDesc);
    const recentVerifyTasks = getRecentTasksList(taskResults.verify.list, createDetailedTask, sortTasksDesc);
    const recentSyncTasks = getRecentTasksList(taskResults.sync.list, createDetailedTask, sortTasksDesc);
    const recentPruneGcTasks = getRecentTasksList(taskResults.pruneGc.list, createDetailedTask, sortTasksDesc);
    
    console.log(`INFO: [pbsUtils] Processed PBS Tasks - Backup: ${taskResults.backup.list.length}, Verify: ${taskResults.verify.list.length}, Sync: ${taskResults.sync.list.length}, Prune/GC: ${taskResults.pruneGc.list.length}`);

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

    if (!allTasks || !Array.isArray(allTasks)) { // Added check for allTasks being an array
        return results; // Return empty results if no tasks or invalid format
    }

    allTasks.forEach(task => {
        const taskType = task.worker_type || task.type;
        const categoryKey = taskTypeMap[taskType];

        if (categoryKey) {
            const category = results[categoryKey];
            category.list.push(task);

            const isOk = task.status === 'OK';
            // Consider 'WARNING' or other non-OK statuses as failed for summary purposes if needed
            const isFailed = task.status && task.status !== 'OK' && task.status !== 'running'; 

            if (isOk) {
                category.ok++;
                if (task.endtime > category.lastOk) category.lastOk = task.endtime;
            } else if (isFailed) {
                category.failed++;
                if (task.endtime > category.lastFailed) category.lastFailed = task.endtime;
            }
        }
    });
    return results;
}

module.exports = { processPbsTasks };
