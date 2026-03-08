#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const TARGET_DIRS = [path.join(ROOT, 'src')];
const IGNORE_DIRS = new Set(['__tests__']);
const ALLOWLIST = new Set([
  'src/components/shared/sourcePlatformBadges.ts',
  'src/components/shared/workloadTypeBadges.ts',
  'src/components/Storage/storageSourceOptions.ts',
  'src/components/Infrastructure/resourceBadges.ts',
  'src/components/Settings/reportingResourceTypes.ts',
  'src/utils/canonicalResourceTypes.ts',
  'src/utils/reportableResourceTypes.ts',
  'src/utils/reportingResourceTypes.ts',
  'src/utils/resourceTypePresentation.ts',
  'src/utils/resourceBadgePresentation.ts',
  'src/utils/workloadTypePresentation.ts',
  'src/features/storageBackups/healthPresentation.ts',
  'src/utils/clusterEndpointPresentation.ts',
  'src/utils/agentCapabilityPresentation.ts',
  'src/utils/unifiedAgentStatusPresentation.ts',
  'src/utils/dashboardAlertPresentation.ts',
  'src/utils/temperature.ts',
  'src/utils/k8sStatusPresentation.ts',
  'src/utils/raidPresentation.ts',
  'src/utils/securityScorePresentation.ts',
]);

const PLATFORM_TOKENS = [
  'pve',
  'proxmox',
  'pbs',
  'pmg',
  'k8s',
  'kubernetes',
  'docker',
  'truenas',
  'unraid',
  'synology-dsm',
  'vmware-vsphere',
  'microsoft-hyperv',
  'aws',
  'azure',
  'gcp',
];

const DISPLAY_LABEL_TOKENS = [
  'PVE',
  'PBS',
  'PMG',
  'K8s',
  'Kubernetes',
  'Containers',
  'TrueNAS',
  'Unraid',
  'Synology',
  'vSphere',
  'Hyper-V',
  'AWS',
  'Azure',
  'GCP',
];

const AI_FINDING_SOURCE_TOKENS = [
  'threshold',
  'ai-patrol',
  'ai-chat',
  'anomaly',
  'correlation',
  'forecast',
];

const AI_FINDING_DISPLAY_TOKENS = [
  'Alert',
  'Pulse Patrol',
  'Pulse Assistant',
  'Anomaly',
  'Correlation',
  'Forecast',
];

const LICENSE_TIER_TOKENS = [
  'free',
  'relay',
  'pro',
  'pro_plus',
  'pro_annual',
  'lifetime',
  'cloud',
  'msp',
  'enterprise',
];

const LICENSE_FEATURE_TOKENS = [
  'ai_patrol',
  'mobile_app',
  'push_notifications',
  'multi_tenant',
  'advanced_reporting',
  'agent_profiles',
];

const RESOURCE_TYPE_TOKENS = [
  'agent',
  'docker-host',
  'k8s-cluster',
  'k8s-node',
  'vm',
  'system-container',
  'app-container',
  'pod',
  'storage',
  'truenas',
  'pbs',
  'pmg',
  'dataset',
  'pool',
  'datastore',
  'physical_disk',
  'proxmox-vm',
  'proxmox-lxc',
  'docker-container',
  'truenas-dataset',
];

const RESOURCE_TYPE_LABEL_TOKENS = [
  'Agent',
  'Container Runtime',
  'K8s Cluster',
  'K8s Node',
  'VM',
  'Container',
  'Storage',
  'TrueNAS',
  'PBS',
  'PMG',
  'Dataset',
  'Pool',
  'Datastore',
  'Physical Disk',
  'LXC',
  'Guest',
  'PVC',
  'Velero',
  'Replication',
];

const STORAGE_HEALTH_TOKENS = ['healthy', 'warning', 'critical', 'offline', 'unknown'];

const findings = [];

function toRelative(absPath) {
  return path.relative(ROOT, absPath).replaceAll(path.sep, '/');
}

