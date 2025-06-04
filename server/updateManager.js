const axios = require('axios');
const semver = require('semver');
const fs = require('fs').promises;
const path = require('path');
const { exec, spawn } = require('child_process');
const { promisify } = require('util');
const { getUpdateChannelPreference } = require('./configLoader');
const execAsync = promisify(exec);

class UpdateManager {
    constructor() {
        this.githubRepo = 'rcourtman/Pulse';
        this.currentVersion = require('../package.json').version;
        this.updateInProgress = false;
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
        try {
            console.log('[UpdateManager] Checking for updates...');
            
            // Use override channel if provided and valid, otherwise use config
            const configChannel = getUpdateChannelPreference();
            const updateChannel = (channelOverride && ['stable', 'rc'].includes(channelOverride)) 
                ? channelOverride 
                : configChannel;
            
            if (channelOverride && channelOverride !== configChannel) {
                console.log(`[UpdateManager] Using channel override: ${channelOverride} (config: ${configChannel})`);
            }
            let response;
            let channelDescription = '';
            
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
                    
                    if (releaseIsRC && semver.gt(releaseVersion, this.currentVersion)) {
                        latestRelease = release;
                        break;
                    }
                }
                
                if (!latestRelease) {
                    // No newer RC version found
                    const updateInfo = {
                        currentVersion: this.currentVersion,
                        latestVersion: this.currentVersion,
                        updateAvailable: false,
                        isDocker: this.isDockerEnvironment(),
                        releaseNotes: 'No newer RC version available',
                        releaseUrl: null,
                        publishedAt: null,
                        assets: [],
                        updateChannel: channelDescription
                    };
                    console.log(`[UpdateManager] No RC updates available: ${this.currentVersion}`);
                    return updateInfo;
                }
                
                response.data = latestRelease;
            }

            const latestVersion = response.data.tag_name.replace('v', '');
            
            // For stable channel, also consider "downgrade" from RC as an update
            const isCurrentRC = this.isReleaseCandidate();
            const isStableChannel = updateChannel === 'stable';
            const isDifferentVersion = latestVersion !== this.currentVersion;
            
            let updateAvailable;
            if (isStableChannel && isCurrentRC && isDifferentVersion) {
                // Offer stable version even if it's older than current RC
                updateAvailable = true;
            } else {
                // Normal case: only newer versions
                updateAvailable = semver.gt(latestVersion, this.currentVersion);
            }

