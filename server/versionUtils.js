/**
 * Centralized version calculation utility
 * Used by both /api/version endpoint and UpdateManager to ensure consistency
 */

const { execSync } = require('child_process');
const path = require('path');

/**
 * Calculate the current version dynamically from git
 * @returns {Object} Version information including version, branch, and isDevelopment
 */
function getCurrentVersionInfo() {
    try {
        const packageJson = require('../package.json');
        
        let currentVersion = packageJson.version || 'N/A';
        let gitBranch = null;
        let isDevelopment = false;
        
        // Try to detect git branch and calculate dynamic version
        try {
            const gitDir = path.join(__dirname, '..');
            
            // Get current branch
            gitBranch = execSync('git branch --show-current', { 
                cwd: gitDir, 
                encoding: 'utf8' 
            }).trim();
            
            // If on develop branch, calculate RC version from git
            if (gitBranch === 'develop') {
                isDevelopment = true;
                try {
                    // Get the latest stable release tag
                    const latestStableTag = execSync('git tag -l "v*" | grep -v "rc\\\\|alpha\\\\|beta" | sort -V | tail -1', { 
                        cwd: gitDir, 
                        encoding: 'utf8',
                        shell: '/bin/bash'
                    }).trim();
                    
                    if (latestStableTag) {
                        // Remove 'v' prefix to get base version
                        const baseVersion = latestStableTag.replace(/^v/, '');
                        
                        // Count commits since the latest stable tag
                        const commitsSince = execSync(`git rev-list --count ${latestStableTag}..HEAD`, { 
                            cwd: gitDir, 
                            encoding: 'utf8' 
                        }).trim();
                        
                        const commitsCount = parseInt(commitsSince, 10);
                        
                        if (commitsCount > 0) {
                            // Calculate RC version: base version + rc + commit count
                            currentVersion = `${baseVersion}-rc${commitsCount}`;
                        } else {
                            // No commits since stable, use base version
                            currentVersion = baseVersion;
                        }
                    }
                } catch (versionError) {
                    console.log('[VersionUtils] Could not calculate RC version from git, using package.json');
                    // Fall back to package.json version
                    currentVersion = packageJson.version;
                }
            }
        } catch (gitError) {
            // Git not available or not a git repo
            gitBranch = null;
            currentVersion = packageJson.version;
        }
        
        return {
            version: currentVersion,
            gitBranch: gitBranch,
            isDevelopment: isDevelopment || gitBranch === 'develop' || process.env.NODE_ENV === 'development'
        };
    } catch (error) {
        console.warn('[VersionUtils] Error getting current version:', error.message);
        const packageJson = require('../package.json');
        return {
            version: packageJson.version || 'N/A',
            gitBranch: null,
            isDevelopment: false
        };
    }
}

/**
 * Get just the version string (for backwards compatibility)
 * @returns {string} The current version
 */
function getCurrentVersion() {
    return getCurrentVersionInfo().version;
}

module.exports = {
    getCurrentVersionInfo,
    getCurrentVersion
};