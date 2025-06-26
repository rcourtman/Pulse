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
                    // Use git describe for accurate development versioning
                    const gitDescribe = execSync('git describe --tags --dirty', { 
                        cwd: gitDir, 
                        encoding: 'utf8' 
                    }).trim();
                    
                    if (gitDescribe) {
                        // Parse git describe output: v3.23.1-109-g3ee29b2-dirty
                        const match = gitDescribe.match(/^v([^-]+)(?:-(\d+)-g([a-f0-9]+))?(-dirty)?$/);
                        
                        if (match) {
                            const [, baseVersion, commitsAhead, shortHash, isDirty] = match;
                            
                            if (commitsAhead && parseInt(commitsAhead) > 0) {
                                // We're ahead of the latest tag - this is development
                                const commits = parseInt(commitsAhead);
                                const dirtyFlag = isDirty ? '-dirty' : '';
                                
                                // For development builds, show as dev version ahead of the base
                                // Calculate next logical version based on base
                                const versionParts = baseVersion.split('.');
                                const major = parseInt(versionParts[0]) || 0;
                                const minor = parseInt(versionParts[1]) || 0;
                                const patch = parseInt(versionParts[2]) || 0;
                                
                                // Increment minor version for development
                                const nextVersion = `${major}.${minor + 1}.0`;
                                currentVersion = `${nextVersion}-dev.${commits}+${shortHash}${dirtyFlag}`;
                            } else if (isDirty) {
                                // On a tag but with uncommitted changes
                                currentVersion = `${baseVersion}-dirty`;
                            } else {
                                // Exactly on a tag
                                currentVersion = baseVersion;
                            }
                        } else {
                            // Fallback if git describe doesn't match expected pattern
                            console.warn('[VersionUtils] Could not parse git describe output:', gitDescribe);
                            currentVersion = gitDescribe.replace(/^v/, '');
                        }
                    } else {
                        throw new Error('git describe returned empty');
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

/**
 * Analyze commits since the last stable release to suggest version bump
 * @returns {Object} Analysis with suggested version bump and reasoning
 */
function analyzeCommitsForVersionBump() {
    try {
        const { execSync } = require('child_process');
        const packageJson = require('../package.json');
        const gitDir = path.join(__dirname, '..');
        
        let latestStableTag;
        try {
            latestStableTag = execSync('git tag -l "v*" | grep -v "rc\\|alpha\\|beta" | sort -V | tail -1', { 
                cwd: gitDir, 
                encoding: 'utf8',
                shell: '/bin/bash'
            }).trim();
        } catch (error) {
            // No stable tags found, use v0.0.0 as baseline
            latestStableTag = 'v0.0.0';
        }
        
        if (!latestStableTag) {
            latestStableTag = 'v0.0.0';
        }
        
        // Get commit messages since last stable release
        let commitMessages;
        try {
            commitMessages = execSync(`git log ${latestStableTag}..HEAD --pretty=format:"%s"`, { 
                cwd: gitDir, 
                encoding: 'utf8' 
            }).trim();
        } catch (error) {
            // If git log fails, assume no commits
            commitMessages = '';
        }
        
        if (!commitMessages) {
            return {
                currentStableVersion: latestStableTag.replace(/^v/, ''),
                suggestedVersion: packageJson.version,
                bumpType: 'none',
                reasoning: 'No commits since last stable release',
                commits: [],
                analysis: {
                    breaking: [],
                    features: [],
                    fixes: [],
                    other: []
                },
                totalCommits: 0
            };
        }
        
        const commits = commitMessages.split('\n').filter(msg => msg.trim());
        
        // Analyze commit types
        const analysis = {
            breaking: [],
            features: [],
            fixes: [],
            other: []
        };
        
        commits.forEach(commit => {
            const msg = commit.toLowerCase();
            
            // Check for breaking changes
            if (commit.includes('BREAKING CHANGE') || commit.includes('!:')) {
                analysis.breaking.push(commit);
            }
            // Check for features
            else if (msg.startsWith('feat:') || msg.startsWith('feature:')) {
                analysis.features.push(commit);
            }
            // Check for fixes
            else if (msg.startsWith('fix:') || msg.startsWith('bugfix:')) {
                analysis.fixes.push(commit);
            }
            // Everything else
            else {
                analysis.other.push(commit);
            }
        });
        
        // Determine version bump type
        let bumpType = 'patch';
        let reasoning = '';
        
        if (analysis.breaking.length > 0) {
            bumpType = 'major';
            reasoning = `Major bump due to ${analysis.breaking.length} breaking change(s)`;
        } else if (analysis.features.length > 0) {
            bumpType = 'minor';
            reasoning = `Minor bump due to ${analysis.features.length} new feature(s)`;
        } else if (analysis.fixes.length > 0) {
            bumpType = 'patch';
            reasoning = `Patch bump due to ${analysis.fixes.length} bug fix(es)`;
        } else {
            bumpType = 'patch';
            reasoning = `Patch bump for ${analysis.other.length} other change(s)`;
        }
        
        // Calculate suggested version
        const currentStableVersion = latestStableTag.replace(/^v/, '');
        const suggestedVersion = calculateNextVersion(currentStableVersion, bumpType);
        
        return {
            currentStableVersion,
            suggestedVersion,
            bumpType,
            reasoning,
            commits: commits,
            analysis,
            totalCommits: commits.length
        };
        
    } catch (error) {
        console.warn('[VersionUtils] Error analyzing commits for version bump:', error.message);
        const packageJson = require('../package.json');
        return {
            currentStableVersion: packageJson.version,
            suggestedVersion: packageJson.version,
            bumpType: 'none',
            reasoning: 'Error analyzing commits',
            commits: [],
            analysis: {
                breaking: [],
                features: [],
                fixes: [],
                other: []
            },
            totalCommits: 0
        };
    }
}

/**
 * Calculate the next version based on current version and bump type
 * @param {string} currentVersion - Current semantic version (e.g., "3.24.0")
 * @param {string} bumpType - Type of bump: major, minor, or patch
 * @returns {string} Next version
 */
function calculateNextVersion(currentVersion, bumpType) {
    try {
        // Parse current version
        const versionMatch = currentVersion.match(/^(\d+)\.(\d+)\.(\d+)/);
        if (!versionMatch) {
            throw new Error(`Invalid version format: ${currentVersion}`);
        }
        
        let [, major, minor, patch] = versionMatch.map(Number);
        
        switch (bumpType) {
            case 'major':
                major += 1;
                minor = 0;
                patch = 0;
                break;
            case 'minor':
                minor += 1;
                patch = 0;
                break;
            case 'patch':
                patch += 1;
                break;
            default:
                // No bump
                break;
        }
        
        return `${major}.${minor}.${patch}`;
    } catch (error) {
        console.warn('[VersionUtils] Error calculating next version:', error.message);
        return currentVersion;
    }
}

/**
 * Check if current branch should trigger a stable release
 * (i.e., we're on main branch and last commit was a merge from develop)
 * @returns {boolean} True if this should trigger a stable release
 */
function shouldTriggerStableRelease() {
    try {
        const { execSync } = require('child_process');
        const gitDir = path.join(__dirname, '..');
        
        // Check if we're on main branch
        const currentBranch = execSync('git branch --show-current', { 
            cwd: gitDir, 
            encoding: 'utf8' 
        }).trim();
        
        if (currentBranch !== 'main') {
            return false;
        }
        
        // Check if the last commit was a merge from develop
        try {
            const lastCommitMessage = execSync('git log -1 --pretty=format:"%s"', { 
                cwd: gitDir, 
                encoding: 'utf8' 
            }).trim();
            
            // Look for merge commit patterns from develop
            const isMergeFromDevelop = lastCommitMessage.includes('Merge branch \'develop\'') ||
                                    lastCommitMessage.includes('Merge pull request') ||
                                    lastCommitMessage.includes('develop');
            
            return isMergeFromDevelop;
        } catch (error) {
            return false;
        }
        
    } catch (error) {
        console.warn('[VersionUtils] Error checking for stable release trigger:', error.message);
        return false;
    }
}

module.exports = {
    getCurrentVersionInfo,
    getCurrentVersion,
    analyzeCommitsForVersionBump,
    calculateNextVersion,
    shouldTriggerStableRelease
};