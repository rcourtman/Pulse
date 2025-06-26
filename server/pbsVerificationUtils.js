/**
 * PBS Verification Utilities
 * 
 * Utilities for monitoring and managing PBS verification jobs and detecting
 * verification failures due to orphaned snapshots or configuration issues.
 */

/**
 * Analyzes PBS verification health by examining snapshot verification status
 * @param {Object} pbsClient - PBS API client instance
 * @param {string} datastoreName - Name of the datastore to analyze
 * @returns {Promise<Object>} - Verification health analysis
 */
async function analyzeVerificationHealth(pbsClient, datastoreName) {
    try {
        const { client } = pbsClient;
        
        // Get all snapshots from root namespace
        const namespacesToQuery = ['']; // Start with root namespace
        let allSnapshots = [];
        
        for (const namespace of namespacesToQuery) {
            try {
                const params = namespace ? { ns: namespace } : { ns: '' };
                const snapshotsResponse = await client.get(`/admin/datastore/${datastoreName}/snapshots`, { params });
                const snapshots = snapshotsResponse.data?.data || [];
                
                // Add namespace info to each snapshot
                snapshots.forEach(snap => {
                    snap.namespace = namespace || 'root';
                });
                
                allSnapshots.push(...snapshots);
            } catch (nsError) {
                if (nsError.response?.status !== 404) {
                    console.warn(`WARN: [PBS Verification] Failed to fetch snapshots from namespace '${namespace}': ${nsError.message}`);
                }
            }
        }
        
        // Analyze verification status
        const verificationAnalysis = {
            totalSnapshots: allSnapshots.length,
            verifiedSnapshots: 0,
            failedVerifications: 0,
            unverifiedSnapshots: 0,
            verificationJobs: new Set(),
            recentFailures: [],
            healthScore: 'unknown'
        };
        
        // Analyze recent snapshots (last 7 days) for verification health
        const sevenDaysAgo = Math.floor((Date.now() - 7 * 24 * 60 * 60 * 1000) / 1000);
        
        allSnapshots.forEach(snapshot => {
            if (snapshot.verification) {
                verificationAnalysis.verifiedSnapshots++;
                
                // Extract verification job ID from UPID
                if (snapshot.verification.upid) {
                    const upidParts = snapshot.verification.upid.split(':');
                    if (upidParts.length > 7) {
                        // UPID format: UPID:node:pid:pstart:starttime:hex:type:id:user:endtime
                        // For verification jobs: type is 'verificationjob' and id contains 'datastore:jobid'
                        const taskType = upidParts[6];
                        const encodedJobInfo = upidParts[7]; 
                        
                        if (taskType === 'verificationjob' && encodedJobInfo) {
                            const decodedInfo = encodedJobInfo.replace(/\\x([0-9a-fA-F]{2})/g, (match, hex) => 
                                String.fromCharCode(parseInt(hex, 16))
                            );
                            
                            // Extract job ID after the datastore name and colon
                            const jobIdMatch = decodedInfo.match(/:([^:]+)$/);
                            if (jobIdMatch && jobIdMatch[1]) {
                                verificationAnalysis.verificationJobs.add(jobIdMatch[1]);
                            }
                        }
                    }
                }
                
                // Check for failed verifications
                if (snapshot.verification.state !== 'ok') {
                    verificationAnalysis.failedVerifications++;
                    
                    // If this is a recent failure, add to recent failures list
                    if (snapshot['backup-time'] >= sevenDaysAgo) {
                        verificationAnalysis.recentFailures.push({
                            backupType: snapshot['backup-type'],
                            backupId: snapshot['backup-id'],
                            backupTime: snapshot['backup-time'],
                            verificationState: snapshot.verification.state,
                            namespace: snapshot.namespace
                        });
                    }
                }
            } else {
                verificationAnalysis.unverifiedSnapshots++;
            }
        });
        
        // Calculate health score
        const verificationRate = verificationAnalysis.totalSnapshots > 0 
            ? verificationAnalysis.verifiedSnapshots / verificationAnalysis.totalSnapshots 
            : 0;
        
        const failureRate = verificationAnalysis.verifiedSnapshots > 0 
            ? verificationAnalysis.failedVerifications / verificationAnalysis.verifiedSnapshots 
            : 0;
        
        if (verificationRate >= 0.95 && failureRate <= 0.01) {
            verificationAnalysis.healthScore = 'excellent';
        } else if (verificationRate >= 0.8 && failureRate <= 0.05) {
            verificationAnalysis.healthScore = 'good';
        } else if (verificationRate >= 0.6 && failureRate <= 0.1) {
            verificationAnalysis.healthScore = 'fair';
        } else {
            verificationAnalysis.healthScore = 'poor';
        }
        
        verificationAnalysis.verificationJobs = Array.from(verificationAnalysis.verificationJobs);
        
        return verificationAnalysis;
        
    } catch (error) {
        console.error(`ERROR: [PBS Verification] Failed to analyze verification health for ${datastoreName}: ${error.message}`);
        return {
            totalSnapshots: 0,
            verifiedSnapshots: 0,
            failedVerifications: 0,
            unverifiedSnapshots: 0,
            verificationJobs: [],
            recentFailures: [],
            healthScore: 'error',
            error: error.message
        };
    }
}

/**
 * Checks if verification job configuration exists and is valid
 * @param {Object} pbsClient - PBS API client instance
 * @param {string} jobId - Verification job ID to check
 * @returns {Promise<Object>} - Job status information
 */
