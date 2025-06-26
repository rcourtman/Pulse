const axios = require('axios');
const semver = require('semver');
const fs = require('fs').promises;
const path = require('path');
const { exec, spawn } = require('child_process');
const { promisify } = require('util');
const { getUpdateChannelPreference } = require('./configLoader');
const { getCurrentVersion } = require('./versionUtils');
const execAsync = promisify(exec);

class UpdateManager {
    constructor() {
        this.githubRepo = 'rcourtman/Pulse';
        this.currentVersion = require('../package.json').version;
        this.updateInProgress = false;
        // Cache to prevent excessive GitHub API calls
        this.cache = new Map();
        this.cacheExpiry = 30 * 60 * 1000; // 30 minutes
    }

    /**
     * Check if current version is a release candidate
     */
    isReleaseCandidate(version) {
        // Only use currentVersion as default if no argument is passed at all
        const versionToCheck = (arguments.length === 0) ? this.currentVersion : version;
        
        if (!versionToCheck || typeof versionToCheck !== 'string') {
            return false;
        }
        const versionLower = versionToCheck.toLowerCase();
        return versionLower.includes('-rc') || versionLower.includes('-alpha') || versionLower.includes('-beta');
    }

    /**
     * Validate download URL for security
     */
    isValidDownloadUrl(downloadUrl) {
        if (!downloadUrl || typeof downloadUrl !== 'string') {
            return false;
        }
        
        try {
            const url = new URL(downloadUrl);
            
            // Allow test mode URLs
            if (process.env.UPDATE_TEST_MODE === 'true' && 
                url.hostname === 'localhost' && 
                url.pathname.includes('/api/test/mock-update.tar.gz')) {
                return true;
            }
            
            // Only allow HTTPS GitHub release asset URLs
            return url.protocol === 'https:' &&
                   url.hostname === 'github.com' && 
                   url.pathname.includes('/releases/download/') &&
                   url.pathname.includes(`/${this.githubRepo}/`);
        } catch (error) {
            return false;
        }
    }

