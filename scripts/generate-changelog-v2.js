#!/usr/bin/env node

// User-focused changelog generation
const { execSync } = require('child_process');
const fs = require('fs');

// Get environment variables
const prevTag = process.env.PREV_TAG || 'v0.0.0';
const newVersion = process.env.NEW_VERSION || '0.0.0';
const bumpType = process.env.BUMP_TYPE || 'patch';
const isRC = newVersion.includes('-rc');

console.log('📝 Generating changelog for', newVersion);
console.log('📊 Analyzing changes since', prevTag);

// For RC releases, always compare against last stable (not last RC)
let compareTag = prevTag;
if (isRC && prevTag.includes('-rc')) {
  try {
    // Find the last stable release
    const lastStable = execSync('git tag -l "v*" | grep -v "rc\\|alpha\\|beta" | sort -V | tail -1', { 
      encoding: 'utf8',
      shell: '/bin/bash'
    }).trim();
    if (lastStable) {
      compareTag = lastStable;
      console.log('🔄 RC release: comparing against last stable', compareTag);
    }
  } catch (e) {
    console.warn('Could not find last stable release, using', prevTag);
  }
}

// Get commits
let commits = [];
try {
  const gitCmd = compareTag === 'v0.0.0' 
    ? 'git log HEAD --pretty=format:"%h %s" --no-merges' 
    : `git log ${compareTag}..HEAD --pretty=format:"%h %s" --no-merges`;
  
  const gitLog = execSync(gitCmd, { encoding: 'utf8' });
  commits = gitLog.trim().split('\n').filter(line => {
    if (!line.trim()) return false;
    const msg = line.toLowerCase();
    // Filter out automated and internal commits
    return !line.includes('🤖 Generated with') && 
           !line.includes('chore: release v') &&
           !line.includes('chore: bump version') &&
           !msg.includes('merge pull request') &&
           !msg.includes('merge branch') &&
           !msg.includes('claude.md') &&
           !msg.includes('readme') &&
           !msg.includes('changelog');
  });
} catch (e) {
  console.warn('Could not get git log:', e.message);
  commits = [];
}

// Categorize commits
const changes = {
  features: [],
  fixes: [],
  improvements: [],
  security: []
};

// Enhanced commit parsing with user-friendly descriptions
commits.forEach(line => {
  const [hash, ...msgParts] = line.split(' ');
  const message = msgParts.join(' ');
  const lower = message.toLowerCase();
  
  // Security fixes
  if (lower.includes('security') || lower.includes('vulnerability') || lower.includes('cve')) {
    changes.security.push(formatUserMessage(message, 'security'));
  }
  // Features
  else if (message.startsWith('feat:') || message.startsWith('feat(')) {
    const userMsg = formatUserMessage(message, 'feature');
    if (userMsg) changes.features.push(userMsg);
  }
  // Fixes
  else if (message.startsWith('fix:') || message.startsWith('fix(')) {
    const userMsg = formatUserMessage(message, 'fix');
    if (userMsg) changes.fixes.push(userMsg);
  }
  // Improvements (refactor, perf, style that affects users)
  else if (isUserFacingChange(message)) {
    const userMsg = formatUserMessage(message, 'improvement');
    if (userMsg) changes.improvements.push(userMsg);
  }
});

// Format commit messages for users
function formatUserMessage(commit, type) {
  // Remove conventional commit prefixes
  let msg = commit.replace(/^(feat|fix|refactor|perf|style|docs|test|chore)(\([^)]*\))?!?:\s*/i, '');
  
  // Skip internal/technical commits
  const lower = msg.toLowerCase();
  if (lower.includes('lint') || lower.includes('eslint') || 
      lower.includes('prettier') || lower.includes('typo') ||
      lower.includes('whitespace') || lower.includes('formatting') ||
      lower.includes('variable name') || lower.includes('console.log') ||
      lower.includes('debug') || msg.length < 10) {
    return null;
  }
  
  // User-friendly replacements
  msg = msg.replace(/PBS/g, 'Proxmox Backup Server');
  msg = msg.replace(/PVE/g, 'Proxmox VE');
  msg = msg.replace(/VM/g, 'virtual machine');
  msg = msg.replace(/LXC/g, 'container');
  msg = msg.replace(/API/g, 'connection');
  msg = msg.replace(/UI/g, 'interface');
  msg = msg.replace(/UX/g, 'user experience');
  
  // Improve readability
  msg = msg.replace(/^implement /i, '');
  msg = msg.replace(/^add /i, '');
  msg = msg.replace(/^update /i, '');
  msg = msg.replace(/^improve /i, '');
  msg = msg.replace(/^enhance /i, '');
  msg = msg.replace(/^fix /i, '');
  msg = msg.replace(/^resolve /i, '');
  
  // Capitalize first letter
  msg = msg.charAt(0).toUpperCase() + msg.slice(1);
  
  // Ensure it ends with proper punctuation
  if (!msg.match(/[.!?]$/)) msg += '.';
  
  return msg;
}

