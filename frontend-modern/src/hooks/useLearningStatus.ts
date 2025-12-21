/**
 * useLearningStatus - Hook for fetching AI learning/baseline status
 * 
 * This shows users how much the AI has learned about their infrastructure,
 * providing transparency into the baseline learning process.
 * 
 * FREE feature - no license required.
 */

import { createResource, onCleanup } from 'solid-js';
import { AIAPI } from '@/api/ai';
import type { LearningStatusResponse } from '@/types/aiIntelligence';

// Default empty state
const emptyLearningStatus: LearningStatusResponse = {
    resources_baselined: 0,
    total_metrics: 0,
    metric_breakdown: {},
    status: 'waiting',
    message: 'Loading learning status...',
    license_required: false,
};

/**
 * Hook to get the current learning/baseline status
 * Polls every 60 seconds (learning status changes slowly)
 */
export function useLearningStatus() {
    const [learningStatus, { refetch }] = createResource<LearningStatusResponse>(
        async () => {
            try {
                return await AIAPI.getLearningStatus();
            } catch {
                return emptyLearningStatus;
            }
        },
        { initialValue: emptyLearningStatus }
    );

    // Poll every 60 seconds (learning progress changes slowly)
    const intervalId = setInterval(() => refetch(), 60000);
    onCleanup(() => clearInterval(intervalId));

    return {
        status: learningStatus,
        refetch,
        // Convenience accessors
        resourceCount: () => learningStatus()?.resources_baselined ?? 0,
        metricCount: () => learningStatus()?.total_metrics ?? 0,
        learningState: () => learningStatus()?.status ?? 'waiting',
        isActive: () => learningStatus()?.status === 'active',
        isLearning: () => learningStatus()?.status === 'learning',
        isWaiting: () => learningStatus()?.status === 'waiting',
    };
}

export default useLearningStatus;
