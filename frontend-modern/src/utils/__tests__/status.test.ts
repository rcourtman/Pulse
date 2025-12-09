/**
 * Tests for status utility functions
 * 
 * These tests verify the status indicator logic for various resource types.
 */
import { describe, expect, it } from 'vitest';
import {
    isNodeOnline,
    isGuestRunning,
    getNodeStatusIndicator,
    getGuestPowerIndicator,
    getHostStatusIndicator,
    getDockerHostStatusIndicator,
    getDockerContainerStatusIndicator,
    getDockerServiceStatusIndicator,
    OFFLINE_HEALTH_STATUSES,
    DEGRADED_HEALTH_STATUSES,
    STOPPED_CONTAINER_STATES,
    ERROR_CONTAINER_STATES,
} from '@/utils/status';

describe('isNodeOnline', () => {
    it('returns false for null/undefined', () => {
        expect(isNodeOnline(null)).toBe(false);
        expect(isNodeOnline(undefined)).toBe(false);
    });

    it('returns true for online node with uptime', () => {
        expect(isNodeOnline({ status: 'online', uptime: 1000 })).toBe(true);
    });

    it('returns false for offline status', () => {
        expect(isNodeOnline({ status: 'offline', uptime: 1000 })).toBe(false);
    });

    it('returns false for zero uptime', () => {
        expect(isNodeOnline({ status: 'online', uptime: 0 })).toBe(false);
    });

    it('returns false for negative uptime', () => {
        expect(isNodeOnline({ status: 'online', uptime: -1 })).toBe(false);
    });

    it('returns false for error connection health', () => {
        expect(isNodeOnline({ status: 'online', uptime: 1000, connectionHealth: 'error' })).toBe(false);
        expect(isNodeOnline({ status: 'online', uptime: 1000, connectionHealth: 'offline' })).toBe(false);
    });
});

describe('isGuestRunning', () => {
    it('returns false for null/undefined', () => {
        expect(isGuestRunning(null)).toBe(false);
        expect(isGuestRunning(undefined)).toBe(false);
    });

    it('returns true for running guest with online parent', () => {
        expect(isGuestRunning({ status: 'running' }, true)).toBe(true);
    });

    it('returns false for running guest with offline parent', () => {
        expect(isGuestRunning({ status: 'running' }, false)).toBe(false);
    });

    it('returns false for stopped guest', () => {
        expect(isGuestRunning({ status: 'stopped' }, true)).toBe(false);
    });
});

describe('getNodeStatusIndicator', () => {
    it('returns muted for null/undefined', () => {
        expect(getNodeStatusIndicator(null)).toEqual({ variant: 'muted', label: 'Unknown' });
        expect(getNodeStatusIndicator(undefined)).toEqual({ variant: 'muted', label: 'Unknown' });
    });

    it('returns success for online node', () => {
        const result = getNodeStatusIndicator({ status: 'online', uptime: 1000 });
        expect(result.variant).toBe('success');
        expect(result.label).toBe('Online');
    });

    it('returns danger for offline node', () => {
        const result = getNodeStatusIndicator({ status: 'offline', uptime: 0 });
        expect(result.variant).toBe('danger');
    });

    it('returns warning for degraded node', () => {
        const result = getNodeStatusIndicator({ status: 'degraded', uptime: 1000 });
        expect(result.variant).toBe('warning');
    });
});

describe('getGuestPowerIndicator', () => {
    it('returns muted for null/undefined', () => {
        expect(getGuestPowerIndicator(null)).toEqual({ variant: 'muted', label: 'Unknown' });
    });

    it('returns success for running guest', () => {
        const result = getGuestPowerIndicator({ status: 'running' }, true);
        expect(result.variant).toBe('success');
        expect(result.label).toBe('Running');
    });

    it('returns danger for stopped guest', () => {
        const result = getGuestPowerIndicator({ status: 'stopped' }, true);
        expect(result.variant).toBe('danger');
        expect(result.label).toBe('Stopped');
    });

    it('returns danger when parent node is offline', () => {
        const result = getGuestPowerIndicator({ status: 'running' }, false);
        expect(result.variant).toBe('danger');
        expect(result.label).toBe('Node offline');
    });
});

describe('getHostStatusIndicator', () => {
    it('returns muted for null/undefined', () => {
        expect(getHostStatusIndicator(null)).toEqual({ variant: 'muted', label: 'Unknown' });
    });

    it('returns success for online host', () => {
        const result = getHostStatusIndicator({ status: 'online' });
        expect(result.variant).toBe('success');
        expect(result.label).toBe('Online');
    });

    it('returns danger for offline host', () => {
        const result = getHostStatusIndicator({ status: 'offline' });
        expect(result.variant).toBe('danger');
    });

    it('returns warning for degraded host', () => {
        const result = getHostStatusIndicator({ status: 'degraded' });
        expect(result.variant).toBe('warning');
    });
});

describe('getDockerHostStatusIndicator', () => {
    it('returns muted for null/undefined', () => {
        expect(getDockerHostStatusIndicator(null)).toEqual({ variant: 'muted', label: 'Unknown' });
    });

    it('accepts string status directly', () => {
        const result = getDockerHostStatusIndicator('online');
        expect(result.variant).toBe('success');
    });

    it('returns success for healthy Docker host', () => {
        const result = getDockerHostStatusIndicator({ status: 'healthy' });
        expect(result.variant).toBe('success');
    });

    it('returns danger for offline Docker host', () => {
        const result = getDockerHostStatusIndicator({ status: 'offline' });
        expect(result.variant).toBe('danger');
    });
});

