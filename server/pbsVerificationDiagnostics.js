
/**
 * Diagnostic function to check a specific PBS verification job
 * @param {Object} pbsClient - The PBS API client instance
 * @param {string} datastore - The datastore name (e.g., 'main')
 * @param {string} jobId - The verification job ID (e.g., 'v-3fb332a6-ba43')
 * @returns {Promise<Object>} Diagnostic information about the verification job
 */
async function checkVerificationJob(pbsClient, datastore, jobId) {
    const diagnostics = {
        jobId,
        datastore,
        jobExists: false,
        jobDetails: null,
        relatedSnapshots: [],
        errors: []
    };

    try {
        // Check if the verification job exists
        const verifyJobsResponse = await pbsClient.get(`/config/datastore/${datastore}/verify`);
        const verifyJobs = verifyJobsResponse.data || [];
        
        const job = verifyJobs.find(j => j.id === jobId);
        if (job) {
            diagnostics.jobExists = true;
            diagnostics.jobDetails = job;
            console.log(`[PBS Diagnostics] Found verification job ${jobId}:`, job);
        } else {
            diagnostics.errors.push(`Verification job ${jobId} not found in datastore ${datastore}`);
            console.log(`[PBS Diagnostics] Verification job ${jobId} not found. Available jobs:`, verifyJobs.map(j => j.id));
        }

        // Check for snapshots that might be related to this verification job
        // The job ID pattern v-3fb332a6-ba43 suggests it might be related to a specific snapshot or backup group
        try {
            const snapshotsResponse = await pbsClient.get(`/admin/datastore/${datastore}/snapshots`);
            const snapshots = snapshotsResponse.data || [];
            
            // Look for snapshots that might match the job pattern
            const relatedSnapshots = snapshots.filter(snap => {
                // Check if snapshot name or group contains parts of the job ID
                const jobIdPart = jobId.replace('v-', '');
                return snap.backup_id?.includes(jobIdPart) || 
                       snap.backup_type?.includes(jobIdPart) ||
                       snap.comment?.includes(jobIdPart);
            });
            
            diagnostics.relatedSnapshots = relatedSnapshots;
            if (relatedSnapshots.length > 0) {
                console.log(`[PBS Diagnostics] Found ${relatedSnapshots.length} potentially related snapshots`);
            }
        } catch (snapError) {
            diagnostics.errors.push(`Failed to check snapshots: ${snapError.message}`);
        }

        // Get recent task logs for this verification job
        try {
            const tasksResponse = await pbsClient.get('/nodes/localhost/tasks', {
                params: {
                    typefilter: 'verificationjob',
                    limit: 100
                }
            });
            const tasks = tasksResponse.data || [];
            
            // Filter tasks related to this job
            const jobTasks = tasks.filter(task => {
                return task.upid?.includes(jobId) || task.worker_id?.includes(jobId);
            });
            
            if (jobTasks.length > 0) {
                diagnostics.recentTasks = jobTasks.slice(0, 5); // Last 5 tasks
                console.log(`[PBS Diagnostics] Found ${jobTasks.length} tasks for job ${jobId}`);
                
                // Check the most recent failure
                const failedTask = jobTasks.find(t => t.status && t.status !== 'OK');
                if (failedTask) {
                    // Try to get the task log
                    try {
                        const logResponse = await pbsClient.get(`/nodes/localhost/tasks/${failedTask.upid}/log`);
                        diagnostics.lastFailureLog = logResponse.data;
                        console.log(`[PBS Diagnostics] Last failure log retrieved for task ${failedTask.upid}`);
                    } catch (logError) {
                        diagnostics.errors.push(`Failed to get task log: ${logError.message}`);
                    }
                }
            }
        } catch (taskError) {
            diagnostics.errors.push(`Failed to check tasks: ${taskError.message}`);
        }

    } catch (error) {
        diagnostics.errors.push(`General error: ${error.message}`);
        console.error(`[PBS Diagnostics] Error checking verification job:`, error);
    }

    return diagnostics;
}

/**
 * Extract the job ID from a verification task data
 * @param {Object} task - The verification task object
 * @returns {Object} Object with datastore and jobId
 */
function extractVerificationJobInfo(task) {
    // Try to extract from comment first (e.g., "main:v-3fb332a6-ba43")
    if (task.comment && task.comment.includes(':')) {
        const parts = task.comment.split(':');
        if (parts.length >= 2) {
            return {
                datastore: parts[0],
                jobId: parts.slice(1).join(':')
            };
        }
    }
    
    // Try to extract from id field
    if (task.id && task.id.includes(':')) {
        const parts = task.id.split(':');
        if (parts.length >= 2) {
            return {
                datastore: parts[0],
                jobId: parts.slice(1).join(':')
            };
        }
    }
    
    // Try to extract from UPID
    if (task.upid) {
        // UPID format: UPID:node:pid:pstart:starttime:type:id:user:
        // For verification job: UPID:pbs:00001234:0056789A:66A0B123:verificationjob:main\x3av-3fb332a6-ba43:root@pam:
        const parts = task.upid.split(':');
        if (parts.length >= 7) {
            const jobIdPart = parts[6]; // The job ID part
            if (jobIdPart.includes('\\x3a')) {
                // Decode hex-encoded colon
                const decoded = jobIdPart.replace(/\\x3a/g, ':');
                const jobParts = decoded.split(':');
                if (jobParts.length >= 2) {
                    return {
                        datastore: jobParts[0],
                        jobId: jobParts[1]
                    };
                }
            }
        }
    }
    
    return null;
}

/**
 * Run diagnostics on the specific failing verification job
 * @param {Object} pbsInstance - The PBS instance data
 * @param {Array} failingTasks - Array of failing verification tasks
 * @returns {Promise<Object>} Diagnostic results
 */
async function runVerificationDiagnostics(pbsInstance, failingTasks = []) {
    const results = {};
    
    // Process each failing task
    for (const task of failingTasks) {
        const jobInfo = extractVerificationJobInfo(task);
        if (jobInfo) {
            const key = `${jobInfo.datastore}:${jobInfo.jobId}`;
            if (!results[key]) {
                console.log(`[PBS Diagnostics] Checking verification job ${key} from task:`, task);
                results[key] = await checkVerificationJob(pbsInstance.api, jobInfo.datastore, jobInfo.jobId);
            }
        }
    }
    
    // Also check the hardcoded job if no failing tasks were provided
    if (failingTasks.length === 0) {
        const jobPattern = 'main:v-3fb332a6-ba43';
        const [datastore, jobId] = jobPattern.split(':');
        
        if (pbsInstance.api) {
            results[jobPattern] = await checkVerificationJob(pbsInstance.api, datastore, jobId);
        }
    }
    
    return results;
}

module.exports = {
    runVerificationDiagnostics
};