async function checkVerificationJobStatus(pbsClient, jobId) {
    try {
        const { client } = pbsClient;
        
        // Get verification job configuration
        const jobResponse = await client.get(`/config/verify/${encodeURIComponent(jobId)}`);
        const jobConfig = jobResponse.data?.data;
        
        if (!jobConfig) {
            return {
                exists: false,
                error: 'Job configuration not found'
            };
        }
        
        return {
            exists: true,
            config: jobConfig,
            enabled: !jobConfig.disable,
            schedule: jobConfig.schedule || 'manual',
            datastore: jobConfig.store,
            ignoreVerified: jobConfig['ignore-verified'] || false,
            outdatedAfter: jobConfig['outdated-after'] || null
        };
        
    } catch (error) {
        if (error.response?.status === 404) {
            return {
                exists: false,
                error: 'Verification job not found'
            };
        }
        return {
            exists: false,
            error: error.message
        };
    }
}

/**
 * Gets list of all verification jobs with their basic status
 * @param {Object} pbsClient - PBS API client instance
 * @returns {Promise<Array>} - Array of verification job info
 */
async function getVerificationJobs(pbsClient) {
    try {
        const { client } = pbsClient;
        
        const response = await client.get('/config/verify');
        const jobs = response.data?.data || [];
        
        return jobs.map(job => ({
            id: job.id,
            datastore: job.store,
            schedule: job.schedule || 'manual',
            enabled: !job.disable,
            comment: job.comment || null
        }));
        
    } catch (error) {
        console.error(`ERROR: [PBS Verification] Failed to get verification jobs: ${error.message}`);
        return [];
    }
}

/**
 * Detects potential stale verification references in snapshots
 * @param {Object} pbsClient - PBS API client instance
 * @param {string} datastoreName - Name of the datastore to check
 * @returns {Promise<Object>} - Analysis of stale verification references
 */
async function detectStaleVerificationReferences(pbsClient, datastoreName) {
    try {
        // Get current verification jobs
        const currentJobs = await getVerificationJobs(pbsClient);
        const currentJobIds = new Set(currentJobs.map(job => job.id));
        
        const healthAnalysis = await analyzeVerificationHealth(pbsClient, datastoreName);
        
        // Find verification job references in snapshots that don't have current job configs
        const staleJobReferences = healthAnalysis.verificationJobs.filter(jobId => 
            !currentJobIds.has(jobId)
        );
        
        return {
            currentJobs: currentJobs.length,
            jobReferencesInSnapshots: healthAnalysis.verificationJobs.length,
            staleJobReferences: staleJobReferences,
            hasStaleReferences: staleJobReferences.length > 0,
            healthAnalysis: healthAnalysis
        };
        
    } catch (error) {
        console.error(`ERROR: [PBS Verification] Failed to detect stale verification references: ${error.message}`);
        return {
            currentJobs: 0,
            jobReferencesInSnapshots: 0,
            staleJobReferences: [],
            hasStaleReferences: false,
            error: error.message
        };
    }
}

/**
 * Provides verification job management recommendations based on analysis
 * @param {Object} pbsClient - PBS API client instance
 * @param {string} datastoreName - Name of the datastore
 * @returns {Promise<Object>} - Recommendations for verification management
 */
async function getVerificationRecommendations(pbsClient, datastoreName) {
    try {
        const healthAnalysis = await analyzeVerificationHealth(pbsClient, datastoreName);
        const staleAnalysis = await detectStaleVerificationReferences(pbsClient, datastoreName);
        const currentJobs = await getVerificationJobs(pbsClient);
        
        const recommendations = {
            priority: 'low',
            actions: [],
            insights: [],
            healthScore: healthAnalysis.healthScore
        };
        
        // High priority issues
        if (healthAnalysis.healthScore === 'poor') {
            recommendations.priority = 'high';
            recommendations.actions.push('Investigate verification failures immediately');
            recommendations.actions.push('Check PBS datastore health and available space');
        }
        
        if (healthAnalysis.recentFailures.length > 0) {
            recommendations.priority = 'medium';
            recommendations.actions.push(`Address ${healthAnalysis.recentFailures.length} recent verification failures`);
        }
        
        // Stale references
        if (staleAnalysis.hasStaleReferences) {
            recommendations.insights.push(`Found ${staleAnalysis.staleJobReferences.length} verification job references in snapshots for jobs that no longer exist`);
            recommendations.insights.push('This is normal after verification jobs are deleted - references will disappear as old snapshots are pruned');
        }
        
        // No verification jobs
        if (currentJobs.length === 0) {
            recommendations.priority = 'medium';
            recommendations.actions.push('Consider creating verification jobs to ensure backup integrity');
        }
        
        // Disabled jobs
        const disabledJobs = currentJobs.filter(job => !job.enabled);
        if (disabledJobs.length > 0) {
            recommendations.insights.push(`${disabledJobs.length} verification jobs are disabled`);
        }
        
        // General insights
        if (healthAnalysis.unverifiedSnapshots > 0) {
            const unverifiedPercent = ((healthAnalysis.unverifiedSnapshots / healthAnalysis.totalSnapshots) * 100).toFixed(1);
            recommendations.insights.push(`${unverifiedPercent}% of snapshots (${healthAnalysis.unverifiedSnapshots}) have no verification data`);
        }
        
        if (recommendations.actions.length === 0 && recommendations.priority === 'low') {
            recommendations.insights.push('Verification system is operating normally');
        }
        
        return recommendations;
        
    } catch (error) {
        return {
            priority: 'high',
            actions: ['Failed to analyze verification system'],
            insights: [`Error: ${error.message}`],
            healthScore: 'error'
        };
    }
}

module.exports = {
    analyzeVerificationHealth,
    checkVerificationJobStatus,
    getVerificationJobs,
    detectStaleVerificationReferences,
    getVerificationRecommendations
};