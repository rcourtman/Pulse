const axios = require('axios');
const semver = require('semver');
const fs = require('fs').promises;
const path = require('path');
const { exec } = require('child_process');
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
        return versionToCheck.includes('-rc') || versionToCheck.includes('-alpha') || versionToCheck.includes('-beta');
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
     */
    async checkForUpdates() {
        try {
            console.log('[UpdateManager] Checking for updates...');
            
            const updateChannel = getUpdateChannelPreference();
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
            const updateAvailable = semver.gt(latestVersion, this.currentVersion);

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
     * Apply update
     */
    async applyUpdate(updateFile, progressCallback) {
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
            console.log('[UpdateManager] Applying update...');
            
            // Create backup directory
            const backupDir = path.join(__dirname, '..', 'backup', `backup-${Date.now()}`);
            await fs.mkdir(backupDir, { recursive: true });

            if (progressCallback) {
                progressCallback({ phase: 'backup', progress: 0 });
            }

            // Backup critical files
            const filesToBackup = [
                '.env',
                'data/metrics.db',
                'data/acknowledgements.json'
            ];

            for (let i = 0; i < filesToBackup.length; i++) {
                const file = filesToBackup[i];
                const sourcePath = path.join(__dirname, '..', file);
                const backupPath = path.join(backupDir, file);
                
                try {
                    await fs.mkdir(path.dirname(backupPath), { recursive: true });
                    await fs.copyFile(sourcePath, backupPath);
                } catch (error) {
                    if (error.code !== 'ENOENT') {
                        console.warn(`[UpdateManager] Warning: Could not backup ${file}:`, error.message);
                    }
                }

                if (progressCallback) {
                    const progress = Math.round(((i + 1) / filesToBackup.length) * 100);
                    progressCallback({ phase: 'backup', progress });
                }
            }

            if (progressCallback) {
                progressCallback({ phase: 'extract', progress: 0 });
            }

            // Extract update
            const tempExtractDir = path.join(__dirname, '..', 'temp', 'extract');
            await fs.mkdir(tempExtractDir, { recursive: true });

            console.log('[UpdateManager] Extracting update tarball...');
            await execAsync(`tar -xzf ${updateFile} -C ${tempExtractDir}`);

            // Find the extracted directory (should be pulse-vX.Y.Z)
            const extractedFiles = await fs.readdir(tempExtractDir);
            const extractedDirName = extractedFiles.find(file => file.startsWith('pulse-v'));
            
            if (!extractedDirName) {
                throw new Error('Invalid update package: pulse directory not found');
            }
            
            const extractedPulsePath = path.join(tempExtractDir, extractedDirName);
            
            // Verify essential files exist
            const requiredFiles = ['package.json', 'server/index.js', 'node_modules'];
            for (const file of requiredFiles) {
                const filePath = path.join(extractedPulsePath, file);
                try {
                    await fs.access(filePath);
                } catch (error) {
                    throw new Error(`Invalid update package: missing ${file}`);
                }
            }
            
            console.log('[UpdateManager] Update package validated successfully');

            if (progressCallback) {
                progressCallback({ phase: 'extract', progress: 100 });
            }

            const currentPulseDir = path.join(__dirname, '..');

            if (progressCallback) {
                progressCallback({ phase: 'apply', progress: 50 });
            }

            // Apply update files
            console.log('[UpdateManager] Applying update files...');
            
            // List files to update from the extracted pulse directory (exclude config and data)
            const updateFiles = await fs.readdir(extractedPulsePath);
            for (const file of updateFiles) {
                if (file === '.env' || file === 'data') continue;
                
                const sourcePath = path.join(extractedPulsePath, file);
                const destPath = path.join(currentPulseDir, file);
                
                // Remove existing file/directory
                try {
                    await fs.rm(destPath, { recursive: true, force: true });
                } catch (e) {
                    // Ignore errors
                }
                
                // Copy new file/directory using Node.js fs
                const stat = await fs.stat(sourcePath);
                if (stat.isDirectory()) {
                    await this.copyDirectory(sourcePath, destPath);
                } else {
                    await fs.copyFile(sourcePath, destPath);
                }
            }

            // No need to install dependencies - the release tarball already includes node_modules
            console.log('[UpdateManager] Release tarball already includes dependencies, skipping npm install...');

            if (progressCallback) {
                progressCallback({ phase: 'apply', progress: 100 });
            }

            // Schedule restart
            console.log('[UpdateManager] Scheduling restart...');
            setTimeout(async () => {
                console.log('[UpdateManager] Attempting restart...');
                
                // In test mode or development, just exit (user needs to restart manually)
                if (process.env.UPDATE_TEST_MODE === 'true' || process.env.NODE_ENV === 'development') {
                    console.log('[UpdateManager] Test/Dev mode: Please restart the server manually');
                    console.log('[UpdateManager] Exiting in 3 seconds...');
                    setTimeout(() => {
                        process.exit(0);
                    }, 3000);
                    return;
                }
                
                // For production deployments, try various restart methods
                try {
                    // Try systemctl first (Linux with systemd)
                    await execAsync('sudo systemctl restart pulse');
                    console.log('[UpdateManager] Restarted via systemctl');
                } catch (error) {
                    try {
                        // Try pm2 restart
                        await execAsync('pm2 restart pulse');
                        console.log('[UpdateManager] Restarted via pm2');
                    } catch (error) {
                        // Last resort: exit and hope something restarts us
                        console.log('[UpdateManager] No restart mechanism found, exiting...');
                        process.exit(0);
                    }
                }
            }, 2000);

            // Cleanup
            await fs.rm(path.join(__dirname, '..', 'temp'), { recursive: true, force: true });

            return {
                success: true,
                message: 'Update applied successfully. The application will restart automatically.'
            };

        } catch (error) {
            console.error('[UpdateManager] Error applying update:', error.message);
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