// Check if a change is user-facing
function isUserFacingChange(commit) {
  const lower = commit.toLowerCase();
  return (
    lower.includes('ui') || lower.includes('interface') ||
    lower.includes('display') || lower.includes('show') ||
    lower.includes('performance') || lower.includes('speed') ||
    lower.includes('memory') || lower.includes('cpu') ||
    lower.includes('chart') || lower.includes('graph') ||
    lower.includes('notification') || lower.includes('alert') ||
    lower.includes('backup') || lower.includes('restore') ||
    lower.includes('filter') || lower.includes('search') ||
    lower.includes('sort') || lower.includes('group')
  );
}

// Generate the changelog
let changelog = '';

// Title
if (isRC) {
  changelog += `# Release Candidate ${newVersion}\n\n`;
  changelog += `This release candidate includes all changes since the last stable release (${compareTag}).\n\n`;
} else {
  changelog += `# Release ${newVersion}\n\n`;
}

// Summary stats
const totalChanges = changes.features.length + changes.fixes.length + 
                    changes.improvements.length + changes.security.length;

if (totalChanges === 0) {
  changelog += 'This release includes minor updates and maintenance improvements.\n\n';
} else {
  const summary = [];
  if (changes.security.length > 0) summary.push(`${changes.security.length} security update${changes.security.length > 1 ? 's' : ''}`);
  if (changes.features.length > 0) summary.push(`${changes.features.length} new feature${changes.features.length > 1 ? 's' : ''}`);
  if (changes.fixes.length > 0) summary.push(`${changes.fixes.length} bug fix${changes.fixes.length > 1 ? 'es' : ''}`);
  if (changes.improvements.length > 0) summary.push(`${changes.improvements.length} improvement${changes.improvements.length > 1 ? 's' : ''}`);
  
  changelog += `This release includes ${summary.join(', ')}.\n\n`;
}

// Security updates (always first if present)
if (changes.security.length > 0) {
  changelog += '## 🔒 Security Updates\n\n';
  changes.security.forEach(change => {
    changelog += `- ${change}\n`;
  });
  changelog += '\n';
}

// New features
if (changes.features.length > 0) {
  changelog += '## ✨ New Features\n\n';
  changes.features.forEach(change => {
    changelog += `- ${change}\n`;
  });
  changelog += '\n';
}

// Bug fixes
if (changes.fixes.length > 0) {
  changelog += '## 🐛 Bug Fixes\n\n';
  changes.fixes.forEach(change => {
    changelog += `- ${change}\n`;
  });
  changelog += '\n';
}

// Improvements (only if significant)
if (changes.improvements.length > 3) {
  changelog += '## 💫 Improvements\n\n';
  // Show max 5 most significant improvements
  changes.improvements.slice(0, 5).forEach(change => {
    changelog += `- ${change}\n`;
  });
  if (changes.improvements.length > 5) {
    changelog += `- ...and ${changes.improvements.length - 5} more improvements.\n`;
  }
  changelog += '\n';
}

// Update instructions (just a link)
changelog += '## 📦 Installation & Updates\n\n';
changelog += 'For detailed update instructions, see our [Update Guide](https://github.com/rcourtman/Pulse/blob/main/docs/UPDATING.md).\n\n';

// Quick links for common update methods
changelog += '**Quick Update Commands:**\n';
changelog += '- Web Interface: Settings → System → Software Updates\n';
changelog += '- Script: `cd /opt/pulse/scripts && ./install-pulse.sh --update`\n';
changelog += '- Docker: `docker compose pull && docker compose up -d`\n';

if (isRC) {
  changelog += '\n---\n';
  changelog += '⚠️ **This is a pre-release for testing.** Report issues on [GitHub](https://github.com/rcourtman/Pulse/issues).\n';
}

// Write the changelog
fs.writeFileSync('CHANGELOG.md', changelog);
console.log('✅ Generated user-focused changelog (' + changelog.split('\n').length + ' lines)');