function lineForIndex(content, index) {
  let line = 1;
  for (let i = 0; i < index; i += 1) {
    if (content[i] === '\n') line += 1;
  }
  return line;
}

function collectFiles(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    if (IGNORE_DIRS.has(entry.name)) continue;
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...collectFiles(fullPath));
      continue;
    }
    if (!entry.isFile()) continue;
    if (!/\.(ts|tsx)$/.test(entry.name)) continue;
    files.push(fullPath);
  }

  return files;
}

function containsAny(content, patterns) {
  return patterns.some((pattern) => content.includes(pattern));
}

function pushMatch(relativePath, content, index, rule, message) {
  findings.push({
    file: relativePath,
    line: lineForIndex(content, index),
    rule,
    message,
  });
}

const HELPER_RULES = [
  {
    rule: 'canonical-source/no-local-format-source-label',
    regex: /\b(?:const|function)\s+formatSourceLabel\b/g,
    message:
      'Do not define local source/platform label formatters in component code. Use shared sourcePlatformBadges helpers.',
  },
  {
    rule: 'canonical-source/no-local-platform-text-class',
    regex: /\b(?:const|function)\s+platformTextClass\b/g,
    message:
      'Do not define local source/platform style helpers in component code. Use shared sourcePlatformBadges helpers.',
  },
  {
    rule: 'canonical-type/no-local-format-type',
    regex: /\b(?:const|function)\s+formatType\b/g,
    message:
      'Do not define local resource type formatters in component or page code. Use shared resourceTypePresentation helpers.',
  },
  {
    rule: 'canonical-type/no-local-format-source-type',
    regex: /\b(?:const|function)\s+formatSourceType\b/g,
    message:
      'Do not define local source-type formatters in component or page code. Use shared sourceTypePresentation helpers.',
  },
  {
    rule: 'canonical-source/no-nonrender-imports-from-badge-component',
    regex:
      /import\s*\{([\s\S]*?(?:getSourcePlatformLabel|normalizeSourcePlatformKey)[\s\S]*?)\}\s*from\s*['"]@\/components\/shared\/sourcePlatformBadges['"]/g,
    message:
      'Do not import canonical source/platform labels or normalization from the badge component. Use @/utils/sourcePlatforms for non-rendering logic.',
  },
  {
    rule: 'canonical-source/no-imports-from-storage-component-shim',
    regex:
      /import\s*\{[\s\S]*?\}\s*from\s*['"]@\/components\/Storage\/storageSourceOptions['"]/g,
    message:
      'Do not import storage source normalization from the Storage component shim. Use @/utils/storageSources instead.',
  },
  {
    rule: 'canonical-source/no-local-storage-source-presentation-helper',
    regex:
      /\b(?:const|function)\s+(?:toneForKey|getStorageSourcePresentation|getStorageSourcePreset)\b/g,
    message:
      'Do not define local storage source presentation helpers in component code. Use @/utils/storageSources instead.',
  },
  {
    rule: 'canonical-storage/no-local-health-presentation-helper',
    regex:
      /\b(?:const|function)\s+(?:getStorageHealthPresentation|getHealthDotClass|getHealthCountClass|healthDotClass|healthCountClass)\b/g,
    message:
      'Do not define local storage health presentation helpers in component code. Use the shared storage health presentation utility instead.',
  },
  {
    rule: 'canonical-storage/no-local-ceph-health-helper',
    regex: /\b(?:const|function)\s+getHealthInfo\b/g,
    message:
      'Do not define local Ceph health presentation helpers in page code. Use the shared storage domain Ceph health helpers instead.',
  },
  {
    rule: 'canonical-type/no-nonrender-imports-from-workload-badge-component',
    regex:
      /import\s*\{([\s\S]*?(?:getWorkloadTypePresentation|normalizeWorkloadTypePresentationKey|getWorkloadTypeLabel)[\s\S]*?)\}\s*from\s*['"]@\/components\/shared\/workloadTypeBadges['"]/g,
    message:
      'Do not import canonical workload-type presentation from the badge component. Use @/utils/workloadTypePresentation for non-rendering logic.',
  },
  {
    rule: 'canonical-type/no-imports-from-resource-badge-component',
    regex:
      /import\s*\{[\s\S]*?(?:getPlatformBadge|getSourceBadge|getTypeBadge|getUnifiedSourceBadges|getContainerRuntimeBadge|ResourceBadge)[\s\S]*?\}\s*from\s*['"]@\/components\/Infrastructure\/resourceBadges['"]/g,
    message:
      'Do not import canonical resource badge presentation from the component shim. Use @/utils/resourceBadgePresentation for non-rendering logic.',
  },
  {
    rule: 'canonical-type/no-local-canonical-resource-type-list',
    regex: /\bconst\s+CANONICAL_RESOURCE_TYPES\s*=\s*\[/g,
    message:
      'Do not define local canonical resource type lists in component code. Use @/utils/canonicalResourceTypes instead.',
  },
  {
    rule: 'canonical-type/no-imports-from-reporting-type-component',
    regex:
      /import\s*\{[\s\S]*?toReportingResourceType[\s\S]*?\}\s*from\s*['"](?:\.\/reportingResourceTypes|@\/components\/Settings\/reportingResourceTypes)['"]/g,
    message:
      'Do not import reporting resource type translation from the Settings component shim. Use @/utils/reportingResourceTypes instead.',
  },
  {
    rule: 'canonical-type/no-local-reportable-resource-policy',
    regex:
      /\b(?:const\s+REPORTABLE_RESOURCE_TYPES\s*=\s*new Set|function\s+normalizeType\s*\(|function\s+matchesTypeFilter\s*\()/g,
    message:
      'Do not define local reportable resource type policy in component code. Use @/utils/reportableResourceTypes instead.',
  },
  {
    rule: 'canonical-settings/no-local-agent-capability-presentation',
    regex: /\b(?:const|function)\s+(?:getCapabilityLabel|getCapabilityBadgeClass)\b/g,
    message:
      'Do not define local unified-agent capability presentation in component code. Use @/utils/agentCapabilityPresentation instead.',
  },
  {
    rule: 'canonical-status/no-local-connected-status-helper',
    regex: /\b(?:const|function)\s+connectedFromStatus\b/g,
    message:
      'Do not define local connected-status helpers in component code. Use @/utils/status instead.',
  },
  {
    rule: 'canonical-settings/no-local-unified-agent-status-pill',
    regex:
      /MONITORING_STOPPED_STATUS_LABEL[\s\S]{0,500}\b(?:const|function)\s+statusBadgeClass\b|\b(?:const|function)\s+statusBadgeClass\b[\s\S]{0,500}MONITORING_STOPPED_STATUS_LABEL/g,
    message:
      'Do not define local unified-agent status pill helpers in component code. Use @/utils/unifiedAgentStatusPresentation instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-problem-status-variant',
    regex:
      /\b(?:const|function)\s+statusVariant\s*\(\s*pr\s*:\s*ProblemResource\s*\)/g,
    message:
      'Do not define local dashboard problem-resource status helpers in page code. Use @/utils/problemResourcePresentation instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-alerts-tone',
    regex: /\b(?:const|function)\s+alertsTone\b/g,
    message:
      'Do not define local dashboard alert summary tone helpers in page code. Use @/utils/dashboardAlertPresentation instead.',
  },
  {
    rule: 'canonical-temperature/no-local-temperature-tone',
    regex: /\b(?:const|function)\s+getTemperatureTone\b/g,
    message:
      'Do not define local temperature tone helpers in component code. Use @/utils/temperature instead.',
  },
  {
    rule: 'canonical-service-health/no-local-service-health-status-tone',
    regex:
      /\b(?:const|function)\s+statusTone\b[\s\S]{0,500}normalized\s*===\s*['"]online['"][\s\S]{0,220}normalized\s*===\s*['"]warning['"][\s\S]{0,220}normalized\s*===\s*['"]offline['"]/g,
    message:
      'Do not define local online/warning/offline service-health tone helpers in component code. Use shared status/service health presentation utilities instead.',
  },
  {
    rule: 'canonical-service-health/no-local-health-tone-class',
    regex: /\bexport\s+const\s+healthToneClass\b|\bexport\s+const\s+normalizeHealthLabel\b/g,
    message:
      'Do not define local health tone/label helpers in mapper code. Use @/utils/serviceHealthPresentation instead.',
  },
  {
    rule: 'canonical-k8s/no-local-namespace-status-tone',
    regex:
      /\b(?:const|function)\s+statusTone\b[\s\S]{0,320}counts\.offline[\s\S]{0,220}counts\.warning[\s\S]{0,220}counts\.online/g,
    message:
      'Do not define local Kubernetes namespace aggregate status tone helpers in component code. Use @/utils/k8sStatusPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-raid-presentation-helpers',
    regex: /\b(?:const|function)\s+(?:raidStateVariant|raidStateTextClass|deviceToneClass)\b/g,
    message:
      'Do not define local RAID state/device presentation helpers in component code. Use @/utils/raidPresentation instead.',
  },
];

const MAP_RULES = [
  {
    rule: 'canonical-source/no-local-source-label-map',
    regex:
      /\b(?:const|let|var)\s+(?:sourceLabels|platformLabels|sourcePlatformLabels)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local source/platform label maps in component code. Normalize canonically in adapters and render through shared sourcePlatformBadges helpers.',
    validate: (snippet) =>
      containsAny(snippet, PLATFORM_TOKENS) && containsAny(snippet, DISPLAY_LABEL_TOKENS),
  },
  {
    rule: 'canonical-source/no-local-source-style-map',
    regex:
      /\b(?:const|let|var)\s+(?:sourceClasses|platformClasses|sourcePlatformClasses)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local source/platform style maps in component code. Use shared sourcePlatformBadges helpers.',
    validate: (snippet) =>
      containsAny(snippet, PLATFORM_TOKENS) && /(?:bg-|text-)/.test(snippet),
  },
  {
    rule: 'canonical-source/no-local-storage-source-preset-map',
    regex:
      /\b(?:const|let|var)\s+(?:STORAGE_SOURCE_PRESETS|storageSourcePresets|sourceFilterPresets)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local storage source presentation maps in component code. Use @/utils/storageSources instead.',
    validate: (snippet) =>
      containsAny(snippet, PLATFORM_TOKENS) && /tone\s*:/.test(snippet),
  },
  {
    rule: 'canonical-storage/no-local-health-presentation-map',
    regex:
      /\b(?:const|let|var)\s+(?:HEALTH_DOT|healthDots|healthDotMap|healthPresentation|storageHealthPresentation)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local storage health presentation maps in component code. Use the shared storage health presentation utility instead.',
    validate: (snippet) =>
      containsAny(snippet, STORAGE_HEALTH_TOKENS) && /(?:bg-|text-)/.test(snippet),
  },
  {
    rule: 'canonical-source/no-local-source-option-array',
    regex:
      /\b(?:const|let|var)\s+(?:sourceOptions|providerOptions)\s*=\s*\[([\s\S]*?)\];?/g,
    message:
      'Do not define local canonical source/provider option arrays in page code. Use @/utils/sourcePlatformOptions instead.',
    validate: (snippet) => containsAny(snippet, PLATFORM_TOKENS),
  },
  {
    rule: 'canonical-ai/no-local-finding-source-map',
    regex:
      /\b(?:const|let|var)\s+(?:sourceLabels|sourceColors|loopStateColors|lifecycleLabels)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local AI finding presentation maps in component code. Use @/utils/aiFindingPresentation instead.',
    validate: (snippet) =>
      containsAny(snippet, AI_FINDING_SOURCE_TOKENS) ||
      containsAny(snippet, AI_FINDING_DISPLAY_TOKENS),
  },
  {
    rule: 'canonical-ai/no-local-provider-display-map',
    regex: /\b(?:const|let|var)\s+PROVIDER_DISPLAY_NAMES\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local AI provider display-name maps in component code. Use @/utils/aiProviderPresentation instead.',
    validate: (snippet) =>
      containsAny(snippet, ['anthropic', 'openai', 'openrouter', 'deepseek', 'gemini', 'ollama']),
  },
  {
    rule: 'canonical-ai/no-legacy-provider-name-import',
    regex: /import\s*\{[\s\S]*?\bPROVIDER_NAMES\b[\s\S]*?\}\s*from\s*['"]@\/types\/ai['"]/g,
    message:
      'Do not import legacy AI provider display names from @/types/ai in UI code. Use @/utils/aiProviderPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-raw-provider-display-map-import',
    regex:
      /import\s*\{[\s\S]*?\bAI_PROVIDER_DISPLAY_NAMES\b[\s\S]*?\}\s*from\s*['"]@\/utils\/aiProviderPresentation['"]/g,
    message:
      'Do not import the raw AI provider display-name map in UI code. Use getAIProviderDisplayName from @/utils/aiProviderPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-local-provider-health-presentation-helper',
    regex:
      /\b(?:const|function)\s+(?:getProviderHealthBadgeClass|getProviderHealthLabel)\b/g,
    message:
      'Do not define local AI provider health presentation helpers in component code. Use @/utils/aiProviderHealthPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-local-approval-risk-presentation-helper',
    regex: /\b(?:const|function)\s+riskBadgeColor\b/g,
    message:
      'Do not define local approval risk badge helpers in component or page code. Use @/utils/approvalRiskPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-inline-approval-risk-badge-ternary',
    regex:
      /(?:riskLevel|risk_level)\s*===\s*['"]high['"][\s\S]{0,220}(?:riskLevel|risk_level)\s*===\s*['"]medium['"]/g,
    message:
      'Do not inline approval risk badge ternaries in component or page code. Use @/utils/approvalRiskPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-alerts/no-local-severity-badge-helper',
    regex: /\b(?:const|function)\s+severityBadgeClass\b/g,
    message:
      'Do not define local alert severity badge helpers in component or page code. Use @/utils/alertSeverityPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-recovery/no-local-outcome-badge-helper',
    regex: /\b(?:const|function)\s+outcomeBadgeClass\b/g,
    message:
      'Do not define local recovery outcome badge helpers in component or page code. Use @/utils/recoveryOutcomePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-recovery/no-local-normalize-outcome',
    regex: /\b(?:const|function)\s+normalizeOutcome\b/g,
    message:
      'Do not define local recovery outcome normalizers in component, page, or hook code. Use @/utils/recoveryOutcomePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-recovery/no-local-outcome-badge-map',
    regex: /\b(?:const|let|var)\s+OUTCOME_BADGE_CLASS\s*=\s*\{/g,
    message:
      'Do not define local recovery outcome badge maps in component or page code. Use @/utils/recoveryOutcomePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-inline-patrol-run-status-ternary',
    regex:
      /run\.status\s*===\s*['"]critical['"][\s\S]{0,260}run\.status\s*===\s*['"]issues_found['"]/g,
    message:
      'Do not inline patrol run status badge ternaries in component code. Use @/utils/patrolRunPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-inline-tool-call-result-badge-ternary',
    regex: /call\.success\s*\?[\s\S]{0,180}bg-green-100[\s\S]{0,180}bg-red-100/g,
    message:
      'Do not inline tool call result badge color ternaries in component code. Use @/utils/patrolRunPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-local-tool-execution-status-helper',
    regex: /\b(?:const|function)\s+statusColor\b/g,
    message:
      'Do not define local tool execution status color helpers in AI components. Use @/utils/patrolRunPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-status/no-inline-offline-degraded-badge-ternary',
    regex:
      /problem\s*===\s*['"]Offline['"][\s\S]{0,220}problem\s*===\s*['"]Degraded['"]/g,
    message:
      'Do not inline Offline/Degraded badge ternaries in component or page code. Use the shared status presentation helpers instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-ai/no-inline-finding-severity-count-badge-ternary',
    regex:
      /criticalFindings\s*>\s*0[\s\S]{0,200}bg-red-100[\s\S]{0,200}bg-amber-100/g,
    message:
      'Do not inline finding severity count badge ternaries in page code. Use shared AI finding severity presentation helpers instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-status/no-local-semantic-tone-maps',
    regex: /\b(?:const|let|var)\s+(?:statusColors|iconColors)\s*=\s*\{/g,
    message:
      'Do not define local semantic tone maps in component code. Use @/utils/semanticTonePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-settings/no-inline-cluster-endpoint-status-color',
    regex:
      /endpoint\.online\s*&&\s*pulseStatus\s*===\s*['"]reachable['"][\s\S]{0,260}pulseStatus\s*===\s*['"]unreachable['"]/g,
    message:
      'Do not inline cluster endpoint status-color logic in settings UI. Use @/utils/clusterEndpointPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-license/no-local-tier-or-feature-label-map',
    regex:
      /\b(?:const|let|var)\s+(?:TIER_LABELS|FEATURE_LABELS|tierLabel|featureMinTier)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local license tier or feature label maps in component code. Use @/utils/licensePresentation instead.',
    validate: (snippet) =>
      containsAny(snippet, LICENSE_TIER_TOKENS) || containsAny(snippet, LICENSE_FEATURE_TOKENS),
  },
  {
    rule: 'canonical-license/no-local-subscription-status-presentation',
    regex:
      /\b(?:const|function)\s+(?:statusLabel|statusTone)\b[\s\S]{0,900}subscriptionState\(\)/g,
    message:
      'Do not define local subscription status label/tone helpers in component code. Use @/utils/licensePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-security/no-local-security-score-presentation',
    regex:
      /\b(?:const|function)\s+(?:scoreTone|scoreLabel)\b[\s\S]{0,1200}securityScore\(\)/g,
    message:
      'Do not define local security score tone/label helpers in component code. Use @/utils/securityScorePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-type/no-local-type-label-map',
    regex:
      /\b(?:const|let|var)\s+(?:SUBJECT_TYPE_LABELS|subjectTypeLabels|typeLabels|resourceTypeLabels)\s*=\s*\{([\s\S]*?)\n\};?/g,
    message:
      'Do not define local resource or subject type label maps in component or page code. Use shared resourceTypePresentation helpers.',
    validate: (snippet) =>
      containsAny(snippet, RESOURCE_TYPE_TOKENS) && containsAny(snippet, RESOURCE_TYPE_LABEL_TOKENS),
  },
];

for (const dir of TARGET_DIRS) {
  for (const filePath of collectFiles(dir)) {
    const relativePath = toRelative(filePath);
    if (ALLOWLIST.has(relativePath)) continue;

    const content = fs.readFileSync(filePath, 'utf8');

    for (const { rule, regex, message } of HELPER_RULES) {
      for (const match of content.matchAll(regex)) {
        pushMatch(relativePath, content, match.index ?? 0, rule, message);
      }
    }

    for (const { rule, regex, message, validate } of MAP_RULES) {
      for (const match of content.matchAll(regex)) {
        const snippet = match[1] || '';
        if (!validate(snippet)) continue;
        pushMatch(relativePath, content, match.index ?? 0, rule, message);
      }
    }
  }
}

if (findings.length === 0) {
  console.log('Canonical platform audit passed with no findings.');
  process.exit(0);
}

console.error('Canonical platform audit findings:');
for (const finding of findings) {
  console.error(`- ${finding.file}:${finding.line} [${finding.rule}] ${finding.message}`);
}

process.exit(1);
