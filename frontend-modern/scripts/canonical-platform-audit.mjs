#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const TARGET_DIRS = [path.join(ROOT, 'src')];
const IGNORE_DIRS = new Set(['__tests__']);
const ALLOWLIST = new Set([
  'src/components/shared/sourcePlatformBadges.ts',
  'src/components/shared/workloadTypeBadges.ts',
  'src/utils/emptyStatePresentation.ts',
  'src/components/Storage/storagePageState.ts',
  'src/components/Storage/storageSourceOptions.ts',
  'src/components/Storage/useStorageExpansionState.ts',
  'src/components/Storage/useStorageResourceHighlight.ts',
  'src/components/Infrastructure/resourceBadges.ts',
  'src/components/Settings/reportingResourceTypes.ts',
  'src/utils/canonicalResourceTypes.ts',
  'src/utils/reportableResourceTypes.ts',
  'src/utils/reportingResourceTypes.ts',
  'src/utils/reportingPresentation.ts',
  'src/utils/resourceTypePresentation.ts',
  'src/utils/resourceBadgePresentation.ts',
  'src/utils/workloadTypePresentation.ts',
  'src/features/storageBackups/diskPresentation.ts',
  'src/features/storageBackups/diskDetailPresentation.ts',
  'src/features/storageBackups/cephRecordPresentation.ts',
  'src/features/storageBackups/healthPresentation.ts',
  'src/features/storageBackups/recordPresentation.ts',
  'src/features/storageBackups/storageModelCore.ts',
  'src/features/storageBackups/resourceStorageMapping.ts',
  'src/features/storageBackups/resourceStoragePresentation.ts',
  'src/features/storageBackups/storageAdapterCore.ts',
  'src/features/storageBackups/storageAlertState.ts',
  'src/features/storageBackups/cephSummaryPresentation.ts',
  'src/features/storageBackups/storagePoolDetailPresentation.ts',
  'src/features/storageBackups/storageDomain.ts',
  'src/features/storageBackups/storagePagePresentation.ts',
  'src/features/storageBackups/storagePageStatus.ts',
  'src/features/storageBackups/rowPresentation.ts',
  'src/features/storageBackups/groupPresentation.ts',
  'src/features/storageBackups/storageRowAlertPresentation.ts',
  'src/utils/clusterEndpointPresentation.ts',
  'src/utils/agentCapabilityPresentation.ts',
  'src/utils/unifiedAgentInventoryPresentation.ts',
  'src/utils/unifiedAgentStatusPresentation.ts',
  'src/utils/dashboardAlertPresentation.ts',
  'src/utils/dashboardEmptyStatePresentation.ts',
  'src/utils/dashboardStoragePresentation.ts',
  'src/utils/dashboardRecoveryPresentation.ts',
  'src/utils/temperature.ts',
  'src/utils/licensePresentation.ts',
  'src/utils/k8sStatusPresentation.ts',
  'src/utils/raidPresentation.ts',
  'src/utils/securityScorePresentation.ts',
  'src/utils/securityAuthPresentation.ts',
  'src/utils/serviceHealthPresentation.ts',
  'src/utils/aiExplorePresentation.ts',
  'src/utils/aiFindingPresentation.ts',
  'src/utils/discoveryPresentation.ts',
  'src/utils/aiSessionDiffPresentation.ts',
  'src/utils/aiQuickstartPresentation.ts',
  'src/utils/pmgPresentation.ts',
  'src/utils/pmgThreatPresentation.ts',
  'src/utils/pmgQueuePresentation.ts',
  'src/components/PMG/ServiceHealthBadge.tsx',
  'src/utils/relayPresentation.ts',
  'src/utils/deployStatusPresentation.ts',
  'src/utils/alertIncidentPresentation.ts',
  'src/utils/alertAdministrationPresentation.ts',
  'src/utils/alertHistoryPresentation.ts',
  'src/utils/alertOverviewPresentation.ts',
  'src/utils/alertDestinationsPresentation.ts',
  'src/utils/alertBulkEditPresentation.ts',
  'src/utils/alertEmailPresentation.ts',
  'src/utils/alertResourceTablePresentation.ts',
  'src/utils/alertWebhookPresentation.ts',
  'src/utils/alertActivationPresentation.ts',
  'src/utils/alertConfigPresentation.ts',
  'src/utils/alertFrequencyPresentation.ts',
  'src/utils/alertTabsPresentation.ts',
  'src/utils/alertGroupingPresentation.ts',
  'src/utils/alertSchedulePresentation.ts',
  'src/utils/configuredNodeCapabilityPresentation.ts',
  'src/utils/auditWebhookPresentation.ts',
  'src/utils/auditLogPresentation.ts',
  'src/utils/diagnosticsPresentation.ts',
  'src/utils/aiChatPresentation.ts',
  'src/utils/aiControlLevelPresentation.ts',
  'src/utils/aiCostPresentation.ts',
  'src/utils/aiSettingsPresentation.ts',
  'src/utils/aiPatrolSchedulePresentation.ts',
  'src/utils/agentProfileSuggestionPresentation.ts',
  'src/utils/agentProfilesPresentation.ts',
  'src/utils/nodeModalPresentation.ts',
  'src/utils/organizationRolePresentation.ts',
  'src/utils/organizationSettingsPresentation.ts',
  'src/utils/ssoProviderPresentation.ts',
  'src/utils/thresholdSliderPresentation.ts',
  'src/utils/recoveryArtifactModePresentation.ts',
  'src/utils/recoveryActionPresentation.ts',
  'src/utils/recoveryDatePresentation.ts',
  'src/utils/recoveryEmptyStatePresentation.ts',
  'src/utils/recoveryRecordPresentation.ts',
  'src/utils/recoveryFilterChipPresentation.ts',
  'src/utils/recoveryIssuePresentation.ts',
  'src/utils/recoverySummaryPresentation.ts',
  'src/utils/recoveryStatusPresentation.ts',
  'src/utils/recoveryTablePresentation.ts',
  'src/utils/recoveryTimelineChartPresentation.ts',
  'src/utils/recoveryTimelinePresentation.ts',
  'src/utils/remediationPresentation.ts',
  'src/utils/patrolEmptyStatePresentation.ts',
  'src/utils/patrolRunPresentation.ts',
  'src/utils/patrolSummaryPresentation.ts',
  'src/utils/rbacPresentation.ts',
  'src/utils/rbacPermissions.ts',
  'src/utils/systemLogsPresentation.ts',
  'src/utils/systemSettingsPresentation.ts',
  'src/utils/settingsShellPresentation.ts',
  'src/utils/updatesPresentation.ts',
  'src/utils/alertThresholdsPresentation.ts',
  'src/utils/environmentLockPresentation.ts',
  'src/utils/swarmPresentation.ts',
  'src/utils/k8sDeploymentPresentation.ts',
  'src/utils/k8sNamespacePresentation.ts',
  'src/components/shared/EnvironmentLockBadge.tsx',
  'src/utils/dashboardCompositionPresentation.ts',
  'src/utils/dashboardGuestPresentation.ts',
  'src/utils/dashboardMetricPresentation.ts',
  'src/utils/dashboardTrendPresentation.ts',
  'src/utils/deployFlowPresentation.ts',
  'src/utils/infrastructureEmptyStatePresentation.ts',
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
    rule: 'canonical-infrastructure/no-local-empty-state-copy',
    regex:
      /No infrastructure resources yet|No resources match filters|Add Proxmox VE nodes or install the Pulse agent on your infrastructure to start monitoring\.|Try adjusting the search, source, or status filters\.|Unable to load infrastructure|We couldn’t fetch unified resources\. Check connectivity or retry\./g,
    message:
      'Do not define local infrastructure empty-state copy in page code. Use @/utils/infrastructureEmptyStatePresentation instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-dashboard-empty-state-copy',
    regex:
      /No infrastructure hosts connected|No guests found|Install the Pulse agent to connect a host and unlock v6 infrastructure data, or add a Proxmox connection in Settings → Infrastructure → Proxmox\.|No guests match your current filters|No guests match your search |Loading dashboard data\.\.\.|Connecting to monitoring service|Reconnecting to monitoring service…|Real-time data is currently unavailable\. Showing last-known state\.|Real-time data is reconnecting\. Showing last-known state\.|Dashboard unavailable|Real-time dashboard data is currently unavailable\. Reconnect to try again\.|No resources yet|Once connected platforms report resources, your dashboard overview will appear here\./g,
    message:
      'Do not define local dashboard empty-state copy in component code. Use @/utils/dashboardEmptyStatePresentation instead.',
  },
  {
    rule: 'canonical-shared/no-local-empty-state-tone-maps',
    regex:
      /\bconst\s+iconBgClass\s*:\s*Record<EmptyStateTone,\s*string>\s*=|\bconst\s+titleToneClass\s*:\s*Record<EmptyStateTone,\s*string>\s*=|\bconst\s+descriptionToneClass\s*:\s*Record<EmptyStateTone,\s*string>\s*=/g,
    message:
      'Do not define local EmptyState tone maps in component code. Use @/utils/emptyStatePresentation instead.',
  },
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
    rule: 'canonical-storage/no-local-disk-presentation-helpers',
    regex:
      /\bfunction\s+hasSmartWarning\s*\(|\bconst\s+getDiskHealthStatus\s*=\s*\(|\bconst\s+getDiskRoleLabel\s*=\s*\(|\bconst\s+getDiskParentLabel\s*=\s*\(/g,
    message:
      'Do not define local physical-disk presentation helpers in component code. Use @/features/storageBackups/diskPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-record-presentation-helpers',
    regex:
      /\bconst\s+getRecordDetails\s*=\s*\(|\bconst\s+getRecordStringDetail\s*=\s*\(|\bconst\s+getRecordStringArrayDetail\s*=\s*\(|\bexport\s*\{\s*getStorageRecord[A-Za-z]+\s+as\s+getRecord[A-Za-z]+|(?:export\s+)?const\s+computeGroupStats\s*=\s*\(|\bexport\s+const\s+getRecord(?:NodeHints|Type|Content|Status|PlatformLabel|HostLabel|TopologyLabel|ProtectionLabel|IssueLabel|IssueSummary|ImpactSummary|ActionSummary|Shared|NodeLabel|UsagePercent|ZfsPool)\b/g,
    message:
      'Do not define or re-export local storage record presentation helpers in component code. Use @/features/storageBackups/recordPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-resource-storage-presentation-helpers',
    regex:
      /\bconst\s+canonicalPlatformKeyForResource\s*=\s*\(|\bconst\s+platformLabelForResource\s*=\s*\(|\bconst\s+topologyLabelForResource\s*=\s*\(|\bconst\s+issueLabelForResource\s*=\s*\(|\bconst\s+issueSummaryForResource\s*=\s*\(|\bconst\s+impactSummaryForResource\s*=\s*\(|\bconst\s+actionSummaryForResource\s*=\s*\(|\bconst\s+protectionLabelForResource\s*=\s*\(/g,
    message:
      'Do not define local resource-to-storage presentation helpers in adapter code. Use @/features/storageBackups/resourceStoragePresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-resource-storage-mapping-helpers',
    regex:
      /\btype\s+ResourceStorageMeta\s*=\s*\{|\bconst\s+normalizeStorageMeta\s*=\s*\(|\bconst\s+readResourceStorageMeta\s*=\s*\(|\bconst\s+resolveStorageContent\s*=\s*\(|\bconst\s+capabilitiesForStorage\s*=\s*\(|\bconst\s+categoryFromStorageType\s*=\s*\(/g,
    message:
      'Do not define local resource-to-storage mapping helpers in adapter code. Use @/features/storageBackups/resourceStorageMapping instead.',
  },
  {
    rule: 'canonical-storage/no-local-storage-adapter-core-helpers',
    regex:
      /\bconst\s+asNumberOrNull\s*=\s*\(|\bconst\s+dedupe\s*=\s*<|\bconst\s+normalizeIdentityPart\s*=\s*\(|\bconst\s+getStringArray\s*=\s*\(|\bconst\s+canonicalStorageIdentityKey\s*=\s*\(|\bconst\s+resolvePlatformFamily\s*=\s*\(|\bconst\s+fromSource\s*=\s*\(|\bconst\s+capacity\s*=\s*\(|\bconst\s+metricsTargetForResource\s*=\s*\(|\bconst\s+extractHealthTag\s*=\s*\(|\bconst\s+normalizeHealthValue\s*=\s*\(|\bconst\s+normalizeResourceHealth\s*=\s*\(/g,
    message:
      'Do not define local storage adapter core helpers in adapter code. Use @/features/storageBackups/storageAdapterCore instead.',
  },
  {
    rule: 'canonical-storage/no-local-storage-alert-state-helpers',
    regex:
      /\bconst\s+EMPTY_ALERT_STATE\s*=\s*\{|(?:export\s+)?type\s+StorageAlertRowState\s*=\s*\{|(?:const|function)\s+asAlertRecord\b|\bconst\s+severityWeight\s*=\s*\(|\bconst\s+mergeAlertState\s*=\s*\(|\bconst\s+getRecordAlertResourceIds\s*=\s*\(/g,
    message:
      'Do not define local storage alert-state helpers in component code. Use @/features/storageBackups/storageAlertState instead.',
  },
  {
    rule: 'canonical-storage/no-local-storage-page-state-helpers',
    regex:
      /\bconst\s+normalizeHealthFilter\s*=\s*\(|\bconst\s+normalizeSortKey\s*=\s*\(|\bconst\s+normalizeGroupKey\s*=\s*\(|\bconst\s+normalizeView\s*=\s*\(|\bconst\s+normalizeSortDirection\s*=\s*\(|\bconst\s+getStorageMetaBoolean\s*=\s*\(|\bconst\s+isRecordCeph\s*=\s*\(/g,
    message:
      'Do not define local storage page state helpers in the page component. Use ./storagePageState instead.',
  },
  {
    rule: 'canonical-storage/no-local-ceph-record-helpers',
    regex:
      /\bexport\s+const\s+isCephRecord\s*=\s*\(|\bexport\s+const\s+getCephClusterKeyFromRecord\s*=\s*\(|\bconst\s+getCephSummaryText\s*=\s*\(|\bconst\s+getCephPoolsText\s*=\s*\(/g,
    message:
      'Do not define local Ceph record helpers in component code. Use @/features/storageBackups/cephRecordPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-disk-detail-presentation-helpers',
    regex:
      /\bfunction\s+attrColor\s*\(|health\(\)\s*!=\s*null[\s\S]{0,160}bg-yellow-500[\s\S]{0,160}bg-green-500|temp\(\)\s*>\s*60[\s\S]{0,160}text-red-500[\s\S]{0,160}text-yellow-500/g,
    message:
      'Do not define local disk detail presentation helpers in storage components. Use @/features/storageBackups/diskDetailPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-zfs-detail-presentation',
    regex:
      /text-yellow-600\s+dark:text-yellow-400\s+italic[\s\S]{0,200}scan|Errors:\s*R:\{[\s\S]{0,120}text-red-600\s+dark:text-red-400\s+font-medium/g,
    message:
      'Do not define local ZFS detail presentation in storage components. Use @/features/storageBackups/storagePoolDetailPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-ceph-health-helper',
    regex: /\b(?:const|function)\s+getHealthInfo\b/g,
    message:
      'Do not define local Ceph health presentation helpers in page code. Use the shared storage domain Ceph health helpers instead.',
  },
  {
    rule: 'canonical-storage/no-local-ceph-service-status-helper',
    regex: /\b(?:const|function)\s+getServiceStatus\b[\s\S]{0,500}svc\.running\s*===\s*svc\.total/g,
    message:
      'Do not define local Ceph service status helpers in page code. Use the shared storage domain Ceph service presentation helper instead.',
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
    rule: 'canonical-recovery/no-local-timeline-column-highlight',
    regex:
      /\bisSelected\b[\s\S]{0,220}bg-blue-100\s+dark:bg-blue-900[\s\S]{0,220}hover:bg-surface-hover/g,
    message:
      'Do not define local recovery timeline column highlight classes in page code. Use @/utils/recoveryTimelinePresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-chart-range-selected-classes',
    regex:
      /\bchartRangeDays\(\)\s*===\s*range\b[\s\S]{0,260}bg-blue-100\s+text-blue-700\s+dark:bg-blue-900\s+dark:text-blue-200/g,
    message:
      'Do not define local recovery chart range selected-button classes in page code. Use the shared segmented button contract instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-trend-range-selected-classes',
    regex:
      /\bselectedRange\(\)\s*===\s*range\b[\s\S]{0,260}bg-blue-600\s+text-white/g,
    message:
      'Do not define local dashboard trend range selected-button classes in page code. Use the shared segmented button contract instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-trend-error-copy',
    regex: /Unable to load trends/g,
    message:
      'Do not define local dashboard trend error copy in panel code. Use @/utils/dashboardTrendPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-findings-filter-selected-classes',
    regex:
      /\bfilter\(\)\s*===\s*['"](?:active|all|resolved)['"]\b[\s\S]{0,240}bg-surface-alt\s+text-base-content\s+border-border\s+shadow-sm|\bfilter\(\)\s*===\s*['"](?:attention|approvals)['"]\b[\s\S]{0,240}bg-amber-50\s+dark:bg-amber-900\s+text-amber-700\s+dark:text-amber-300\s+border-amber-300\s+dark:border-amber-700\s+shadow-sm/g,
    message:
      'Do not define local findings filter selected-button classes in component code. Use the shared segmented button contract instead.',
  },
  {
    rule: 'canonical-ai/no-local-quickstart-credits-presentation',
    regex:
      /\bquickstart_credits_remaining\b[\s\S]{0,360}bg-blue-50\s+dark:bg-blue-950\s+border-blue-200\s+dark:border-blue-800\s+text-blue-700\s+dark:text-blue-300|\bquickstart_credits_remaining\b[\s\S]{0,360}bg-amber-50\s+dark:bg-amber-950\s+border-amber-200\s+dark:border-amber-800\s+text-amber-700\s+dark:text-amber-300/g,
    message:
      'Do not define local AI quickstart credits badge presentation in page code. Use @/utils/aiQuickstartPresentation instead.',
  },
  {
    rule: 'canonical-patrol/no-local-summary-card-presentation',
    regex:
      /\bsummaryStats\(\)\.criticalFindings\s*>\s*0\b[\s\S]{0,320}bg-red-50[\s\S]{0,320}text-red-600|\bsummaryStats\(\)\.warningFindings\s*>\s*0\b[\s\S]{0,320}bg-amber-50[\s\S]{0,320}text-amber-600|\bsummaryStats\(\)\.fixedCount\s*>\s*0\b[\s\S]{0,320}bg-green-50[\s\S]{0,320}text-green-600/g,
    message:
      'Do not define local patrol summary card presentation in page code. Use @/utils/patrolSummaryPresentation instead.',
  },
  {
    rule: 'canonical-patrol/no-local-empty-state-copy',
    regex:
      /Loading messages\.\.\.|No investigation messages available\.|No patrol runs yet\. Trigger a run to populate history\.|Loading investigation\.\.\.|No investigation data available\. Enable patrol autonomy to investigate findings\.|Loading run history…|Loading tool calls\.\.\.|Tool call details not available for this run\./g,
    message:
      'Do not define local patrol loading or empty-state copy in component code. Use @/utils/patrolEmptyStatePresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-provider-test-result-text-class',
    regex:
      /\bproviderTestResult\(\)\?\.success\s*\?\s*['"]text-green-600['"]\s*:\s*['"]text-red-600['"]/g,
    message:
      'Do not define local AI provider test result text classes in component code. Use @/utils/aiSettingsPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-cost-range-button-selected-classes',
    regex:
      /\bdays\(\)\s*===\s*(?:1|7|30|90|365)\b[\s\S]{0,260}bg-blue-100\s+dark:bg-blue-900\s+text-blue-700\s+dark:text-blue-300\s+border-blue-300\s+dark:border-blue-700/g,
    message:
      'Do not define local AI cost range selected-button classes in page code. Use the shared segmented button contract instead.',
  },
  {
    rule: 'canonical-patrol/no-local-remediation-presentation',
    regex:
      /\bprops\.result\.success\b[\s\S]{0,900}bg-green-50[\s\S]{0,900}bg-red-50/g,
    message:
      'Do not define local remediation success/failure presentation in component code. Use @/utils/remediationPresentation instead.',
  },
  {
    rule: 'canonical-status/no-local-system-log-presentation',
    regex:
      /\blog\.includes\(['"]ERR['"]\)|\blog\.includes\(['"]\[WARN\]['"]\)|\bisPaused\(\)\s*\?\s*['"]Stream Paused['"]\s*:\s*['"]Live['"]|\bbg-amber-100\s+text-amber-600\s+dark:bg-amber-900\s+dark:text-amber-400/g,
    message:
      'Do not define local system log severity or stream-state presentation in component code. Use @/utils/systemLogsPresentation instead.',
  },
  {
    rule: 'canonical-status/no-local-diagnostics-alert-badge-ternary',
    regex:
      /\bdiagnosticsData\(\)\?\.alerts\?\.missingCooldown\b[\s\S]{0,260}bg-amber-100[\s\S]{0,260}bg-green-100|\bdiagnosticsData\(\)\?\.alerts\?\.missingGroupingWindow\b[\s\S]{0,260}bg-amber-100[\s\S]{0,260}bg-green-100/g,
    message:
      'Do not define local diagnostics alert badge ternaries in component code. Use the shared status tone contract instead.',
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
    rule: 'canonical-status/no-local-resource-picker-status-color',
    regex:
      /\bREPORTABLE_RESOURCE_TYPES\b[\s\S]{0,2200}\b(?:const|function)\s+getStatusColor\b|\b(?:const|function)\s+getStatusColor\b[\s\S]{0,2200}\bREPORTABLE_RESOURCE_TYPES\b/g,
    message:
      'Do not define local resource-picker status color helpers in component code. Use @/utils/status with StatusDot instead.',
  },
  {
    rule: 'canonical-status/no-local-mention-autocomplete-status-color',
    regex:
      /\bMentionResource\b[\s\S]{0,2600}\b(?:const|function)\s+getStatusColor\b|\b(?:const|function)\s+getStatusColor\b[\s\S]{0,2600}\bMentionResource\b/g,
    message:
      'Do not define local mention-autocomplete status color helpers in component code. Use @/utils/status with StatusDot instead.',
  },
  {
    rule: 'canonical-settings/no-local-unified-agent-status-pill',
    regex:
      /MONITORING_STOPPED_STATUS_LABEL[\s\S]{0,500}\b(?:const|function)\s+statusBadgeClass\b|\b(?:const|function)\s+statusBadgeClass\b[\s\S]{0,500}MONITORING_STOPPED_STATUS_LABEL/g,
    message:
      'Do not define local unified-agent status pill helpers in component code. Use @/utils/unifiedAgentStatusPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-unified-agent-lookup-status-pill',
    regex:
      /\b(?:const|function)\s+statusBadgeClasses\b[\s\S]{0,260}Connected[\s\S]{0,260}Not reporting yet/g,
    message:
      'Do not define local unified-agent lookup status helpers in component code. Use @/utils/unifiedAgentStatusPresentation instead.',
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
    rule: 'canonical-dashboard/no-local-storage-recovery-or-alert-copy',
    regex:
      /No active alerts|No storage resources|No recovery data available|Last recovery point over 24 hours ago/g,
    message:
      'Do not define local dashboard storage, recovery, or alert empty-state/staleness copy in page code. Use the shared dashboard presentation utilities instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-composition-or-ai-empty-state-copy',
    regex:
      /No resources detected|Loading usage…|No usage data yet\.|No daily USD trend yet\.|No daily token trend yet\.|No issues found/g,
    message:
      'Do not define local dashboard or AI empty-state copy in component code. Use the shared presentation utilities instead.',
  },
  {
    rule: 'canonical-ai/no-local-chat-empty-state-copy',
    regex: /No previous conversations|No matching models\./g,
    message:
      'Do not define local AI chat empty-state copy in component code. Use @/utils/aiChatPresentation instead.',
  },
  {
    rule: 'canonical-relay/no-local-onboarding-copy',
    regex:
      /Pair Your Mobile Device|Get Relay\s*(?:—|&mdash;)\s*\$49\/yr|or start a Pro trial|Starting trial\.\.\.|Set Up Relay|Relay is currently disconnected\.|Start Free Trial & Set Up Mobile|14-DAY PRO TRIAL/g,
    message:
      'Do not define local relay onboarding copy in component code. Use @/utils/relayPresentation instead.',
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
    rule: 'canonical-service-health/no-local-service-summary-tone-helper',
    regex: /\b(?:const|function)\s+summarizeServiceHealthTone\b/g,
    message:
      'Do not define local service summary tone helpers in component code. Use @/utils/serviceHealthPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-explore-status-presentation-helper',
    regex: /\b(?:const|function)\s+(?:phaseLabel|phaseClasses)\b/g,
    message:
      'Do not define local AI explore status label or tone helpers in component code. Use @/utils/aiExplorePresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-session-diff-status-presentation-helper',
    regex: /\b(?:const|function)\s+(?:formatDiffStatus|diffStatusClasses)\b/g,
    message:
      'Do not define local AI session diff status presentation helpers in component code. Use @/utils/aiSessionDiffPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-control-level-presentation-helper',
    regex:
      /\bconst\s+normalizeControlLevel\s*=|\bform\.controlLevel\s*===\s*['"]autonomous['"][\s\S]{0,320}bg-red-100|\bform\.controlLevel\s*===\s*['"]controlled['"][\s\S]{0,320}bg-amber-100/g,
    message:
      'Do not define local AI control-level normalization or presentation helpers in component code. Use @/utils/aiControlLevelPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-settings-readiness-presentation-helper',
    regex:
      /\bsettings\(\)\?\.configured\b[\s\S]{0,360}bg-green-50[\s\S]{0,360}bg-amber-50|\bsettings\(\)\?\.configured\b[\s\S]{0,360}bg-emerald-400[\s\S]{0,360}bg-amber-400/g,
    message:
      'Do not define local AI settings readiness presentation in component code. Use @/utils/aiSettingsPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-oauth-error-message-map',
    regex:
      /\bconst\s+errorMessages\s*:\s*Record<string,\s*string>\s*=\s*\{[\s\S]{0,260}missing_params[\s\S]{0,260}invalid_state[\s\S]{0,260}token_exchange_failed[\s\S]{0,260}save_failed/g,
    message:
      'Do not define local AI OAuth error message maps in component code. Use @/utils/aiSettingsPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-finding-status-presentation-helper',
    regex:
      /\bfinding\.status\s*===\s*['"]resolved['"][\s\S]{0,260}Resolved|\bfinding\.status\s*===\s*['"]snoozed['"][\s\S]{0,260}Snoozed/g,
    message:
      'Do not define local AI finding status badge or label logic in component code. Use @/utils/aiFindingPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-finding-severity-or-outcome-order',
    regex:
      /\bconst\s+severityOrder\s*:\s*Record<[^>]+>\s*=\s*\{|\bconst\s+outcomeOrder\s*:\s*Record<[^>]+>\s*=\s*\{/g,
    message:
      'Do not define local AI finding severity or investigation outcome sort order in component code. Use @/utils/aiFindingPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-finding-compact-severity-labels',
    regex:
      /\bfinding\.severity\s*===\s*['"]critical['"][\s\S]{0,120}['"]CRIT['"]|\bfinding\.severity\s*===\s*['"]warning['"][\s\S]{0,120}['"]WARN['"]/g,
    message:
      'Do not define local AI finding compact severity labels in component code. Use @/utils/aiFindingPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-local-findings-filter-or-empty-copy',
    regex:
      /Needs Attention \(|No active findings|Your infrastructure looks healthy!|No findings need attention right now\.|No pending approvals\.|No Patrol findings to display/g,
    message:
      'Do not define local findings filter labels or empty-state copy in component code. Use @/utils/aiFindingPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-alert-compact-severity-labels',
    regex:
      /\balert\.level\s*===\s*['"]critical['"]\s*\?\s*['"]CRIT['"]\s*:\s*['"]WARN['"]/g,
    message:
      'Do not define local alert compact severity labels in component code. Use @/utils/alertSeverityPresentation instead.',
  },
  {
    rule: 'canonical-ai/no-direct-patrol-outcome-presentation-imports',
    regex:
      /import\s*\{[\s\S]{0,200}\binvestigationStatusLabels\b[\s\S]{0,200}\}\s*from\s*['"]@\/api\/patrol['"]|import\s*\{[\s\S]{0,200}\binvestigationOutcomeLabels\b[\s\S]{0,200}\}\s*from\s*['"]@\/api\/patrol['"]|import\s*\{[\s\S]{0,200}\binvestigationOutcomeColors\b[\s\S]{0,200}\}\s*from\s*['"]@\/api\/patrol['"]/g,
    message:
      'Do not import patrol status or outcome labels/colors directly into UI code. Use @/utils/aiFindingPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-resource-picker-type-filter-label-map',
    regex:
      /\bconst\s+typeFilterLabels\s*:\s*Record<[^>]*TypeFilter[^>]*,\s*string>\s*=\s*\{|\[\s*['"]all['"]\s*,\s*['"]infrastructure['"]\s*,\s*['"]workloads['"]\s*,\s*['"]storage['"]\s*,\s*['"]recovery['"]\s*\]\s+as\s+TypeFilter\[\]/g,
    message:
      'Do not define local ResourcePicker filter labels or hardcoded filter lists in component code. Use @/utils/reportableResourceTypes instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-threshold-slider-color-map',
    regex:
      /\bconst\s+colorMap\s*:\s*Record<ThresholdSliderProps\['type'\],\s*string>\s*=\s*\{[\s\S]{0,220}cpu:[\s\S]{0,220}memory:[\s\S]{0,220}disk:[\s\S]{0,220}temperature:/g,
    message:
      'Do not define local threshold slider color maps in component code. Use @/utils/thresholdSliderPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-agent-profile-suggestion-value-badge-helper',
    regex: /\bconst\s+formatValueBadgeClass\s*=\s*\(/g,
    message:
      'Do not define local agent profile suggestion value badge helpers in component code. Use @/utils/agentProfileSuggestionPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-common-discovery-subnets',
    regex: /\bconst\s+COMMON_DISCOVERY_SUBNETS\s*=\s*\[/g,
    message:
      'Do not define local discovery subnet presets in settings components. Use @/utils/systemSettingsPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-audit-webhook-copy-or-shell',
    regex:
      /No audit webhooks configured yet\.|Loading audit webhooks…|Audit Webhooks \(Pro\)|Audit webhooks are part of the audit logging feature set and require Pro\./g,
    message:
      'Do not define local audit webhook copy or shell presentation in component code. Use @/utils/auditWebhookPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-rbac-permission-vocabulary',
    regex: /\bconst\s+(?:ACTIONS|RESOURCES)\s*=\s*\[/g,
    message:
      'Do not define local RBAC permission action/resource lists in settings components. Use @/utils/rbacPermissions instead.',
  },
  {
    rule: 'canonical-settings/no-local-rbac-copy',
    regex:
      /Custom Roles \(Pro\)|Centralized Access Control \(Pro\)|No users yet|Configure SSO in Security settings|Users sync on first login|No roles available\./g,
    message:
      'Do not define local RBAC feature-gate or empty-state copy in settings components. Use @/utils/rbacPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-organization-role-vocabulary',
    regex:
      /const\s+roleOptions:\s*Array<\{\s*value:\s*OrganizationRole;\s*label:\s*string\s*\}>\s*=\s*\[|const\s+accessRoleOptions:\s*Array<\{\s*value:\s*ShareAccessRole;\s*label:\s*string\s*\}>\s*=\s*\[|const\s+normalizeShareRole\s*=\s*\(role:\s*OrganizationRole\)/g,
    message:
      'Do not define local organization role option or share-role normalization helpers in settings components. Use @/utils/organizationRolePresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-organization-settings-fallback-or-error-copy',
    regex:
      /Multi-tenant requires an Enterprise license|Multi-tenant is not enabled on this server|This feature is not available\.|Failed to load organization access settings|Failed to load organization details|Failed to load organization sharing details|Failed to load billing and plan details|No organization members found\.|No members found\.|No outgoing shares configured\.|No incoming shares from other organizations\./g,
    message:
      'Do not define local organization settings fallback, load-error, or empty-state copy in panel code. Use @/utils/organizationSettingsPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-agent-profiles-empty-copy',
    regex: /No profiles yet\. Create one to get started\.|No agents connected\. Install an agent to assign profiles\./g,
    message:
      'Do not define local agent profile or assignment empty-state copy in component code. Use @/utils/agentProfilesPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-resource-picker-empty-copy',
    regex: /Resources appear as Pulse collects infrastructure and workload metrics/g,
    message:
      'Do not define local resource picker empty-state copy in component code. Use @/utils/reportableResourceTypes instead.',
  },
  {
    rule: 'canonical-settings/no-local-billing-status-or-tenant-vocabulary',
    regex:
      /Grace Period|No License|No trial|Trial \(ends|Trial \(started|Organization billing suspended|Organization billing activated|soft-deleted|No organizations found\./g,
    message:
      'Do not define local billing status, trial, or tenant-state vocabulary in settings panel code. Use @/utils/licensePresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-security-auth-copy',
    regex:
      /Authentication disabled|Authentication is currently disabled\. Set up password authentication to protect your Pulse instance\.|This account can view authentication status but cannot configure it\.|Authentication settings are read-only for this account\.|Security Configured - Restart Required|Security settings have been configured but the service needs to be restarted to activate them\.|After restarting, you'll need to log in with your saved credentials\./g,
    message:
      'Do not define local security authentication panel copy in component code. Use @/utils/securityAuthPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-sso-provider-type-label',
    regex: /\b(?:provider|form)\.type\.toUpperCase\(\)/g,
    message:
      'Do not derive SSO provider type labels inline in component code. Use @/utils/ssoProviderPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-inline-node-modal-test-result-tone-ternary',
    regex:
      /testResult\(\)\?\.(?:status)\s*===\s*['"]success['"][\s\S]{0,260}bg-green-50[\s\S]{0,260}testResult\(\)\?\.(?:status)\s*===\s*['"]warning['"][\s\S]{0,260}bg-amber-50/g,
    message:
      'Do not inline node modal test-result tone ternaries in component code. Use @/utils/nodeModalPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-node-modal-temperature-lock-copy',
    regex:
      /Locked by environment variables\. Remove the override \(ENABLE_TEMPERATURE_MONITORING\) and restart Pulse to manage it in the UI\./g,
    message:
      'Do not define local node modal temperature-monitoring lock copy in component code. Use @/utils/nodeModalPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-sso-provider-type-icon-switch',
    regex: /provider\.type\s*===\s*['"]oidc['"]\s*\?\s*\(/g,
    message:
      'Do not define local SSO provider type icon switches in component code. Use a shared SSO provider type icon component instead.',
  },
  {
    rule: 'canonical-settings/no-local-sso-empty-state-copy',
    regex: /No SSO providers configured|Loading SSO providers\.\.\./g,
    message:
      'Do not define local SSO empty-state copy in component code. Use @/utils/ssoProviderPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-ai-settings-loading-copy',
    regex:
      /Loading Pulse Assistant settings\.\.\.|Loading chat sessions\.\.\.|No chat sessions yet\. Start a chat to create one\./g,
    message:
      'Do not define local AI settings loading or empty-state copy in component code. Use @/utils/aiSettingsPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-profile-suggestion-loading-copy',
    regex: /Generating suggestion\.\.\./g,
    message:
      'Do not define local profile suggestion loading copy in component code. Use @/utils/agentProfileSuggestionPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-license-loading-copy',
    regex: /Loading license status\.\.\.|No Pro license is active\./g,
    message:
      'Do not define local license loading or empty-state copy in component code. Use @/utils/licensePresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-audit-log-copy',
    regex:
      /No audit events found|No events match your current filters\. Try adjusting or clearing them\.|Audit logging is active, but no events have been recorded yet\.|Loading audit events…/g,
    message:
      'Do not define local audit log loading or empty-state copy in component code. Use @/utils/auditLogPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-agent-ledger-copy',
    regex: /Loading agent ledger\.\.\.|Failed to load agent ledger\./g,
    message:
      'Do not define local agent ledger loading or error copy in component code. Use @/utils/unifiedAgentInventoryPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-settings-shell-copy',
    regex: /No settings found for "|Loading configuration\.\.\./g,
    message:
      'Do not define local settings shell loading or empty-state copy in component code. Use @/utils/settingsShellPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-inline-sso-test-result-tone-ternary',
    regex:
      /testResult\(\)\?\.(?:success)[\s\S]{0,260}bg-green-50[\s\S]{0,260}bg-red-50/g,
    message:
      'Do not inline SSO test-result tone ternaries in component code. Use @/utils/ssoProviderPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-inline-sso-certificate-tone-ternary',
    regex:
      /cert\.isExpired\s*\?[\s\S]{0,220}bg-red-100[\s\S]{0,220}bg-surface-hover/g,
    message:
      'Do not inline SSO certificate tone ternaries in component code. Use @/utils/ssoProviderPresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-artifact-mode-presentation-maps',
    regex:
      /\bconst\s+MODE_LABELS\s*:\s*Record<ArtifactMode,\s*string>\s*=|\bconst\s+MODE_BADGE_CLASS\s*:\s*Record<ArtifactMode,\s*string>\s*=|\bconst\s+CHART_SEGMENT_CLASS\s*:\s*Record<ArtifactMode,\s*string>\s*=/g,
    message:
      'Do not define local recovery artifact mode presentation maps in page code. Use @/utils/recoveryArtifactModePresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-filter-chip-presentation',
    regex:
      /(?:border-blue-200\s+bg-blue-50|border-cyan-200\s+bg-cyan-50|border-emerald-200\s+bg-emerald-50|border-violet-200\s+bg-violet-50)[\s\S]{0,260}(?:Day|Cluster|Node\/Agent|Namespace)/g,
    message:
      'Do not define local recovery filter chip presentation in page code. Use @/utils/recoveryFilterChipPresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-issue-rail-presentation-map',
    regex: /\bconst\s+ISSUE_RAIL_CLASS\s*:\s*Record<Exclude<IssueTone,\s*'none'>,\s*string>\s*=/g,
    message:
      'Do not define local recovery issue rail presentation maps in page code. Use @/utils/recoveryIssuePresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-summary-presentation-maps',
    regex:
      /\bconst\s+RECOVERY_TIME_RANGE_LABELS\s*:\s*Record<string,\s*string>\s*=|\bconst\s+FRESHNESS_LABELS\s*:\s*\{[\s\S]{0,120}label:\s*string;\s*color:\s*string\s*\}\[\]\s*=/g,
    message:
      'Do not define local recovery summary presentation maps in component code. Use @/utils/recoverySummaryPresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-empty-state-copy',
    regex:
      /No protected items yet|Pulse hasn’t observed any protected items for this org yet\.|No recovery history matches your filters|Adjust your search, provider, method, status, or verification filters\.|Loading protected items\.\.\.|Loading recovery activity\.\.\.|No recovery activity in the selected window\.|Loading recovery points\.\.\.|Failed to load protected items|Failed to load recovery points/g,
    message:
      'Do not define local recovery empty-state copy in component code. Use @/utils/recoveryEmptyStatePresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-table-presentation-helpers',
    regex:
      /\bconst\s+groupHeaderRowClass\s*=\s*\(|\bconst\s+groupHeaderTextClass\s*=\s*\(|\bconst\s+eventTimeTextClass\s*=\s*\(|\bconst\s+subjectTypeBadgeClass\s*=\s*\(|\bconst\s+artifactColumnHeaderClass\s*=\s*\(|\bconst\s+artifactRowClass\s*=\s*\(|\bconst\s+advancedFilterLabelClass\s*=|\bconst\s+advancedFilterFieldClass\s*=|\bconst\s+deriveRollupIssueTone\s*=\s*\(|\bconst\s+rollupAgeTextClass\s*=\s*\(/g,
    message:
      'Do not define local recovery table presentation helpers in component code. Use @/utils/recoveryTablePresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-date-presentation-helpers',
    regex:
      /\bconst\s+dateKeyFromTimestamp\s*=\s*\(|\bconst\s+parseDateKey\s*=\s*\(|\bconst\s+prettyDateLabel\s*=\s*\(|\bconst\s+fullDateLabel\s*=\s*\(|\bconst\s+compactAxisLabel\s*=\s*\(|\bconst\s+formatTimeOnly\s*=\s*\(|\bconst\s+niceAxisMax\s*=\s*\(/g,
    message:
      'Do not define local recovery date or timeline formatting helpers in component code. Use @/utils/recoveryDatePresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-record-presentation-helpers',
    regex:
      /\bconst\s+rollupSubjectLabel\s*=\s*\(|\bconst\s+pointTimestampMs\s*=\s*\(|\bconst\s+buildSubjectLabelForPoint\s*=\s*\(|\bconst\s+buildRepositoryLabelForPoint\s*=\s*\(|\bconst\s+buildDetailsSummaryForPoint\s*=\s*\(|\bconst\s+normalizeModeFromQuery\s*=\s*\(/g,
    message:
      'Do not define local recovery record-shaping helpers in component code. Use @/utils/recoveryRecordPresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-status-pill-presentation',
    regex:
      /\bprotectedStaleOnly\(\)\s*\?\s*['"]border-amber-300\s+bg-amber-50\s+text-amber-800|\brounded-full\s+bg-blue-100\/80\s+px-1\.5\s+py-px|\bwhitespace-nowrap\s+rounded\s+px-1\s+py-0\.5\s+text-\[10px\]\s+font-medium\s+text-amber-700\s+bg-amber-50|\bwhitespace-nowrap\s+rounded\s+px-1\s+py-0\.5\s+text-\[10px\]\s+font-medium\s+text-rose-700\s+bg-rose-100|\btext-amber-600\s+dark:text-amber-400['"]?\s*>\s*never/g,
    message:
      'Do not define local recovery status-pill or stale-toggle presentation in component code. Use @/utils/recoveryStatusPresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-action-link-presentation',
    regex:
      /setRollupId\(''\);\s*setView\('protected'\);[\s\S]{0,220}text-sm\s+font-medium\s+text-blue-600\s+dark:text-blue-400\s+hover:text-blue-700\s+dark:hover:text-blue-300\s+transition-colors|resetAdvancedArtifactFilters[\s\S]{0,220}text-xs\s+font-medium\s+text-blue-600\s+hover:text-blue-700\s+dark:text-blue-400\s+dark:hover:text-blue-300|resetAllArtifactFilters[\s\S]{0,260}inline-flex\s+items-center\s+gap-2\s+rounded-md\s+border\s+border-border\s+bg-surface\s+px-3\s+py-1\.5\s+text-xs\s+font-medium\s+text-base-content\s+hover:bg-surface-hover|aria-label=\"Close details\"[\s\S]{0,180}rounded-md\s+p-1\s+hover:text-base-content\s+hover:bg-surface-hover/g,
    message:
      'Do not define local recovery action-link, empty-state action, or drawer-close presentation in component code. Use @/utils/recoveryActionPresentation instead.',
  },
  {
    rule: 'canonical-recovery/no-local-timeline-chart-presentation',
    regex:
      /\bconst\s+labelEvery\s*=\s*dayCount\s*<=\s*7\s*\?\s*1\s*:\s*dayCount\s*<=\s*30\s*\?\s*3\s*:\s*10|inline-flex\s+rounded\s+border\s+border-border\s+bg-surface\s+p-0\.5\s+text-xs|font-semibold\s+text-blue-700\s+dark:text-blue-300|chartRangeDays\(\)\s*===\s*7[\s\S]{0,120}min-w-\[28px\][\s\S]{0,120}chartRangeDays\(\)\s*===\s*30[\s\S]{0,120}min-w-\[14px\]/g,
    message:
      'Do not define local recovery timeline chart presentation in component code. Use @/utils/recoveryTimelineChartPresentation instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-dashboard-helper-implementations',
    regex:
      /\bexport function statusBadgeClass\b|\bexport function priorityBadgeClass\b|\bexport function deltaColorClass\b/g,
    message:
      'Do not define dashboard metric presentation functions in page-local helpers. Use @/utils/dashboardMetricPresentation instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-composition-icon-map',
    regex: /\bconst\s+TYPE_ICONS\s*:\s*Record<string,\s*any>\s*=/g,
    message:
      'Do not define local dashboard composition icon maps in page code. Use @/utils/dashboardCompositionPresentation instead.',
  },
  {
    rule: 'canonical-dashboard/no-local-guest-backup-status-map',
    regex:
      /\bconst\s+BACKUP_STATUS_CONFIG\s*:\s*Record<|No backup found|No IP assigned|No filesystems found\. VM may be booting or using a Live ISO\./g,
    message:
      'Do not define local dashboard guest fallback copy or backup status maps in component code. Use @/utils/dashboardGuestPresentation instead.',
  },
  {
    rule: 'canonical-infrastructure/no-local-deploy-flow-copy',
    regex:
      /Loading cluster nodes\.\.\.|No online source agents found\. At least one node in this cluster must have a connected Pulse agent to deploy to other nodes\.|No nodes found in this cluster\.|Loading install command\.\.\./g,
    message:
      'Do not define local infrastructure deploy flow loading or empty-state copy in component code. Use @/utils/deployFlowPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-diagnostics-empty-copy',
    regex: /No PBS configured/g,
    message:
      'Do not define local diagnostics empty-state copy in component code. Use @/utils/diagnosticsPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-updates-status-copy-or-badges',
    regex: /Auto-check enabled|Manual checks only|Update Ready|Up to date/g,
    message:
      'Do not define local update-status copy or build-flavor badges in component code. Use @/utils/updatesPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-environment-lock-badge',
    regex: /Locked by environment variable PULSE_[A-Z0-9_]+|>\s*ENV\s*</g,
    message:
      'Do not define local environment-lock badges in component code. Use the shared EnvironmentLockBadge and @/utils/environmentLockPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-monitoring-stopped-empty-copy',
    regex:
      /No monitoring-stopped items match the current filters\.|No infrastructure currently has monitoring stopped\./g,
    message:
      'Do not define local monitoring-stopped empty-state copy in component code. Use @/utils/unifiedAgentInventoryPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-thresholds-empty-copy',
    regex:
      /No PBS servers configured\.|No VMs or containers found\.|No nodes match the current filters\.|No PBS servers match the current filters\.|No VMs or containers match the current filters\.|Configure guest filtering rules\.|Configure recovery alert thresholds\.|Configure snapshot age thresholds\.|No storage devices found\.|No storage devices match the current filters\.|No mail gateways configured yet\. Add a PMG instance in Settings to manage thresholds\.|No mail gateways match the current filters\.|No agents match the current filters\.|No agent disks found\. Agents with mounted filesystems will appear here\.|No agent disks match the current filters\.|No container runtimes match the current filters\.|No containers match the current filters\.|Search resources\.\.\.|Dismiss tips|Quick tips:|Swarm service alerts|Toggle Swarm service replica monitoring|Warning gap %|Convert to warning when at least this percentage of replicas are missing\.|Critical gap %|Raise a critical alert when the missing replica gap meets or exceeds this value\.|Critical gap must be greater than or equal to the warning gap when enabled\.|id=["']nodes["'][\s\S]{0,120}title=["']Proxmox Nodes["']|id=["']pbs["'][\s\S]{0,120}title=["']PBS Servers["']|id=["']guests["'][\s\S]{0,120}title=["']VMs & Containers["']|id=["']guest-filtering["'][\s\S]{0,120}title=["']Guest Filtering["']|id=["']backups["'][\s\S]{0,120}title=["']Recovery["']|id=["']snapshots["'][\s\S]{0,120}title=["']Snapshot Age["']|id=["']storage["'][\s\S]{0,120}title=["']Storage Devices["']|id=["']agentDisks["'][\s\S]{0,120}title=["']Agent Disks["']|title=["']Mail Gateway Thresholds["']|title=["']Container Runtimes["'][\s\S]{0,120}resources=\{dockerHostsWithOverrides\(\)\}|title=["']Containers["'][\s\S]{0,120}groupedResources=\{dockerContainersGroupedByHost\(\)\}|title=["']Agents["'][\s\S]{0,120}resources=\{agentsWithOverrides\(\)\}/g,
    message:
      'Do not define local thresholds empty-state copy in component code. Use @/utils/alertThresholdsPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-thresholds-section-status-copy',
    regex: /title=["']Unsaved changes["']|>\s*Disabled\s*</g,
    message:
      'Do not define local thresholds section status copy in component code. Use @/utils/alertThresholdsSectionPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-config-shell-copy',
    regex:
      /You have unsaved changes that will be lost\. Discard changes and leave\?|• Quiet hours active from|• Suppressing |• Recovery notifications enabled when alerts clear|Minimum time between alerts for the same issue|Per guest\/metric combination|Alerts within this window are grouped together\. Set to 0 to send immediately\.|CPU, memory, disk, and network thresholds stay quiet\.|Silence storage usage, disk health, and ZFS events\.|Skip connectivity and powered-off alerts during backups\./g,
    message:
      'Do not define local alerts config shell copy in page code. Use @/utils/alertConfigPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-timeline-state-copy',
    regex:
      /Loading timeline\.\.\.|No timeline events match the selected filters\.|No timeline events yet\.|No incident timeline available\.|Failed to load timeline\./g,
    message:
      'Do not define local alert incident timeline state copy in feature code. Use @/utils/alertOverviewPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-history-state-copy',
    regex:
      /Search alerts\.\.\.|No alerts found|Try adjusting your filters or check back later|Loading incidents\.\.\.|No incidents recorded for this resource yet\.|No alerts\b|Resource incidents|Acknowledged by |No events match the selected filters\.|Showing last \d+ events/g,
    message:
      'Do not define local alert history, incident-detail, or bucket-label copy in page code. Use the shared alert presentation utilities instead.',
  },
  {
    rule: 'canonical-alerts/no-local-incident-note-or-admin-copy',
    regex:
      /Add a note for this incident\.\.\.|Save Note|Loading alert history\.\.\.|Administrative Actions|Permanently clear all alert history\. Use with caution - this action cannot be undone\.|Are you sure you want to clear all alert history\?|Error clearing alert history: Please check your connection and try again\./g,
    message:
      'Do not define local incident-note, alert-history loading, or alert administration copy in page code. Use the shared alert presentation utilities instead.',
  },
  {
    rule: 'canonical-alerts/no-local-destinations-error-copy',
    regex:
      /Failed to load notification configuration\. Your existing settings could not be retrieved\.|Failed to load webhook configuration\.|Saving now may overwrite your existing settings with defaults\./g,
    message:
      'Do not define local alert destinations load-error or warning copy in page code. Use @/utils/alertDestinationsPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-destinations-panel-copy',
    regex:
      /Configure SMTP delivery for alert emails\.|Relay grouped alerts through Apprise via CLI or remote API\.|Choose how Pulse should execute Apprise notifications\.|Enter one Apprise URL per line\. Commas are also supported\.|Optional: override the URLs defined on your Apprise API instance\. Leave blank to use the server defaults\.|Enable Apprise notifications before sending a test\.|Add at least one Apprise target to test CLI delivery\.|Enable only when the Apprise API uses a self-signed certificate\./g,
    message:
      'Do not define local alert destinations panel vocabulary in page code. Use @/utils/alertDestinationsPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-resource-table-empty-copy',
    regex: /No resources available\.|No \{props\.title\.toLowerCase\(\)\} found/g,
    message:
      'Do not define local alert resource-table empty-state copy in component code. Use @/utils/alertResourceTablePresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-bulk-edit-copy',
    regex:
      /Bulk Edit Settings|Applying changes to \{props\.selectedIds\.length\} items\. Leave fields empty to keep existing options\.|Unchanged|Apply to \{props\.selectedIds\.length\} items/g,
    message:
      'Do not define local alert bulk-edit dialog copy in component code. Use @/utils/alertBulkEditPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-webhook-presentation-copy',
    regex:
      /Custom webhook endpoint|Discord server webhook|Slack incoming webhook|Mattermost incoming webhook|Telegram bot notifications|Microsoft Teams webhook|Teams with Adaptive Cards|Mobile push notifications|Self-hosted push notifications|Push notifications via ntfy\.sh|PagerDuty Events API v2|My Webhook|https:\/\/example\.com\/webhook|Optional — tag users or groups|Enable this webhook|\+ Add custom field|\+ Add header/g,
    message:
      'Do not define local alert webhook service vocabulary, placeholders, or action copy in component code. Use @/utils/alertWebhookPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-email-provider-copy',
    regex:
      /smtp\.example\.com|noreply@example\.com|Leave empty to use .*Or add one recipient per line|Show setup instructions|Hide setup instructions|Show advanced options|Hide advanced options|Send test email|Sending test email…/g,
    message:
      'Do not define local alert email-provider vocabulary, placeholders, or action copy in component code. Use @/utils/alertEmailPresentation instead.',
  },
  {
    rule: 'canonical-docker/no-local-swarm-empty-state-copy',
    regex:
      /Loading Swarm services\.\.\.|No services match your filters|No Swarm services found|Enable Swarm service collection in the container runtime agent \(includeServices\) and wait for the next report\.|Try clearing the search\./g,
    message:
      'Do not define local Swarm services loading or empty-state copy in component code. Use @/utils/swarmPresentation instead.',
  },
  {
    rule: 'canonical-k8s/no-local-deployments-empty-state-copy',
    regex:
      /Loading deployments\.\.\.|Fetching unified resources\.|No deployments match your filters|Try clearing the search or namespace filter\.|No deployments found|Enable the Kubernetes agent deployment collection, then wait for the next report\./g,
    message:
      'Do not define local Kubernetes deployments loading or empty-state copy in component code. Use @/utils/k8sDeploymentPresentation instead.',
  },
  {
    rule: 'canonical-k8s/no-local-namespaces-empty-state-copy',
    regex:
      /Loading namespaces\.\.\.|Aggregating Kubernetes namespaces\.|Failed to load namespaces|No namespaces match your filters|Try clearing your search\.|No namespaces found|Enable Kubernetes collection and wait for the next report\./g,
    message:
      'Do not define local Kubernetes namespaces loading, failure, or empty-state copy in component code. Use @/utils/k8sNamespacePresentation instead.',
  },
  {
    rule: 'canonical-discovery/no-local-url-suggestion-source-label-helper',
    regex: /\b(?:const|function)\s+getURLSuggestionSourceLabel\b/g,
    message:
      'Do not define local discovery URL suggestion label helpers in component code. Use @/utils/discoveryPresentation instead.',
  },
  {
    rule: 'canonical-discovery/no-local-empty-state-copy',
    regex:
      /No discovery data yet|Run a discovery scan to identify services and configurations|Checking existing discovery data\.\.\.|You can run discovery now if this takes too long\.|Loading discovery\.\.\.|No suggested URL found|No notes yet\. Add notes to document important information\./g,
    message:
      'Do not define local discovery empty-state copy in component code. Use @/utils/discoveryPresentation instead.',
  },
  {
    rule: 'canonical-discovery/no-local-discovery-badge-classes',
    regex:
      /(?:bg-blue-100\s+text-blue-700|bg-green-100\s+text-green-700)[\s\S]{0,260}(?:Analysis:|getCategoryDisplayName\()/g,
    message:
      'Do not define local discovery badge classes in component code. Use @/utils/discoveryPresentation instead.',
  },
  {
    rule: 'canonical-ceph/no-local-page-state-copy',
    regex:
      /Loading Ceph data\.\.\.|Connecting to the monitoring service\.|No Ceph Clusters Detected|Ceph cluster data will appear here when detected via the Pulse agent on your Proxmox nodes\. Install the agent on a node with Ceph configured\.|No pools match "/g,
    message:
      'Do not define local Ceph page-state copy in page code. Use @/features/storageBackups/storageDomain instead.',
  },
  {
    rule: 'canonical-pmg/no-local-threat-bar-presentation-helper',
    regex: /\b(?:const|function)\s+(?:barColor|textColor)\b[\s\S]{0,500}quarantine/g,
    message:
      'Do not define local PMG threat bar presentation helpers in component code. Use @/utils/pmgThreatPresentation instead.',
  },
  {
    rule: 'canonical-pmg/no-local-queue-severity-helper',
    regex: /\b(?:const|function)\s+queueSeverity\b/g,
    message:
      'Do not define local PMG queue severity helpers in component code. Use @/utils/pmgQueuePresentation instead.',
  },
  {
    rule: 'canonical-pmg/no-local-service-health-badge',
    regex: /\bconst\s+StatusBadge:\s*Component<\{\s*status:\s*string;\s*health\?:\s*string\s*\}>\b/g,
    message:
      'Do not define local PMG service health badge components in page code. Use the shared PMG ServiceHealthBadge component instead.',
  },
  {
    rule: 'canonical-pmg/no-local-empty-state-copy',
    regex:
      /No Mail Gateways configured|Add a Proxmox Mail Gateway via Settings → Infrastructure → Proxmox to start collecting mail analytics and security metrics\.|Loading mail gateway data\.\.\.|Search gateways\.\.\.|No gateways match "|No PMG details for this resource yet|Pulse hasn't ingested PMG analytics for this instance\.|Loading mail gateway details\.\.\.|Fetching PMG resource details\.|Failed to load PMG details/g,
    message:
      'Do not define local PMG loading, failure, or empty-state copy in component code. Use @/utils/pmgPresentation instead.',
  },
  {
    rule: 'canonical-relay/no-local-connection-status-helper',
    regex: /\b(?:const|function)\s+(?:connectionStatusVariant|connectionStatusText)\b/g,
    message:
      'Do not define local relay connection status helpers in component code. Use @/utils/relayPresentation instead.',
  },
  {
    rule: 'canonical-deploy/no-local-deploy-status-config',
    regex: /\bconst\s+statusConfig\s*:\s*Record<DeployTargetStatus/g,
    message:
      'Do not define local deploy status presentation maps in component code. Use @/utils/deployStatusPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-incident-status-or-level-classes',
    regex: /\bconst\s+(?:statusClasses|levelClasses)\s*=/g,
    message:
      'Do not define local alert incident status or level badge classes in page code. Use @/utils/alertIncidentPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-history-status-ternary',
    regex:
      /alert\.status\s*===\s*['"]active['"][\s\S]{0,260}(?:bg-red-100|bg-yellow-100)|(?:bg-red-100|bg-yellow-100)[\s\S]{0,260}alert\.status\s*===\s*['"]active['"]/g,
    message:
      'Do not define local alert history status ternaries in page code. Use @/utils/alertIncidentPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-history-source-or-type-badge-ternary',
    regex:
      /alert\.source\s*===\s*['"]ai['"][\s\S]{0,260}(?:bg-violet-100|bg-sky-100)|alert\.resourceType\s*===\s*['"]VM['"][\s\S]{0,420}(?:bg-blue-100|bg-green-100|bg-orange-100)/g,
    message:
      'Do not define local alert history source or resource-type badge ternaries in page code. Use @/utils/alertHistoryPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-activation-toggle-presentation',
    regex:
      /isAlertsActive\(\)\s*\?\s*'text-green-600\s+dark:text-green-400'\s*:\s*'text-muted'|isAlertsActive\(\)\s*\?\s*'bg-blue-600'\s*:\s*'bg-surface-hover'|isAlertsActive\(\)\s*\?\s*'translate-x-5'\s*:\s*'translate-x-0'/g,
    message:
      'Do not define local alert activation toggle presentation in page code. Use @/utils/alertActivationPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-frequency-filter-presentation',
    regex:
      /Filtered Range[\s\S]{0,220}border-blue-200[\s\S]{0,220}bg-blue-50|Clear filter[\s\S]{0,220}bg-blue-100[\s\S]{0,220}hover:bg-blue-200/g,
    message:
      'Do not define local alert frequency filter presentation in page code. Use @/utils/alertFrequencyPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-severity-dot-classes',
    regex:
      /h-2\s+w-2\s+rounded-full\s+bg-yellow-500[\s\S]{0,120}warnings|h-2\s+w-2\s+rounded-full\s+bg-red-500[\s\S]{0,120}critical/g,
    message:
      'Do not define local alert severity dot classes in page code. Use @/utils/alertSeverityPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-tab-state-presentation',
    regex:
      /areAlertsDisabled\(\)[\s\S]{0,260}cursor-not-allowed\s+text-muted\s+bg-surface-alt|activeTab\(\)\s*===\s*item\.id[\s\S]{0,260}bg-blue-50\s+text-blue-600|activeTab\(\)\s*===\s*tab\.id[\s\S]{0,260}bg-surface\s+text-base-content\s+shadow-sm/g,
    message:
      'Do not define local alerts tab state presentation in page code. Use @/utils/alertTabsPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-tab-labels',
    regex:
      /id:\s*['"]status['"][\s\S]{0,220}label:\s*['"]Status['"][\s\S]{0,260}id:\s*['"]overview['"][\s\S]{0,120}label:\s*['"]Overview['"][\s\S]{0,220}id:\s*['"]history['"][\s\S]{0,120}label:\s*['"]History['"]|id:\s*['"]configuration['"][\s\S]{0,220}label:\s*['"]Configuration['"][\s\S]{0,260}id:\s*['"]thresholds['"][\s\S]{0,120}label:\s*['"]Thresholds['"][\s\S]{0,220}id:\s*['"]destinations['"][\s\S]{0,120}label:\s*['"]Notifications['"][\s\S]{0,220}id:\s*['"]schedule['"][\s\S]{0,120}label:\s*['"]Schedule['"]/g,
    message:
      'Do not define local alerts tab labels in page code. Use @/utils/alertTabsPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-grouping-card-presentation',
    regex:
      /grouping\(\)\.byNode[\s\S]{0,260}border-blue-500\s+bg-blue-50\s+shadow-sm[\s\S]{0,260}grouping\(\)\.byGuest[\s\S]{0,260}border-blue-500\s+bg-blue-50\s+shadow-sm|grouping\(\)\.byNode[\s\S]{0,180}border-blue-500\s+bg-blue-500|grouping\(\)\.byGuest[\s\S]{0,180}border-blue-500\s+bg-blue-500/g,
    message:
      'Do not define local alert grouping selector presentation in page code. Use @/utils/alertGroupingPresentation instead.',
  },
  {
    rule: 'canonical-alerts/no-local-quiet-day-button-presentation',
    regex:
      /quietHours\(\)\.days\[day\.id\][\s\S]{0,220}rounded-md\s+bg-blue-500\s+text-white\s+shadow-sm[\s\S]{0,220}rounded-md\s+text-muted\s+hover:bg-surface-hover/g,
    message:
      'Do not define local quiet-day button presentation in page code. Use @/utils/alertSchedulePresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-configured-node-capability-badges',
    regex:
      /'monitorVMs'\s+in\s+node|'monitorDatastores'\s+in\s+node|\bmonitorMailStats\b[\s\S]{0,220}(?:bg-blue-100|bg-green-100)/g,
    message:
      'Do not define local configured-node capability badge branches in component code. Use @/utils/configuredNodeCapabilityPresentation instead.',
  },
  {
    rule: 'canonical-settings/no-local-audit-log-badge-helpers',
    regex: /\bconst\s+(?:getEventTypeBadge|getVerificationBadge)\s*=\s*\(/g,
    message:
      'Do not define local audit log badge helpers in component code. Use @/utils/auditLogPresentation instead.',
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
    rule: 'canonical-storage/no-local-storage-pool-text-helpers',
    regex: /\bconst\s+protectionTextClass\s*=\s*\(|\bconst\s+issueTextClass\s*=\s*\(/g,
    message:
      'Do not define local storage pool text-tone helpers in component code. Use @/features/storageBackups/rowPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-storage-pool-compact-helpers',
    regex:
      /\bconst\s+compactProtection\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*\{|\bconst\s+compactImpact\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*\{|\bconst\s+compactIssue\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*\{|\bconst\s+compactIssueSummary\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*\{/g,
    message:
      'Do not define local compact storage row helpers in component code. Use @/features/storageBackups/rowPresentation instead.',
  },
  {
    rule: 'canonical-storage/no-local-storage-group-helpers',
    regex:
      /\bconst\s+visibleHealthCounts\s*=\s*\(\)\s*=>\s*|\bconst\s+poolCountLabel\s*=\s*\(\)\s*=>\s*|\bconst\s+usagePercentLabel\s*=\s*\(\)\s*=>\s*/g,
    message:
      'Do not define local storage group aggregate helpers in component code. Use @/features/storageBackups/groupPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-filter-option-arrays',
    regex:
      /\bconst\s+sortOptions\s*=\s*props\.sortOptions\s*\?\?\s*\[|\b<option\s+value=\"available\">Healthy<\/option>/g,
    message:
      'Do not define local storage filter option defaults in component code. Use ./storagePageState instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-groupby-options',
    regex:
      /options=\s*\[\s*\{\s*value:\s*'none',\s*label:\s*'Flat'[\s\S]*?\{\s*value:\s*'status',\s*label:\s*'By Status'\s*\}\s*\]/g,
    message:
      'Do not define local storage group-by options in component code. Use ./storagePageState instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-reset-defaults',
    regex:
      /props\.setSortKey\('priority'\)|props\.setSortDirection\('desc'\)|props\.setGroupBy\('none'\)|props\.setStatusFilter\('all'\)|props\.setSourceFilter\('all'\)/g,
    message:
      'Do not hardcode storage filter reset defaults in component code. Use ./storagePageState constants instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-model-core-helpers',
    regex:
      /\bconst\s+matchesSelectedNode\s*=\s*\(|\bconst\s+sourceOptions\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*\{|\bconst\s+sortedRecords\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*\{|\bconst\s+groupedRecords\s*=\s*createMemo\s*<StorageGroupedRecords\[]>\s*\(\s*\(\)\s*=>\s*\{/g,
    message:
      'Do not define storage model filter/sort/group helpers in the hook. Use @/features/storageBackups/storageModelCore instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-row-alert-presentation',
    regex:
      /\bconst\s+showAlertHighlight\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*[\s\S]{0,120}?parentNodeOnline\(\)|\bconst\s+hasAcknowledgedOnlyAlert\s*=\s*createMemo\s*\(\s*\(\)\s*=>\s*[\s\S]{0,120}?parentNodeOnline\(\)/g,
    message:
      'Do not define local storage row alert/highlight presentation in page code. Use @/features/storageBackups/storageRowAlertPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-page-copy',
    regex:
      /Reconnecting to backend data stream…|Storage data stream disconnected\. Data may be stale\.|Waiting for storage data from connected platforms\.|Unable to refresh storage resources\. Showing latest available data\.|No storage records match the current filters\.|Ceph Summary/g,
    message:
      'Do not define storage page copy inline. Use @/features/storageBackups/storagePagePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-page-vocabulary',
    regex:
      /\{\s*value:\s*'pools',\s*label:\s*'Pools'\s*\}|\{\s*value:\s*'disks',\s*label:\s*'Physical Disks'\s*\}|Loading storage resources\.\.\.|rounded border border-amber-300 bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800 hover:bg-amber-200 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200 dark:hover:bg-amber-900/g,
    message:
      'Do not define storage page tabs, loading copy, or banner action styling inline. Use @/features/storageBackups/storagePagePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-controls-node-styling',
    regex:
      /aria-label="Node"[\s\S]{0,220}focus:ring-blue-500|aria-label="Node"[\s\S]{0,220}h-5 w-px bg-surface-hover hidden sm:block/g,
    message:
      'Do not define storage control node select styling inline. Use @/features/storageBackups/storagePagePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-filter-sort-styling',
    regex:
      /aria-label="Sort By"[\s\S]{0,220}focus:ring-blue-500|aria-label="Sort Direction"[\s\S]{0,220}hover:bg-surface-hover transition-colors/g,
    message:
      'Do not define storage filter sort control styling inline. Use @/features/storageBackups/storageFilterPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-page-status-helpers',
    regex:
      /\bconst\s+isWaitingForData\s*=\s*createMemo\s*\(|\bconst\s+isDisconnectedAfterLoad\s*=\s*createMemo\s*\(/g,
    message:
      'Do not define storage page banner/loading state helpers inline. Use @/features/storageBackups/storagePageStatus instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-deeplink-helpers',
    regex:
      /\bconst\s+\{\s*resource\s*\}\s*=\s*parseStorageLinkSearch\(/g,
    message:
      'Do not define storage deep-link highlight state inline. Use ./useStorageResourceHighlight instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-storage-expansion-state',
    regex:
      /\bconst\s+\[expandedGroups,\s*setExpandedGroups\]\s*=\s*createSignal<Set<string>>\(|\bconst\s+\[expandedPoolId,\s*setExpandedPoolId\]\s*=\s*createSignal<string\s*\|\s*null>\(|\bsyncExpandedStorageGroups\(|\btoggleExpandedStorageGroup\(/g,
    message:
      'Do not define storage expansion state inline in page code. Use ./useStorageExpansionState instead.',
    validate: () => true,
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
    rule: 'canonical-storage/no-local-ceph-summary-card-classes',
    regex:
      /title=\{cluster\.healthMessage\}[\s\S]{0,220}text-\[11px\] text-muted truncate max-w-\[240px\]|cluster\.healthLabel[\s\S]{0,220}px-1\.5 py-0\.5 rounded text-\[10px\] font-medium/g,
    message:
      'Do not define Ceph summary card styling inline. Use @/features/storageBackups/cephSummaryCardPresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-storage/no-local-zfs-health-tooltip-classes',
    regex:
      /hoveredTooltip\(\)\?\.\w+[\s\S]{0,320}fixed z-\[9999\] pointer-events-none|hoveredTooltip\(\)\?\.\w+[\s\S]{0,320}bg-surface text-base-content text-\[10px\] rounded-md shadow-sm px-2 py-1\.5 min-w-\[120px\] border border-border/g,
    message:
      'Do not define ZFS health-map tooltip styling inline. Use @/features/storageBackups/zfsHealthMapPresentation instead.',
    validate: () => true,
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
    rule: 'canonical-license/no-local-plan-or-trial-notice-presentation',
    regex:
      /\bconst\s+formatTitleCase\s*=\s*\(|\bconst\s+commercialMigrationActionText\s*=\s*\(|\bconst\s+commercialMigrationNoticeFor\s*=\s*\(/g,
    message:
      'Do not define local license plan-version or migration/trial notice presentation helpers in component code. Use @/utils/licensePresentation instead.',
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
    rule: 'canonical-security/no-local-security-posture-item-or-network-copy',
    regex:
      /\blabel:\s*'Password login'|\b(?:'Public network access detected'|'Private network access')/g,
    message:
      'Do not define local security posture item or network-access copy in component code. Use @/utils/securityScorePresentation instead.',
    validate: () => true,
  },
  {
    rule: 'canonical-security/no-inline-security-warning-severity-ternary',
    regex:
      /publicAccess\s*&&\s*!.*hasAuthentication[\s\S]{0,260}bg-red-50[\s\S]{0,260}bg-yellow-50/g,
    message:
      'Do not inline security warning severity/background ternaries in component code. Use @/utils/securityScorePresentation instead.',
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
    if (/(\.test|\.spec)\.[jt]sx?$/.test(relativePath)) continue;

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
