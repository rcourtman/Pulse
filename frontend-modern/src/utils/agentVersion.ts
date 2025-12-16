/**
 * Agent version comparison utilities
 *
 * Used to detect outdated agents by comparing their version to the server version.
 */

import { updateStore } from '@/stores/updates';

/**
 * Parses a version string (e.g., "v5.0.0", "5.0.0-rc.3", "dev") into components
 */
interface ParsedVersion {
    major: number;
    minor: number;
    patch: number;
    prerelease: string | null;
    isValid: boolean;
}

function parseVersion(version: string): ParsedVersion {
    if (!version || version === 'dev' || version === 'unknown') {
        return { major: 0, minor: 0, patch: 0, prerelease: null, isValid: false };
    }

    // Remove leading 'v' if present
    const cleaned = version.startsWith('v') ? version.slice(1) : version;

    // Split into version and prerelease parts
    const [versionPart, ...prereleaseParts] = cleaned.split('-');
    const prerelease = prereleaseParts.length > 0 ? prereleaseParts.join('-') : null;

    // Parse major.minor.patch
    const parts = versionPart.split('.').map((p) => parseInt(p, 10));

    if (parts.some((p) => isNaN(p))) {
        return { major: 0, minor: 0, patch: 0, prerelease: null, isValid: false };
    }

    return {
        major: parts[0] || 0,
        minor: parts[1] || 0,
        patch: parts[2] || 0,
        prerelease,
        isValid: true,
    };
}

/**
 * Compares two version strings
 * Returns:
 *   -1 if a < b
 *    0 if a == b
 *    1 if a > b
 */
export function compareVersions(a: string, b: string): number {
    const va = parseVersion(a);
    const vb = parseVersion(b);

    // Invalid versions are considered older
    if (!va.isValid && !vb.isValid) return 0;
    if (!va.isValid) return -1;
    if (!vb.isValid) return 1;

    // Compare major.minor.patch
    if (va.major !== vb.major) return va.major - vb.major;
    if (va.minor !== vb.minor) return va.minor - vb.minor;
    if (va.patch !== vb.patch) return va.patch - vb.patch;

    // Same base version, compare prerelease
    // No prerelease = stable = newer than prerelease
    if (!va.prerelease && vb.prerelease) return 1;
    if (va.prerelease && !vb.prerelease) return -1;
    if (!va.prerelease && !vb.prerelease) return 0;

    // Both have prerelease, compare lexicographically
    return (va.prerelease || '').localeCompare(vb.prerelease || '');
}

/**
 * Gets the current server version from the update store
 */
export function getServerVersion(): string {
    const info = updateStore.versionInfo();
    return info?.version ?? '';
}

/**
 * Checks if an agent version is outdated compared to the server version
 * Returns an object with the status and details
 */
export interface AgentVersionStatus {
    isOutdated: boolean;
    isDev: boolean;
    isUnknown: boolean;
    serverVersion: string;
    comparisonResult: number; // -1 = older, 0 = same, 1 = newer
}

export function checkAgentVersion(agentVersion?: string): AgentVersionStatus {
    const result: AgentVersionStatus = {
        isOutdated: false,
        isDev: false,
        isUnknown: false,
        serverVersion: getServerVersion(),
        comparisonResult: 0,
    };

    if (!agentVersion) {
        result.isUnknown = true;
        return result;
    }

    // Dev versions are always considered potentially outdated
    if (agentVersion === 'dev' || agentVersion.includes('-dirty')) {
        result.isDev = true;
        // Don't mark dev as outdated if server is also dev
        if (result.serverVersion && !result.serverVersion.includes('dev')) {
            result.isOutdated = true;
        }
        return result;
    }

    // Compare to server version if available
    if (result.serverVersion) {
        result.comparisonResult = compareVersions(agentVersion, result.serverVersion);
        // Agent is outdated if it's older than server
        result.isOutdated = result.comparisonResult < 0;
    }

    return result;
}

/**
 * Simple check for whether an agent should show an outdated warning
 * This is a convenience wrapper around checkAgentVersion
 */
export function isAgentOutdated(agentVersion?: string): boolean {
    const status = checkAgentVersion(agentVersion);
    return status.isOutdated || status.isDev;
}

/**
 * Gets a tooltip message explaining the agent version status
 */
export function getAgentVersionTooltip(agentVersion?: string): string {
    const status = checkAgentVersion(agentVersion);

    if (status.isUnknown) {
        return 'Agent version unknown';
    }

    if (status.isDev) {
        return 'Development build - consider updating to a release version';
    }

    if (status.isOutdated && status.serverVersion) {
        return `Agent is outdated (${agentVersion} < ${status.serverVersion}). Re-run the install script to update.`;
    }

    if (status.comparisonResult > 0 && status.serverVersion) {
        return `Agent is newer than server (${agentVersion} > ${status.serverVersion})`;
    }

    return 'Agent is up to date';
}
