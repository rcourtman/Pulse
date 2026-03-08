import { describe, expect, it } from 'vitest';
import resourceTypeCompatSource from '@/utils/resourceTypeCompat.ts?raw';
import discoveryTypesSource from '@/types/discovery.ts?raw';
import resourceLinksSource from '@/routing/resourceLinks.ts?raw';
import reportingResourceTypesSource from '@/components/Settings/reportingResourceTypes.ts?raw';
import chartsApiSource from '@/api/charts.ts?raw';
import investigateAlertButtonSource from '@/components/Alerts/InvestigateAlertButton.tsx?raw';
import resourceBadgesSource from '@/components/Infrastructure/resourceBadges.ts?raw';
import workloadTypeBadgesSource from '@/components/shared/workloadTypeBadges.ts?raw';
import discoveryTargetSource from '@/utils/discoveryTarget.ts?raw';

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
    expect(resourceLinksSource).toContain('canonicalizeFrontendResourceType');
    expect(resourceLinksSource).not.toContain("normalized === 'docker'");
    expect(resourceLinksSource).not.toContain("normalized === 'k8s'");

    expect(reportingResourceTypesSource).toContain('export function toReportingResourceType');
    expect(reportingResourceTypesSource).toContain("case 'k8s-cluster'");
    expect(reportingResourceTypesSource).toContain("return 'k8s';");
    expect(reportingResourceTypesSource).not.toContain("case 'host'");

    expect(chartsApiSource).toContain('export function toMetricsHistoryAPIResourceType');
    expect(chartsApiSource).toContain("| 'k8s-cluster'");
    expect(chartsApiSource).toContain("| 'k8s-node'");
    expect(chartsApiSource).toContain("| 'pod'");
    expect(chartsApiSource).toContain("case 'k8s-cluster'");
    expect(chartsApiSource).toContain("return 'k8s';");
    expect(chartsApiSource).toContain(
      "guestTypes?: Record<string, 'vm' | 'system-container' | 'k8s'>",
    );

    expect(investigateAlertButtonSource).toContain('canonicalizeFrontendResourceType');
    expect(resourceBadgesSource).toContain('canonicalizeFrontendResourceType');
    expect(workloadTypeBadgesSource).toContain('canonicalizeFrontendResourceType');
    expect(discoveryTargetSource).toContain('canonicalizeFrontendResourceType');
  });
});