    /**
     * Check for available updates
     * @param {string} channelOverride - Optional channel override ('stable' or 'rc')
     */
    async checkForUpdates(channelOverride = null) {
        const dynamicCurrentVersion = getCurrentVersion();
        
        // Use override channel if provided and valid, otherwise use config
        const configChannel = getUpdateChannelPreference();
        const updateChannel = (channelOverride && ['stable', 'rc'].includes(channelOverride)) 
            ? channelOverride 
            : configChannel;
        let channelDescription = '';
        
        // Check cache first to prevent excessive API calls
        const cacheKey = `${updateChannel}-${dynamicCurrentVersion}`;
        const cached = this.cache.get(cacheKey);
        if (cached && Date.now() - cached.timestamp < this.cacheExpiry) {
            console.log(`[UpdateManager] Using cached result for ${updateChannel} channel`);
            return cached.data;
        }
        
        try {
            console.log('[UpdateManager] Checking for updates...');
            
            if (channelOverride && channelOverride !== configChannel) {
                console.log(`[UpdateManager] Using channel override: ${channelOverride} (config: ${configChannel})`);
            }
            let response;
            
            if (updateChannel === 'stable') {
                // Stable channel: only check latest stable release
                channelDescription = 'stable releases only';
                console.log('[UpdateManager] Checking for stable releases...');
                response = await axios.get(
                    `https://api.github.com/repos/${this.githubRepo}/releases/latest`,
                    {
                        headers: {
                            'Accept': 'application/vnd.github.v3+json',
                            'User-Agent': 'Pulse-Update-Checker'
                        },
                        timeout: 10000
                    }
                );
            } else {
                // RC channel: check all releases for RC versions
                channelDescription = 'RC releases only';
                console.log('[UpdateManager] Checking for RC releases...');
                response = await axios.get(
                    `https://api.github.com/repos/${this.githubRepo}/releases?per_page=10`,
                    {
                        headers: {
                            'Accept': 'application/vnd.github.v3+json',
                            'User-Agent': 'Pulse-Update-Checker'
                        },
                        timeout: 10000
                    }
                );

                // Find the latest RC release that's newer than current
                let latestRelease = null;
                const releases = response.data;
                
                for (const release of releases) {
                    const releaseVersion = release.tag_name.replace('v', '');
                    const releaseIsRC = this.isReleaseCandidate(releaseVersion);
                    
                    if (releaseIsRC) {
                        // For RC channel, show the latest RC regardless of current version
                        // This allows showing RC versions even if current is stable
                        if (!latestRelease || semver.gt(releaseVersion, latestRelease.tag_name.replace('v', ''))) {
                            latestRelease = release;
                        }
                    }
                }
                
                if (!latestRelease) {
                    // No newer RC version found
                    const updateInfo = {
                        currentVersion: dynamicCurrentVersion,
                        latestVersion: dynamicCurrentVersion,
                        updateAvailable: false,
                        isDocker: this.isDockerEnvironment(),
                        releaseNotes: 'No newer RC version available',
                        releaseUrl: null,
                        publishedAt: null,
                        assets: [],
                        updateChannel: channelDescription
                    };
                    console.log(`[UpdateManager] No RC updates available: ${dynamicCurrentVersion}`);
                    return updateInfo;
                }
                
                response.data = latestRelease;
            }

            const latestVersion = response.data.tag_name.replace('v', '');
            
            // Channels are completely separate - no cross-channel comparisons
            // Simply check if the latest version in the selected channel differs from current
            const updateAvailable = latestVersion !== dynamicCurrentVersion;

            const updateInfo = {
                currentVersion: dynamicCurrentVersion,
                latestVersion,
                updateAvailable,
                isDocker: this.isDockerEnvironment(),
                releaseNotes: response.data.body || 'No release notes available',
                releaseUrl: response.data.html_url,
                publishedAt: response.data.published_at,
                updateChannel: channelDescription,
                assets: response.data.assets.map(asset => ({
                    name: asset.name,
                    size: asset.size,
                    downloadUrl: asset.browser_download_url
                }))
            };

            console.log(`[UpdateManager] Current version: ${dynamicCurrentVersion}, Latest version: ${latestVersion}, Channel: ${channelDescription}, Docker: ${updateInfo.isDocker}`);
            
            // Cache the successful result
            this.cache.set(cacheKey, {
                data: updateInfo,
                timestamp: Date.now()
            });
            
            return updateInfo;

        } catch (error) {
            console.error('[UpdateManager] Error checking for updates:', error.message);
            
            // Handle different types of errors gracefully
            if (error.response?.status === 403) {
                // GitHub API rate limit exceeded
                console.warn('[UpdateManager] GitHub API rate limit exceeded, returning current version info');
                return {
                    currentVersion: dynamicCurrentVersion,
                    latestVersion: dynamicCurrentVersion,
                    updateAvailable: false,
                    isDocker: this.isDockerEnvironment(),
                    releaseNotes: 'Unable to check for updates: GitHub API rate limit exceeded. Please try again later.',
                    releaseUrl: null,
                    publishedAt: null,
                    assets: [],
                    updateChannel: channelDescription || 'unknown',
                    rateLimited: true
                };
            } else if (error.response?.status === 404) {
                // Repository or release not found
                console.warn('[UpdateManager] Repository or release not found');
                return {
                    currentVersion: dynamicCurrentVersion,
                    latestVersion: dynamicCurrentVersion,
                    updateAvailable: false,
                    isDocker: this.isDockerEnvironment(),
                    releaseNotes: 'Unable to check for updates: Repository or release not found.',
                    releaseUrl: null,
                    publishedAt: null,
                    assets: [],
                    updateChannel: channelDescription || 'unknown'
                };
            } else if (error.code === 'ENOTFOUND' || error.code === 'ECONNREFUSED' || error.code === 'ETIMEDOUT') {
                // Network connectivity issues
                console.warn('[UpdateManager] Network connectivity issues');
                return {
                    currentVersion: dynamicCurrentVersion,
                    latestVersion: dynamicCurrentVersion,
                    updateAvailable: false,
                    isDocker: this.isDockerEnvironment(),
                    releaseNotes: 'Unable to check for updates: Network connectivity issues. Please check your internet connection.',
                    releaseUrl: null,
                    publishedAt: null,
                    assets: [],
                    updateChannel: channelDescription || 'unknown'
                };
            }
            
            // For other errors, still throw but with more context
            throw new Error(`Failed to check for updates: ${error.message}`);
        }
    }

