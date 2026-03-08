import { describe, expect, it } from 'vitest';
import resourceTypeCompatSource from '@/utils/resourceTypeCompat.ts?raw';
import discoveryTypesSource from '@/types/discovery.ts?raw';
import resourceLinksSource from '@/routing/resourceLinks.ts?raw';
import reportingResourceTypesSource from '@/components/Settings/reportingResourceTypes.ts?raw';
import reportingResourceTypesUtilSource from '@/utils/reportingResourceTypes.ts?raw';
import chartsApiSource from '@/api/charts.ts?raw';
import investigateAlertButtonSource from '@/components/Alerts/InvestigateAlertButton.tsx?raw';
import alertTargetTypesSource from '@/utils/alertTargetTypes.ts?raw';
import resourceBadgesSource from '@/components/Infrastructure/resourceBadges.ts?raw';
import resourceBadgePresentationSource from '@/utils/resourceBadgePresentation.ts?raw';
import workloadTypeBadgesSource from '@/components/shared/workloadTypeBadges.ts?raw';
import workloadTypePresentationSource from '@/utils/workloadTypePresentation.ts?raw';
import sourcePlatformsSource from '@/utils/sourcePlatforms.ts?raw';
import discoveryTargetSource from '@/utils/discoveryTarget.ts?raw';
import recoverySummarySource from '@/components/Recovery/RecoverySummary.tsx?raw';
import dashboardRecoverySource from '@/hooks/useDashboardRecovery.ts?raw';
import recoveryOutcomePresentationSource from '@/utils/recoveryOutcomePresentation.ts?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
import problemResourcePresentationSource from '@/utils/problemResourcePresentation.ts?raw';
import kpiStripSource from '@/pages/DashboardPanels/KPIStrip.tsx?raw';
import recentAlertsPanelSource from '@/pages/DashboardPanels/RecentAlertsPanel.tsx?raw';
import dashboardAlertPresentationSource from '@/utils/dashboardAlertPresentation.ts?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import temperatureUtilSource from '@/utils/temperature.ts?raw';
import pmgInstanceDrawerSource from '@/components/PMG/PMGInstanceDrawer.tsx?raw';
import serviceHealthPresentationSource from '@/utils/serviceHealthPresentation.ts?raw';
import swarmServicesDrawerSource from '@/components/Docker/SwarmServicesDrawer.tsx?raw';
import k8sDeploymentsDrawerSource from '@/components/Kubernetes/K8sDeploymentsDrawer.tsx?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import k8sStatusPresentationSource from '@/utils/k8sStatusPresentation.ts?raw';
import raidCardSource from '@/components/shared/cards/RaidCard.tsx?raw';
import raidPresentationSource from '@/utils/raidPresentation.ts?raw';
import proLicensePanelSource from '@/components/Settings/ProLicensePanel.tsx?raw';
import securityPostureSummarySource from '@/components/Settings/SecurityPostureSummary.tsx?raw';
import licensePresentationSource from '@/utils/licensePresentation.ts?raw';
import securityScorePresentationSource from '@/utils/securityScorePresentation.ts?raw';
import resourceDetailDrawerSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';