describe('getDockerContainerStatusIndicator', () => {
    it('returns muted for null/undefined', () => {
        expect(getDockerContainerStatusIndicator(null)).toEqual({ variant: 'muted', label: 'Unknown' });
    });

    it('returns success for running healthy container', () => {
        const result = getDockerContainerStatusIndicator({ state: 'running', health: 'healthy' });
        expect(result.variant).toBe('success');
        expect(result.label).toBe('Running');
    });

    it('returns success for running container without health', () => {
        const result = getDockerContainerStatusIndicator({ state: 'running' });
        expect(result.variant).toBe('success');
    });

    it('returns danger for unhealthy container', () => {
        const result = getDockerContainerStatusIndicator({ state: 'running', health: 'unhealthy' });
        expect(result.variant).toBe('danger');
        expect(result.label).toBe('Unhealthy');
    });

    it('returns danger for exited container', () => {
        const result = getDockerContainerStatusIndicator({ state: 'exited' });
        expect(result.variant).toBe('danger');
    });

    it('returns danger for dead container', () => {
        const result = getDockerContainerStatusIndicator({ state: 'dead' });
        expect(result.variant).toBe('danger');
    });
});

describe('getDockerServiceStatusIndicator', () => {
    it('returns muted for null/undefined', () => {
        expect(getDockerServiceStatusIndicator(null)).toEqual({ variant: 'muted', label: 'Unknown' });
    });

    it('returns success when running >= desired', () => {
        const result = getDockerServiceStatusIndicator({ desiredTasks: 3, runningTasks: 3 });
        expect(result.variant).toBe('success');
        expect(result.label).toBe('Healthy');
    });

    it('returns warning when running < desired but > 0', () => {
        const result = getDockerServiceStatusIndicator({ desiredTasks: 3, runningTasks: 2 });
        expect(result.variant).toBe('warning');
        expect(result.label).toBe('Degraded (2/3)');
    });

    it('returns danger when running is 0', () => {
        const result = getDockerServiceStatusIndicator({ desiredTasks: 3, runningTasks: 0 });
        expect(result.variant).toBe('danger');
        expect(result.label).toBe('Stopped (0/3)');
    });

    it('returns muted when desired is 0 and running is 0', () => {
        const result = getDockerServiceStatusIndicator({ desiredTasks: 0, runningTasks: 0 });
        expect(result.variant).toBe('muted');
        expect(result.label).toBe('No tasks');
    });

    it('returns warning when desired is 0 but running > 0', () => {
        const result = getDockerServiceStatusIndicator({ desiredTasks: 0, runningTasks: 2 });
        expect(result.variant).toBe('warning');
        expect(result.label).toBe('Running 2 tasks');
    });
});

describe('Status sets', () => {
    describe('OFFLINE_HEALTH_STATUSES', () => {
        it('contains expected offline statuses', () => {
            expect(OFFLINE_HEALTH_STATUSES.has('offline')).toBe(true);
            expect(OFFLINE_HEALTH_STATUSES.has('error')).toBe(true);
            expect(OFFLINE_HEALTH_STATUSES.has('failed')).toBe(true);
            expect(OFFLINE_HEALTH_STATUSES.has('timeout')).toBe(true);
            expect(OFFLINE_HEALTH_STATUSES.has('stopped')).toBe(true);
        });

        it('does not contain online statuses', () => {
            expect(OFFLINE_HEALTH_STATUSES.has('online')).toBe(false);
            expect(OFFLINE_HEALTH_STATUSES.has('running')).toBe(false);
            expect(OFFLINE_HEALTH_STATUSES.has('healthy')).toBe(false);
        });
    });

    describe('DEGRADED_HEALTH_STATUSES', () => {
        it('contains expected degraded statuses', () => {
            expect(DEGRADED_HEALTH_STATUSES.has('degraded')).toBe(true);
            expect(DEGRADED_HEALTH_STATUSES.has('warning')).toBe(true);
            expect(DEGRADED_HEALTH_STATUSES.has('syncing')).toBe(true);
            expect(DEGRADED_HEALTH_STATUSES.has('pending')).toBe(true);
        });
    });

    describe('STOPPED_CONTAINER_STATES', () => {
        it('contains expected stopped states', () => {
            expect(STOPPED_CONTAINER_STATES.has('exited')).toBe(true);
            expect(STOPPED_CONTAINER_STATES.has('stopped')).toBe(true);
            expect(STOPPED_CONTAINER_STATES.has('created')).toBe(true);
            expect(STOPPED_CONTAINER_STATES.has('paused')).toBe(true);
        });

        it('does not contain running state', () => {
            expect(STOPPED_CONTAINER_STATES.has('running')).toBe(false);
        });
    });

    describe('ERROR_CONTAINER_STATES', () => {
        it('contains expected error states', () => {
            expect(ERROR_CONTAINER_STATES.has('dead')).toBe(true);
            expect(ERROR_CONTAINER_STATES.has('oomkilled')).toBe(true);
            expect(ERROR_CONTAINER_STATES.has('unhealthy')).toBe(true);
            expect(ERROR_CONTAINER_STATES.has('restarting')).toBe(true);
        });
    });
});
