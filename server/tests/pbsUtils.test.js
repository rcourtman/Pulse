const { processPbsTasks } = require('../pbsUtils');

describe('PBS Utils - processPbsTasks', () => {

    test('should return default structure for null input', () => {
        const result = processPbsTasks(null);
        expect(result).toEqual({
            backupTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
            verificationTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
            syncTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
            pruneTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } }
        });
    });

    test('should return default structure for empty array input', () => {
        const result = processPbsTasks([]);
        expect(result).toEqual({
            backupTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
            verificationTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
            syncTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } },
            pruneTasks: { recentTasks: [], summary: { ok: 0, failed: 0, total: 0, lastOk: null, lastFailed: null } }
        });
    });

    test('should correctly categorize and summarize various task types', () => {
        const now = Math.floor(Date.now() / 1000);
        const tasks = [
            // Backups
            { upid: 'B1', worker_type: 'backup', status: 'OK', starttime: now - 3600, endtime: now - 3500 },
            { upid: 'B2', type: 'backup', status: 'OK', starttime: now - 7200, endtime: now - 7100 },
            { upid: 'B3', worker_type: 'backup', status: 'FAILED', starttime: now - 100, endtime: now - 50 },
            // Verifications
            { upid: 'V1', worker_type: 'verify', status: 'OK', starttime: now - 500, endtime: now - 400 },
            { upid: 'V2', type: 'verificationjob', status: 'WARNING', starttime: now - 600, endtime: now - 550 }, // Treated as failed
            // Sync
            { upid: 'S1', worker_type: 'sync', status: 'OK', starttime: now - 1000, endtime: now - 900 },
            // Prune/GC
            { upid: 'P1', worker_type: 'prune', status: 'OK', starttime: now - 2000, endtime: now - 1900 },
            { upid: 'G1', type: 'garbage_collection', status: 'OK', starttime: now - 2100, endtime: now - 2050 },
            // Unknown/Other
            { upid: 'U1', type: 'unknown', status: 'OK', starttime: now - 5000, endtime: now - 4900 },
            // Running task (should not count as OK or Failed)
            { upid: 'R1', worker_type: 'backup', status: 'running', starttime: now - 10, endtime: null },
        ];

        const result = processPbsTasks(tasks);

        // Backup Summary
        expect(result.backupTasks.summary.ok).toBe(2);
        expect(result.backupTasks.summary.failed).toBe(1);
        expect(result.backupTasks.summary.total).toBe(3);
        expect(result.backupTasks.summary.lastOk).toBe(now - 3500);
        expect(result.backupTasks.summary.lastFailed).toBe(now - 50);
        expect(result.backupTasks.recentTasks).toHaveLength(4); // B3, B1, R1, B2 (all backup tasks included)

        // Verification Summary
        expect(result.verificationTasks.summary.ok).toBe(1);
        expect(result.verificationTasks.summary.failed).toBe(1);
        expect(result.verificationTasks.summary.total).toBe(2);
        expect(result.verificationTasks.summary.lastOk).toBe(now - 400);
        expect(result.verificationTasks.summary.lastFailed).toBe(now - 550);
        expect(result.verificationTasks.recentTasks).toHaveLength(2);

        // Sync Summary
        expect(result.syncTasks.summary.ok).toBe(1);
        expect(result.syncTasks.summary.failed).toBe(0);
        expect(result.syncTasks.summary.total).toBe(1);
        expect(result.syncTasks.summary.lastOk).toBe(now - 900);
        expect(result.syncTasks.summary.lastFailed).toBeNull();
        expect(result.syncTasks.recentTasks).toHaveLength(1);

        // Prune/GC Summary
        expect(result.pruneTasks.summary.ok).toBe(2);
        expect(result.pruneTasks.summary.failed).toBe(0);
        expect(result.pruneTasks.summary.total).toBe(2);
        expect(result.pruneTasks.summary.lastOk).toBe(now - 1900); // P1 is later than G1
        expect(result.pruneTasks.summary.lastFailed).toBeNull();
        expect(result.pruneTasks.recentTasks).toHaveLength(2);
    });

    test('should correctly format recent tasks', () => {
        const now = Math.floor(Date.now() / 1000);
        const tasks = [
            { upid: 'B1', node: 'pbsnode', worker_type: 'backup', worker_id: 'vm/100', status: 'OK', starttime: now - 100, endtime: now - 50 },
            { upid: 'B2', node: 'pbsnode', type: 'backup', id: 'ct/101', status: 'FAILED', starttime: now - 200, endtime: now - 150 },
            { upid: 'B3', node: 'pbsnode', worker_type: 'backup', worker_id: 'vm/102', status: 'running', starttime: now - 10, endtime: null }, // Running task
        ];
        const result = processPbsTasks(tasks);
        const recent = result.backupTasks.recentTasks;

        expect(recent).toHaveLength(3);
        // Check sorting (most recent first)
        expect(recent[0].upid).toBe('B3');
        expect(recent[1].upid).toBe('B1');
        expect(recent[2].upid).toBe('B2');

        // Check formatting of one task
        expect(recent[1]).toEqual({
            upid: 'B1',
            node: 'pbsnode',
            type: 'backup',
            id: 'vm/100',
            status: 'OK',
            startTime: now - 100,
            endTime: now - 50,
            duration: 50,
        });

        // Check running task formatting (null duration)
        expect(recent[0]).toEqual({
            upid: 'B3',
            node: 'pbsnode',
            type: 'backup',
            id: 'vm/102',
            status: 'running',
            startTime: now - 10,
            endTime: null,
            duration: null,
        });
    });

    test('should limit recent tasks to 20 by default', () => {
        const now = Math.floor(Date.now() / 1000);
        const tasks = [];
        for (let i = 0; i < 25; i++) {
            tasks.push({ upid: `B${i}`, worker_type: 'backup', status: 'OK', starttime: now - (i * 100), endtime: now - (i * 100) + 50 });
        }
        const result = processPbsTasks(tasks);
        expect(result.backupTasks.recentTasks).toHaveLength(20);
        expect(result.backupTasks.recentTasks[0].upid).toBe('B0'); // Most recent
        expect(result.backupTasks.recentTasks[19].upid).toBe('B19'); // 20th most recent
    });

    test('should handle tasks with missing start or end times gracefully', () => {
        const now = Math.floor(Date.now() / 1000);
        const tasks = [
            { upid: 'B1', worker_type: 'backup', status: 'OK', starttime: now - 100, endtime: now - 50 },
            { upid: 'B2', worker_type: 'backup', status: 'OK', starttime: null, endtime: now - 150 }, // Missing starttime
            { upid: 'B3', worker_type: 'backup', status: 'OK', starttime: now - 200, endtime: undefined }, // Missing endtime
            { upid: 'B4', worker_type: 'backup', status: 'OK', starttime: null, endtime: null }, // Missing both
        ];
        const result = processPbsTasks(tasks);
        const recent = result.backupTasks.recentTasks;

        expect(recent).toHaveLength(4);
        // Sorting might be affected, but check formatting
        const taskB2 = recent.find(t => t.upid === 'B2');
        const taskB3 = recent.find(t => t.upid === 'B3');
        const taskB4 = recent.find(t => t.upid === 'B4');

        expect(taskB2.duration).toBeNull();
        expect(taskB3.duration).toBeNull();
        expect(taskB4.duration).toBeNull();

        // Check summary timestamps (should ignore tasks without endtime)
        expect(result.backupTasks.summary.lastOk).toBe(now - 50); // Only B1 has a valid endtime
    });

    test('should handle different verification task types', () => {
        const now = Math.floor(Date.now() / 1000);
        const tasks = [
            { upid: 'V1', worker_type: 'verify', status: 'OK', starttime: now - 100, endtime: now - 50 },
            { upid: 'V2', type: 'verificationjob', status: 'OK', starttime: now - 200, endtime: now - 150 },
            { upid: 'V3', type: 'verify_group', status: 'FAILED', starttime: now - 300, endtime: now - 250 },
        ];
        const result = processPbsTasks(tasks);

        expect(result.verificationTasks.summary.ok).toBe(2);
        expect(result.verificationTasks.summary.failed).toBe(1);
        expect(result.verificationTasks.summary.total).toBe(3);
        expect(result.verificationTasks.recentTasks).toHaveLength(3);
        expect(result.verificationTasks.recentTasks.map(t => t.upid)).toEqual(['V1', 'V2', 'V3']); // Sorted by start time
    });

     test('should handle different prune/gc task types', () => {
        const now = Math.floor(Date.now() / 1000);
        const tasks = [
            { upid: 'P1', worker_type: 'prune', status: 'OK', starttime: now - 100, endtime: now - 50 },
            { upid: 'G1', type: 'garbage_collection', status: 'FAILED', starttime: now - 200, endtime: now - 150 },
        ];
        const result = processPbsTasks(tasks);

        expect(result.pruneTasks.summary.ok).toBe(1);
        expect(result.pruneTasks.summary.failed).toBe(1);
        expect(result.pruneTasks.summary.total).toBe(2);
        expect(result.pruneTasks.recentTasks).toHaveLength(2);
        expect(result.pruneTasks.recentTasks.map(t => t.upid)).toEqual(['P1', 'G1']); // Sorted by start time
    });

});