    /**
     * Download update package
     */
    async downloadUpdate(downloadUrl, progressCallback) {
        try {
            // Validate download URL for security
            if (!this.isValidDownloadUrl(downloadUrl)) {
                throw new Error('Invalid download URL. Only GitHub release assets are allowed.');
            }
            
            console.log('[UpdateManager] Downloading update from:', downloadUrl);
            
            const tempDir = path.join(__dirname, '..', 'temp');
            await fs.mkdir(tempDir, { recursive: true });
            
            const tempFile = path.join(tempDir, 'update.tar.gz');
            
            // In test mode, create a mock tarball directly instead of downloading
            if (process.env.UPDATE_TEST_MODE === 'true' && downloadUrl.includes('/api/test/mock-update.tar.gz')) {
                console.log('[UpdateManager] Test mode: Creating mock update package...');
                
                const tar = require('tar');
                await tar.create({
                    gzip: true,
                    file: tempFile,
                    cwd: path.join(__dirname, '..'),
                    filter: (path) => {
                        return !path.includes('node_modules') && 
                               !path.includes('.git') && 
                               !path.includes('temp') &&
                               !path.includes('data/backups');
                    }
                }, ['.']);
                
                // Simulate download progress
                if (progressCallback) {
                    for (let i = 0; i <= 100; i += 10) {
                        progressCallback({ phase: 'download', progress: i });
                        await new Promise(resolve => setTimeout(resolve, 100));
                    }
                }
                
                console.log('[UpdateManager] Test mode: Mock package created successfully');
                return tempFile;
            }
            
            // Normal download for real updates
            const response = await axios({
                method: 'get',
                url: downloadUrl,
                responseType: 'stream',
                timeout: 300000 // 5 minutes
            });

            const totalSize = parseInt(response.headers['content-length'], 10);
            let downloadedSize = 0;

            const writer = require('fs').createWriteStream(tempFile);
            
            response.data.on('data', (chunk) => {
                downloadedSize += chunk.length;
                if (progressCallback) {
                    const progress = Math.round((downloadedSize / totalSize) * 100);
                    progressCallback({ phase: 'download', progress });
                }
            });

            response.data.pipe(writer);

            return new Promise((resolve, reject) => {
                writer.on('finish', () => resolve(tempFile));
                writer.on('error', (error) => {
                    console.error('[UpdateManager] Writer error:', error);
                    reject(error);
                });
                response.data.on('error', (error) => {
                    console.error('[UpdateManager] Response stream error:', error);
                    writer.destroy();
                    reject(error);
                });
            });

        } catch (error) {
            console.error('[UpdateManager] Error downloading update:', error);
            console.error('[UpdateManager] Error details:', {
                message: error.message,
                code: error.code,
                response: error.response?.status,
                responseData: error.response?.data
            });
            throw new Error(`Failed to download update: ${error.message || error.toString()}`);
        }
    }

    /**
     * Check if running in Docker
     */
    isDockerEnvironment() {
        return process.env.DOCKER_DEPLOYMENT === 'true' || 
               require('fs').existsSync('/.dockerenv') ||
               (process.env.container === 'docker');
    }

    /**
     * Recursively copy directory
     */
    async copyDirectory(src, dest) {
        await fs.mkdir(dest, { recursive: true });
        const entries = await fs.readdir(src, { withFileTypes: true });
        
        for (const entry of entries) {
            const srcPath = path.join(src, entry.name);
            const destPath = path.join(dest, entry.name);
            
            if (entry.isDirectory()) {
                await this.copyDirectory(srcPath, destPath);
            } else {
                await fs.copyFile(srcPath, destPath);
            }
        }
    }

