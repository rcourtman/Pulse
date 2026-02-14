/**
 * Container Update Store
 * 
 * Tracks the state of container updates across WebSocket refreshes.
 * This ensures the UI shows consistent "updating" state until the update completes.
 */
import { createSignal } from 'solid-js';
import type { DockerHostCommand } from '@/types/api';

export type ContainerUpdateState = 'queued' | 'updating' | 'success' | 'error';

interface UpdateEntry {
    state: ContainerUpdateState;
    startedAt: number;
    message?: string;
    commandId?: string;
    // Real-time progress from backend
    backendStatus?: string;
    acknowledgedAt?: number;
}

// Global store for container update states
// Key format: "hostId:containerId"
const [updateStates, setUpdateStates] = createSignal<Record<string, UpdateEntry>>({});
const AUTO_CLEAR_SUCCESS_MS = 5000;
const AUTO_CLEAR_ERROR_MS = 10000;
const STALE_CLEANUP_INTERVAL_MS = 60000;
const STALE_THRESHOLD_MS = 5 * 60 * 1000;

const pendingClearTimers = new Map<string, ReturnType<typeof setTimeout>>();
let staleCleanupInterval: ReturnType<typeof setInterval> | null = null;
let unloadHandlerRegistered = false;

function clearPendingAutoClearTimer(key: string): void {
    const timer = pendingClearTimers.get(key);
    if (timer) {
        clearTimeout(timer);
        pendingClearTimers.delete(key);
    }
}

function scheduleAutoClear(hostId: string, containerId: string, delayMs: number): void {
    const key = `${hostId}:${containerId}`;
    clearPendingAutoClearTimer(key);

    const timer = setTimeout(() => {
        pendingClearTimers.delete(key);
        clearContainerUpdateState(hostId, containerId);
    }, delayMs);

    pendingClearTimers.set(key, timer);
}

export function startContainerUpdateCleanup(): void {
    if (staleCleanupInterval !== null) return;
    staleCleanupInterval = setInterval(cleanupStaleUpdates, STALE_CLEANUP_INTERVAL_MS);
}

export function stopContainerUpdateCleanup(options?: { clearStates?: boolean }): void {
    if (staleCleanupInterval !== null) {
        clearInterval(staleCleanupInterval);
        staleCleanupInterval = null;
    }

    pendingClearTimers.forEach((timer) => clearTimeout(timer));
    pendingClearTimers.clear();

    if (options?.clearStates) {
        setUpdateStates({});
    }
}

/**
 * Get the update state for a specific container
 */
export function getContainerUpdateState(hostId: string, containerId: string): UpdateEntry | undefined {
    const key = `${hostId}:${containerId}`;
    return updateStates()[key];
}

/**
 * Mark a container as updating
 */
export function markContainerUpdating(hostId: string, containerId: string, commandId?: string): void {
    const key = `${hostId}:${containerId}`;
    clearPendingAutoClearTimer(key);
    setUpdateStates(prev => ({
        ...prev,
        [key]: {
            state: 'updating',
            startedAt: Date.now(),
            commandId,
        }
    }));
}

/**
 * Mark a container update as queued (command sent, waiting for agent)
 */
export function markContainerQueued(hostId: string, containerId: string, commandId?: string): void {
    const key = `${hostId}:${containerId}`;
    clearPendingAutoClearTimer(key);
    setUpdateStates(prev => ({
        ...prev,
        [key]: {
            state: 'queued',
            startedAt: Date.now(),
            commandId,
        }
    }));
}

/**
 * Sync container update state with backend command status from WebSocket.
 * This provides real-time progress tracking.
 */
export function syncWithHostCommand(hostId: string, command: DockerHostCommand | undefined): void {
    if (!command) return;

    // Only update if we have a matching entry or if the command is for update_container
    if (command.type !== 'update_container') return;

    const containerId = command.id.split(':')[1] || ''; // Extract containerId if encoded in commandID
    const key = `${hostId}:${containerId}`;

    // Check if we're tracking this update
    const existing = updateStates()[key];
    if (!existing) return;

    // Update based on backend status
    if (command.status === 'completed' || command.completedAt) {
        markContainerUpdateSuccess(hostId, containerId);
    } else if (command.status === 'failed' || command.failedAt) {
        markContainerUpdateError(hostId, containerId, command.failureReason || command.message);
    } else if (command.status === 'in_progress' || command.acknowledgedAt) {
        // Agent is actively working on the update - show the current step
        clearPendingAutoClearTimer(key);
        setUpdateStates(prev => ({
            ...prev,
            [key]: {
                ...prev[key],
                state: 'updating',
                backendStatus: command.status,
                acknowledgedAt: command.acknowledgedAt,
                message: command.message, // This contains the current step (e.g., "Pulling image...")
            }
        }));
    }
}

/**
 * Mark a container update as successful
 */
export function markContainerUpdateSuccess(hostId: string, containerId: string): void {
    const key = `${hostId}:${containerId}`;
    clearPendingAutoClearTimer(key);
    setUpdateStates(prev => ({
        ...prev,
        [key]: {
            state: 'success',
            startedAt: prev[key]?.startedAt || Date.now(),
        }
    }));

    // Auto-clear success state after 5 seconds
    scheduleAutoClear(hostId, containerId, AUTO_CLEAR_SUCCESS_MS);
}

/**
 * Mark a container update as failed
 */
export function markContainerUpdateError(hostId: string, containerId: string, message?: string): void {
    const key = `${hostId}:${containerId}`;
    clearPendingAutoClearTimer(key);
    setUpdateStates(prev => ({
        ...prev,
        [key]: {
            state: 'error',
            startedAt: prev[key]?.startedAt || Date.now(),
            message,
        }
    }));

    // Auto-clear error state after 10 seconds
    scheduleAutoClear(hostId, containerId, AUTO_CLEAR_ERROR_MS);
}

/**
 * Clear the update state for a container
 */
export function clearContainerUpdateState(hostId: string, containerId: string): void {
    const key = `${hostId}:${containerId}`;
    clearPendingAutoClearTimer(key);
    setUpdateStates(prev => {
        const next = { ...prev };
        delete next[key];
        return next;
    });
}

/**
 * Check if update state is stale (older than 5 minutes) and should be auto-cleared
 */
export function cleanupStaleUpdates(): void {
    const now = Date.now();

    setUpdateStates(prev => {
        const next: Record<string, UpdateEntry> = {};
        for (const [key, entry] of Object.entries(prev)) {
            if (now - entry.startedAt < STALE_THRESHOLD_MS) {
                next[key] = entry;
            } else {
                clearPendingAutoClearTimer(key);
            }
        }
        return next;
    });
}

/**
 * Get all current update states (useful for debugging)
 */
export function getAllUpdateStates(): Record<string, UpdateEntry> {
    return updateStates();
}

/**
 * Reactive accessor for all update states
 */
export { updateStates };

// Start cleanup lifecycle immediately so stale states are pruned in long-lived sessions.
startContainerUpdateCleanup();

if (typeof window !== 'undefined' && !unloadHandlerRegistered) {
    window.addEventListener('beforeunload', () => {
        stopContainerUpdateCleanup();
    }, { once: true });
    unloadHandlerRegistered = true;
}