            const updateInfo = {
                currentVersion: this.currentVersion,
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

            console.log(`[UpdateManager] Current version: ${this.currentVersion}, Latest version: ${latestVersion}, Channel: ${channelDescription}, Docker: ${updateInfo.isDocker}`);
            return updateInfo;

        } catch (error) {
            console.error('[UpdateManager] Error checking for updates:', error.message);
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
     * Apply update using the install script (reliable method)
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
            console.log('[UpdateManager] Applying update using install script...');
            
            if (progressCallback) {
                progressCallback({ phase: 'preparing', progress: 10 });
            }

            // Extract version from the download URL (more reliable than temp file path)
            let targetVersion = 'latest';
            if (downloadUrl && typeof downloadUrl === 'string') {
                // Extract from download URL like: https://github.com/user/repo/releases/download/v3.21.0/pulse-v3.21.0.tar.gz
                const urlMatch = downloadUrl.match(/\/releases\/download\/(v[\d\.\-\w]+)\//); 
                if (urlMatch) {
                    targetVersion = urlMatch[1];
                    console.log(`[UpdateManager] Extracted version from URL: ${targetVersion}`);
                }
            }
            
            // Fallback: try to extract from updateFile path if URL parsing failed
            if (targetVersion === 'latest' && typeof updateFile === 'string' && updateFile.includes('pulse-v')) {
                const fileMatch = updateFile.match(/pulse-v([\d\.\-\w]+)\.tar\.gz/);
                if (fileMatch) {
                    targetVersion = 'v' + fileMatch[1];
                    console.log(`[UpdateManager] Extracted version from file: ${targetVersion}`);
                }
            }

            if (progressCallback) {
                progressCallback({ phase: 'updating', progress: 20 });
            }

            // Use the proven install script for updates
            const installScriptPath = path.join(__dirname, '..', 'scripts', 'install-pulse.sh');
            
            // Check if install script exists
            try {
                await fs.access(installScriptPath);
            } catch (error) {
                throw new Error('Install script not found. Please update manually using the install script.');
            }

            console.log(`[UpdateManager] Running install script update to ${targetVersion}...`);
            
            // Validate version parameter to prevent injection attacks
            if (targetVersion !== 'latest' && !/^v[\d\.\-\w]+$/.test(targetVersion)) {
                throw new Error(`Invalid version format: ${targetVersion}. Expected format like v3.21.0`);
            }

            if (progressCallback) {
                progressCallback({ phase: 'downloading', progress: 30 });
            }

            // Execute the install script with real-time output parsing
            const updateProcess = spawn('sudo', ['bash', installScriptPath, '--update', ...(targetVersion !== 'latest' ? ['--version', targetVersion] : [])], {
                stdio: ['pipe', 'pipe', 'pipe'],
                env: { ...process.env, DEBIAN_FRONTEND: 'noninteractive' }
            });

            let output = '';
            let hasError = false;

            // Parse output for progress updates
            updateProcess.stdout.on('data', (data) => {
                const text = data.toString();
                output += text;
                console.log('[UpdateManager] Install script:', text.trim());
                
                // Parse progress from install script output
                if (progressCallback) {
                    if (text.includes('Downloading')) {
                        progressCallback({ phase: 'downloading', progress: 40 });
                    } else if (text.includes('Extracting')) {
                        progressCallback({ phase: 'extracting', progress: 60 });
                    } else if (text.includes('Backing up')) {
                        progressCallback({ phase: 'backup', progress: 70 });
                    } else if (text.includes('dependencies')) {
                        progressCallback({ phase: 'dependencies', progress: 80 });
                    } else if (text.includes('Service started') || text.includes('complete')) {
                        progressCallback({ phase: 'finishing', progress: 95 });
                    }
                }
            });

            updateProcess.stderr.on('data', (data) => {
                const text = data.toString();
                console.error('[UpdateManager] Install script error:', text.trim());
                if (!text.includes('Warning:') && !text.includes('WARN:')) {
                    hasError = true;
                }
            });

            // Wait for install script to complete with timeout
            const exitCode = await new Promise((resolve, reject) => {
                const timeout = setTimeout(() => {
                    updateProcess.kill('SIGTERM');
                    reject(new Error('Install script timeout after 10 minutes'));
                }, 600000); // 10 minutes timeout

                updateProcess.on('close', (code) => {
                    clearTimeout(timeout);
                    resolve(code);
                });

                updateProcess.on('error', (error) => {
                    clearTimeout(timeout);
                    reject(error);
                });
            });

            if (exitCode !== 0 || hasError) {
                throw new Error(`Install script failed with exit code ${exitCode}. Output: ${output}`);
            }

            if (progressCallback) {
                progressCallback({ phase: 'complete', progress: 100 });
            }

            console.log('[UpdateManager] Update completed successfully via install script');

            // The install script handles restart automatically
            console.log('[UpdateManager] Install script will handle service restart');
            
            // Cleanup temp file before process terminates
            try {
                await fs.unlink(updateFile);
                console.log('[UpdateManager] Cleaned up temporary update file');
            } catch (cleanupError) {
                console.warn('[UpdateManager] Could not cleanup temp file:', cleanupError.message);
            }
            
            // Note: The install script will restart the service, so this process will be terminated
            // Reset flag before process terminates (good practice)
            this.updateInProgress = false;

            return {
                success: true,
                message: 'Update applied successfully. The application will restart automatically.'
            };

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