    /**
     * Apply update without requiring sudo
     */
    async applyUpdate(updateFile, progressCallback, downloadUrl = null) {
        if (this.updateInProgress) {
            throw new Error('Update already in progress');
        }

        // Check if running in Docker
        if (this.isDockerEnvironment()) {
            throw new Error(
                'Automatic updates are not supported in Docker deployments. ' +
                'Please update your Docker image by pulling the latest version:\n' +
                'docker pull rcourtman/pulse:latest\n' +
                'or update your docker-compose.yml to use the new version tag.'
            );
        }

        this.updateInProgress = true;

        try {
            console.log('[UpdateManager] Starting update process...');
            
            if (progressCallback) {
                progressCallback({ phase: 'preparing', progress: 5 });
            }

            // Extract version from the download URL
            let targetVersion = 'latest';
            if (downloadUrl && typeof downloadUrl === 'string') {
                const urlMatch = downloadUrl.match(/\/releases\/download\/(v[\d\.\-\w]+)\//); 
                if (urlMatch) {
                    targetVersion = urlMatch[1];
                    console.log(`[UpdateManager] Target version: ${targetVersion}`);
                }
            }
            
            // Validate version parameter
            if (targetVersion !== 'latest' && !/^v[\d\.\-\w]+$/.test(targetVersion)) {
                throw new Error(`Invalid version format: ${targetVersion}. Expected format like v3.21.0`);
            }

            // Step 1: Create backup
            if (progressCallback) {
                progressCallback({ phase: 'backup', progress: 15 });
            }
            await this.createBackup();

            // Step 2: Extract update
            if (progressCallback) {
                progressCallback({ phase: 'extract', progress: 40 });
            }
            await this.extractUpdate(updateFile);

            // Step 3: Install dependencies
            if (progressCallback) {
                progressCallback({ phase: 'apply', progress: 70 });
            }
            await this.installDependencies();

            // Step 4: Signal completion and schedule restart
            if (progressCallback) {
                progressCallback({ phase: 'restarting', progress: 100 });
            }
            
            console.log('[UpdateManager] Update completed successfully. Preparing for restart...');
            
            // Cleanup temp file before restart
            try {
                await fs.unlink(updateFile);
                console.log('[UpdateManager] Cleaned up temporary update file');
            } catch (cleanupError) {
                console.warn('[UpdateManager] Could not cleanup temp file:', cleanupError.message);
            }

            // Return success immediately, then restart after a delay
            const result = {
                success: true,
                message: 'Update applied successfully. Service will restart shortly...',
                targetVersion: targetVersion
            };

            // Schedule service restart after allowing time for response to be sent
            setTimeout(async () => {
                try {
                    this.updateInProgress = false;
                    await this.restartService();
                } catch (error) {
                    console.error('[UpdateManager] Failed to restart service:', error.message);
                    this.updateInProgress = false;
                }
            }, 2000); // Give time for WebSocket message to be sent

            return result;

        } catch (error) {
            console.error('[UpdateManager] Error applying update:', error.message);
            
            // Cleanup temp file on failure
            try {
                await fs.unlink(updateFile);
                console.log('[UpdateManager] Cleaned up temporary update file after failure');
            } catch (cleanupError) {
                console.warn('[UpdateManager] Could not cleanup temp file after failure:', cleanupError.message);
            }
            
            this.updateInProgress = false;
            throw new Error(`Failed to apply update: ${error.message}`);
        }
    }

    /**
     * Create backup of current installation
     */
    async createBackup() {
        console.log('[UpdateManager] Creating backup...');
        
        const backupDir = path.join(__dirname, '..', 'data', 'backups');
        await fs.mkdir(backupDir, { recursive: true });
        
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
        const backupPath = path.join(backupDir, `pulse-backup-${timestamp}.tar.gz`);
        
        const tar = require('tar');
        await tar.create({
            gzip: true,
            file: backupPath,
            cwd: path.join(__dirname, '..'),
            filter: (path) => {
                // Exclude node_modules, temp files, and backups from backup
                return !path.includes('node_modules') && 
                       !path.includes('.git') && 
                       !path.includes('temp') &&
                       !path.includes('data/backups');
            }
        }, ['.']);
        
        console.log(`[UpdateManager] Backup created: ${backupPath}`);
    }

    /**
     * Extract update files
     */
    async extractUpdate(updateFile) {
        console.log('[UpdateManager] Extracting update files...');
        
        const pulseDir = path.join(__dirname, '..');
        const tar = require('tar');
        
        await tar.extract({
            file: updateFile,
            cwd: pulseDir,
            strip: 1 // Remove the top-level directory from the tarball
        });
        
        console.log('[UpdateManager] Update files extracted successfully');
    }

    /**
     * Install npm dependencies if needed
     */
    async installDependencies() {
        console.log('[UpdateManager] Checking dependencies...');
        
        const pulseDir = path.join(__dirname, '..');
        const nodeModulesPath = path.join(pulseDir, 'node_modules');
        
        try {
            // Check if node_modules exists and has content
            const nodeModulesExists = await fs.access(nodeModulesPath).then(() => true).catch(() => false);
            
            if (nodeModulesExists) {
                const moduleFiles = await fs.readdir(nodeModulesPath);
                if (moduleFiles.length > 0) {
                    console.log('[UpdateManager] Dependencies already bundled in update package');
                    return;
                }
            }
            
            // Only run npm ci if node_modules is missing or empty
            console.log('[UpdateManager] Installing missing dependencies...');
            await execAsync('npm ci --production', { 
                cwd: pulseDir,
                timeout: 300000 // 5 minutes
            });
            console.log('[UpdateManager] Dependencies installed successfully');
        } catch (error) {
            console.warn('[UpdateManager] Failed to install dependencies:', error.message);
            // Don't fail the update for dependency issues
        }
    }

    /**
     * Restart the pulse service using multiple strategies
     */
    async restartService() {
        console.log('[UpdateManager] Restarting pulse service...');
        
        try {
            console.log('[UpdateManager] Attempting restart via pkexec...');
            await execAsync('pkexec systemctl restart pulse.service', { timeout: 10000 });
            console.log('[UpdateManager] Service restarted successfully via pkexec');
            return;
        } catch (error) {
            console.warn('[UpdateManager] pkexec restart failed:', error.message);
        }
        
        try {
            console.log('[UpdateManager] Attempting restart via systemctl...');
            await execAsync('systemctl restart pulse.service', { timeout: 10000 });
            console.log('[UpdateManager] Service restarted successfully via systemctl');
            return;
        } catch (error) {
            console.warn('[UpdateManager] systemctl restart failed:', error.message);
        }
        
        // Strategy 3: Use systemd-run to restart in separate session
        try {
            console.log('[UpdateManager] Attempting restart via systemd-run...');
            await execAsync('systemd-run --no-ask-password --scope systemctl restart pulse.service', { timeout: 10000 });
            console.log('[UpdateManager] Service restarted successfully via systemd-run');
            return;
        } catch (error) {
            console.warn('[UpdateManager] systemd-run restart failed:', error.message);
        }
        
        // Strategy 4: Graceful shutdown and let systemd restart
        console.log('[UpdateManager] All restart methods failed. Using graceful shutdown...');
        console.log('[UpdateManager] systemd will automatically restart the service');
        
        // Close server gracefully then exit with non-zero status to trigger systemd restart
        if (global.server) {
            global.server.close(() => {
                console.log('[UpdateManager] Server closed gracefully');
                process.exit(1);
            });
            
            // Force exit after 5 seconds if graceful close doesn't work
            setTimeout(() => {
                console.log('[UpdateManager] Forcing process exit');
                process.exit(1);
            }, 5000);
        } else {
            process.exit(1);
        }
    }

    /**
     * Get update status
     */
    getUpdateStatus() {
        return {
            updateInProgress: this.updateInProgress,
            currentVersion: this.currentVersion
        };
    }
}

module.exports = UpdateManager;