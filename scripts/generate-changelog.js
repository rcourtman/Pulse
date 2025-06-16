#!/usr/bin/env node

// Standalone changelog generation script
const { execSync } = require('child_process');
const fs = require('fs');

// Get environment variables passed from GitHub Actions
const prevTag = process.env.PREV_TAG || 'v0.0.0';
const newVersion = process.env.NEW_VERSION || '0.0.0';
const bumpType = process.env.BUMP_TYPE || 'patch';
const reasoning = process.env.REASONING || 'Manual release';

console.log('🔍 Analyzing commits from', prevTag, 'to HEAD');

// Get commits since previous stable release
let commits = [];
try {
  const gitCmd = prevTag === 'v0.0.0' ? 'git log HEAD --oneline --no-merges' : `git log ${prevTag}..HEAD --oneline --no-merges`;
  const gitLog = execSync(gitCmd, { encoding: 'utf8' });
  commits = gitLog.trim().split('\n').filter(line => {
    if (!line.trim()) return false;
    const lowerLine = line.toLowerCase();
    // Exclude version bumps, automated commits, debug commits, and internal changes
    return !line.includes('🤖 Generated with') && 
           !line.includes('chore: release v') &&
           !line.includes('chore: bump version') &&
           !line.includes('bump version to') &&
           !lowerLine.includes('merge pull request') &&
           !lowerLine.includes('merge branch') &&
           !line.includes('resolve: merge conflicts') &&
           !lowerLine.startsWith('debug:') &&
           !lowerLine.startsWith('cleanup:') &&
           !lowerLine.includes('debug logging') &&
           !lowerLine.includes('add logging') &&
           !lowerLine.includes('remove logging') &&
           !lowerLine.includes('console.log') &&
           !lowerLine.includes('add console') &&
           !lowerLine.includes('streamline claude.md') &&
           !lowerLine.includes('consolidate codebase') &&
           !lowerLine.includes('add instructions') &&
           !lowerLine.includes('add changelog generation notes');
  });
} catch (e) {
  console.warn('Could not get git log:', e.message);
  commits = [];
}

console.log('📊 Found', commits.length, 'commits to analyze');

// Analyze commit types
const analysis = {
  breaking: [],
  features: [],
  fixes: [],
  other: []
};

commits.forEach(commit => {
  const message = commit.replace(/^[a-f0-9]+\s+/, ''); // Remove hash
  console.log('Analyzing:', message);
  
  if (message.includes('!:') || message.toLowerCase().includes('breaking')) {
    analysis.breaking.push(message);
  } else if (message.startsWith('feat:') || message.startsWith('feat(')) {
    analysis.features.push(message);
  } else if (message.startsWith('fix:') || message.startsWith('fix(')) {
    analysis.fixes.push(message);
  } else {
    analysis.other.push(message);
  }
});

console.log('📊 Analysis results:', {
  totalCommits: commits.length,
  breaking: analysis.breaking.length,
  features: analysis.features.length,
  fixes: analysis.fixes.length,
  other: analysis.other.length
});

// Generate meaningful changelog
let changelog = '## What\'s Changed\n\n';

// Add version summary
changelog += '**' + bumpType.charAt(0).toUpperCase() + bumpType.slice(1) + ' release** with ' + commits.length + ' commit' + (commits.length !== 1 ? 's' : '') + ' since ' + prevTag + '\n\n';

if (analysis.breaking.length > 0) {
  changelog += '### 💥 Breaking Changes\n';
  analysis.breaking.forEach(commit => {
    changelog += '- ' + commit.replace(/^(fix|feat|docs|style|refactor|test|chore)!?:\s*/, '') + '\n';
  });
  changelog += '\n';
}

if (analysis.features.length > 0) {
  changelog += '### ✨ New Features\n';
  analysis.features.forEach(commit => {
    let cleanCommit = commit.replace(/^feat(\([^)]*\))?:\s*/, '');
    // Make commit messages more user-friendly
    cleanCommit = cleanCommit.replace('implement ', '');
    cleanCommit = cleanCommit.replace('add ', '');
    cleanCommit = cleanCommit.charAt(0).toUpperCase() + cleanCommit.slice(1);
    changelog += '- ' + cleanCommit + '\n';
  });
  changelog += '\n';
}

if (analysis.fixes.length > 0) {
  changelog += '### 🐛 Bug Fixes\n';
  analysis.fixes.forEach(commit => {
    let cleanCommit = commit.replace(/^fix(\([^)]*\))?:\s*/, '');
    // Make commit messages more user-friendly
    cleanCommit = cleanCommit.replace('resolve ', '');
    cleanCommit = cleanCommit.replace('fix ', '');
    cleanCommit = cleanCommit.charAt(0).toUpperCase() + cleanCommit.slice(1);
    changelog += '- ' + cleanCommit + '\n';
  });
  changelog += '\n';
}