describe('frontend resource type boundaries', () => {
  it('keeps the shared compatibility adapter narrow and explicit', () => {
    expect(resourceTypeCompatSource).toContain('export const canonicalizeFrontendResourceType');
    expect(resourceTypeCompatSource).toContain("case 'host'");
    expect(resourceTypeCompatSource).toContain("case 'docker'");
    expect(resourceTypeCompatSource).toContain("case 'docker_host'");
    expect(resourceTypeCompatSource).toContain("case 'k8s'");
    expect(resourceTypeCompatSource).toContain("case 'kubernetes_cluster'");
    expect(resourceTypeCompatSource).not.toContain("case 'qemu'");
    expect(resourceTypeCompatSource).not.toContain("case 'lxc'");
    expect(resourceTypeCompatSource).not.toContain("case 'container'");
  });

  it('keeps canonical frontend discovery types separate from backend API aliases', () => {
    expect(discoveryTypesSource).toContain(
      "export type ResourceType = 'vm' | 'system-container' | 'app-container' | 'pod' | 'agent';",
    );
    expect(discoveryTypesSource).toContain('export type APIResourceType =');
    expect(discoveryTypesSource).toContain("| 'k8s'");
  });

  it('keeps compatibility handling centralized in shared adapters and edge translators', () => {
    expect(resourceLinksSource).toContain('canonicalizeWorkloadFilterType');
    expect(resourceLinksSource).toContain('normalizeSourcePlatformQueryValue');
    expect(resourceLinksSource).not.toContain("normalized === 'docker'");
    expect(resourceLinksSource).not.toContain("normalized === 'k8s'");
    expect(sourcePlatformsSource).toContain('export const normalizeSourcePlatformQueryValue');

    expect(reportingResourceTypesSource).toContain('@/utils/reportingResourceTypes');
    expect(reportingResourceTypesUtilSource).toContain('export function toReportingResourceType');
    expect(reportingResourceTypesUtilSource).toContain("case 'k8s-cluster'");
    expect(reportingResourceTypesUtilSource).toContain("return 'k8s';");
    expect(reportingResourceTypesSource).not.toContain("case 'host'");

    expect(chartsApiSource).toContain('export function toMetricsHistoryAPIResourceType');
    expect(chartsApiSource).toContain('export function asMetricsHistoryResourceType');
    expect(chartsApiSource).toContain('export function mapUnifiedTypeToHistoryResourceType');
    expect(chartsApiSource).toContain('export function canonicalizeMetricsHistoryTargetType');
    expect(chartsApiSource).toContain("| 'k8s-cluster'");
    expect(chartsApiSource).toContain("| 'k8s-node'");
    expect(chartsApiSource).toContain("| 'pod'");
    expect(chartsApiSource).toContain("case 'k8s-cluster'");
    expect(chartsApiSource).toContain("return 'k8s';");
    expect(chartsApiSource).toContain(
      "guestTypes?: Record<string, 'vm' | 'system-container' | 'k8s'>",
    );

    expect(investigateAlertButtonSource).toContain('resolveAlertTargetType');
    expect(investigateAlertButtonSource).not.toContain('canonicalizeFrontendResourceType');
    expect(alertTargetTypesSource).toContain('canonicalizeFrontendResourceType');
    expect(resourceBadgesSource).toContain('@/utils/resourceBadgePresentation');
    expect(resourceBadgePresentationSource).toContain('getResourceTypePresentation');
    expect(resourceBadgesSource).not.toContain('function formatType(');
    expect(workloadTypePresentationSource).toContain('canonicalizeFrontendResourceType');
    expect(workloadTypeBadgesSource).not.toContain('canonicalizeFrontendResourceType');
    expect(workloadTypeBadgesSource).toContain('getWorkloadTypePresentation');
    expect(discoveryTargetSource).toContain('canonicalizeFrontendResourceType');
    expect(recoveryOutcomePresentationSource).toContain('import type { RecoveryOutcome }');
    expect(recoverySummarySource).toContain('normalizeRecoveryOutcome');
    expect(recoverySummarySource).not.toContain('const normalizeOutcome =');
    expect(dashboardRecoverySource).toContain('normalizeRecoveryOutcome');
    expect(dashboardRecoverySource).not.toContain('const normalizeOutcome =');
    expect(problemResourcesTableSource).toContain('getProblemResourceStatusVariant');
    expect(problemResourcesTableSource).not.toContain(
      'function statusVariant(pr: ProblemResource)',
    );
    expect(problemResourcePresentationSource).toContain(
      'export function getProblemResourceStatusVariant',
    );
    expect(kpiStripSource).toContain('getDashboardAlertTone');
    expect(kpiStripSource).not.toContain('const alertsTone =');
    expect(recentAlertsPanelSource).toContain('getAlertSeverityTextClass');
    expect(dashboardAlertPresentationSource).toContain('export function getDashboardAlertTone');
    expect(diskListSource).toContain('getTemperatureTextClass');
    expect(diskListSource).not.toContain('const getTemperatureTone =');
    expect(temperatureUtilSource).toContain('export const getTemperatureTextClass');
    expect(pmgInstanceDrawerSource).toContain('getServiceHealthPresentation');
    expect(pmgInstanceDrawerSource).not.toContain('const statusTone =');
    expect(serviceHealthPresentationSource).toContain(
      'export function getServiceHealthPresentation',
    );
    expect(swarmServicesDrawerSource).toContain('getSimpleStatusIndicator');
    expect(swarmServicesDrawerSource).toContain('<StatusDot');
    expect(swarmServicesDrawerSource).not.toContain('const statusTone =');
    expect(k8sDeploymentsDrawerSource).toContain('getSimpleStatusIndicator');
    expect(k8sDeploymentsDrawerSource).toContain('<StatusDot');
    expect(k8sDeploymentsDrawerSource).not.toContain('const statusTone =');
    expect(k8sNamespacesDrawerSource).toContain('getNamespaceCountsIndicator');
    expect(k8sNamespacesDrawerSource).toContain('<StatusDot');
    expect(k8sNamespacesDrawerSource).not.toContain('const statusTone =');
    expect(k8sStatusPresentationSource).toContain('export function getNamespaceCountsIndicator');
    expect(raidCardSource).toContain('getRaidStateVariant');
    expect(raidCardSource).toContain('getRaidStateTextClass');
    expect(raidCardSource).toContain('getRaidDeviceBadgeClass');
    expect(raidCardSource).not.toContain('const raidStateVariant =');
    expect(raidCardSource).not.toContain('const deviceToneClass =');
    expect(raidPresentationSource).toContain('export function getRaidStateVariant');
    expect(raidPresentationSource).toContain('export function getRaidDeviceBadgeClass');
    expect(proLicensePanelSource).toContain('getLicenseSubscriptionStatusPresentation');
    expect(proLicensePanelSource).not.toContain('const statusLabel =');
    expect(proLicensePanelSource).not.toContain('const statusTone =');
    expect(licensePresentationSource).toContain(
      'export const getLicenseSubscriptionStatusPresentation',
    );
    expect(securityPostureSummarySource).toContain('getSecurityScorePresentation');
    expect(securityPostureSummarySource).not.toContain('const scoreTone =');
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityScorePresentation',
    );
    expect(resourceDetailDrawerSource).toContain('getServiceHealthPresentation');
    expect(resourceDetailDrawerSource).not.toContain('healthToneClass(');
    expect(resourceDetailDrawerSource).not.toContain('normalizeHealthLabel(');
    expect(resourceDetailMappersSource).not.toContain('export const normalizeHealthLabel');
    expect(resourceDetailMappersSource).not.toContain('export const healthToneClass');
  });
});