if (analysis.other.length > 0) {
  changelog += '### 🔧 Improvements\n';
  analysis.other.forEach(commit => {
    let cleanCommit = commit.replace(/^(docs|style|refactor|test|chore)(\([^)]*\))?:\s*/, '');
    // Skip very technical commits that aren't user-facing
    const lowerCommit = cleanCommit.toLowerCase();
    if (!cleanCommit.includes('resolve:') && 
        !lowerCommit.includes('variable name conflict') &&
        !lowerCommit.includes('tracking-wider css') &&
        !lowerCommit.includes('addmobileclick') &&
        !lowerCommit.includes('expanded state') &&
        !lowerCommit.includes('scrollbar') &&
        !lowerCommit.includes('viewport transition') &&
        cleanCommit.length > 15) {
      changelog += '- ' + cleanCommit + '\n';
    }
  });
  changelog += '\n';
}

if (analysis.features.length > 0 || analysis.fixes.length > 0) {
  changelog += '---\n\n';
  changelog += `**${analysis.features.length} new features** • **${analysis.fixes.length} bug fixes** • ${commits.length} total changes\n\n`;
}

changelog += '## 🚀 Installation & Update Instructions\n';
changelog += '\n';

changelog += '### 🔄 **Existing Users - Update Instructions**\n';
changelog += '\n';
changelog += '#### 🖥️ **Web Interface Update (Available from v3.27.2+)**\n';
changelog += '**✨ New! One-click updates from your browser:**\n';
changelog += '1. Open your Pulse web interface\n';
changelog += '2. Go to **Settings** → **System** tab → **Software Updates**\n';
changelog += '3. Select **"Stable"** channel if not already selected\n';
changelog += '4. Click **"Check for Updates"** or wait for automatic checking\n';
changelog += '5. Click **"Apply Update"** to install v' + newVersion + '\n';
changelog += '6. The interface will automatically refresh after the update completes\n';
changelog += '\n';
changelog += '> **Note**: Web updates are automatically disabled for Docker deployments\n';
changelog += '\n';
changelog += '#### 🛠️ **Script-Based Update**\n';
changelog += '**For LXC, VMs, and regular installations:**\n';
changelog += '```bash\n';
changelog += '# Update to latest stable version\n';
changelog += 'cd /opt/pulse/scripts\n';
changelog += './install-pulse.sh --update\n';
changelog += '\n';
changelog += '# Or update to this specific version\n';
changelog += './install-pulse.sh --update --version v' + newVersion + '\n';
changelog += '```\n';
changelog += '\n';
changelog += '#### 🐳 **Docker Update**\n';
changelog += '```bash\n';
changelog += '# Pull latest stable release\n';
changelog += 'docker pull rcourtman/pulse:latest\n';
changelog += 'docker pull rcourtman/pulse:v' + newVersion + '\n';
changelog += '\n';
changelog += '# Update with docker-compose\n';
changelog += 'docker compose down && docker compose pull && docker compose up -d\n';
changelog += '\n';
changelog += '# Or update manually\n';
changelog += 'docker stop pulse && docker rm pulse\n';
changelog += 'docker run -d --name pulse -p 7655:7655 -v pulse-config:/app/config rcourtman/pulse:latest\n';
changelog += '```\n';
changelog += '\n';
changelog += '#### 🏠 **Proxmox LXC (Community Script)**\n';
changelog += '```bash\n';
changelog += '# Update existing LXC container\n';
changelog += 'bash -c "$(wget -qLO - https://github.com/community-scripts/ProxmoxVE/raw/main/ct/pulse.sh)"\n';
changelog += '# Choose "Update" option when prompted\n';
changelog += '```\n';
changelog += '\n';

changelog += '### 📥 **New Users - Fresh Installation**\n';
changelog += '\n';
changelog += '**Automated Installer (Recommended):**\n';
changelog += '```bash\n';
changelog += 'curl -sL https://raw.githubusercontent.com/rcourtman/Pulse/main/scripts/install-pulse.sh | bash\n';
changelog += '```\n';
changelog += '\n';
changelog += '**Docker:**\n';
changelog += '```bash\n';
changelog += 'docker run -d \\\n';
changelog += '  --name pulse \\\n';
changelog += '  -p 7655:7655 \\\n';
changelog += '  -v pulse-config:/app/config \\\n';
changelog += '  rcourtman/pulse:latest\n';
changelog += '```\n';
changelog += '\n';
changelog += '**Manual Download:**\n';
changelog += '```bash\n';
changelog += 'wget https://github.com/rcourtman/Pulse/releases/download/v' + newVersion + '/pulse-v' + newVersion + '.tar.gz\n';
changelog += 'tar -xzf pulse-v' + newVersion + '.tar.gz\n';
changelog += 'cd pulse-v' + newVersion + '\n';
changelog += 'npm install --production\n';
changelog += 'npm start\n';
changelog += '```\n';
changelog += '\n';
changelog += '🤖 *This release was automatically generated from the develop branch*';

console.log('✅ Generated changelog with', changelog.split('\n').length, 'lines');

// Write to file for GitHub release
fs.writeFileSync('CHANGELOG.md', changelog);
console.log('📝 Changelog written to CHANGELOG.md');