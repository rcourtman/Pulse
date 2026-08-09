import type { Resource } from '@/types/resource';

const PREVIEW_TIMESTAMP_MS = Date.UTC(2026, 3, 10, 12, 0, 0, 0);

export type SetupCompletionPreviewScenarioId = 'empty' | 'vmware-api-backed' | 'pro-unlicensed';

export interface SetupCompletionPreviewScenario {
  id: SetupCompletionPreviewScenarioId;
  resources: readonly Resource[];
  // Always explicit so the preview never falls through to the live license
  // probe: the panel treats undefined as "fetch", and preview must not fetch.
  proActivation: boolean;
}

const SETUP_COMPLETION_PREVIEW_SCENARIOS: Record<
  SetupCompletionPreviewScenarioId,
  SetupCompletionPreviewScenario
> = {
  empty: {
    id: 'empty',
    resources: [],
    proActivation: false,
  },
  'vmware-api-backed': {
    id: 'vmware-api-backed',
    resources: [
      {
        id: 'vmware-preview-1',
        type: 'agent',
        name: 'vcsa-prod',
        displayName: 'vCenter Prod',
        platformId: 'vmware-preview',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        status: 'online',
        lastSeen: PREVIEW_TIMESTAMP_MS,
        platformData: {
          vmware: { hostname: 'vcenter.preview.local' },
        },
      },
    ],
    proActivation: false,
  },
  'pro-unlicensed': {
    id: 'pro-unlicensed',
    resources: [],
    proActivation: true,
  },
};

export const DEFAULT_SETUP_COMPLETION_PREVIEW_SCENARIO_ID: SetupCompletionPreviewScenarioId =
  'empty';

export const getSetupCompletionPreviewScenario = (
  search: string | null | undefined,
): SetupCompletionPreviewScenario => {
  const scenario = new URLSearchParams(search || '').get('scenario');
  if (scenario === 'vmware-api-backed' || scenario === 'pro-unlicensed') {
    return SETUP_COMPLETION_PREVIEW_SCENARIOS[scenario];
  }
  return SETUP_COMPLETION_PREVIEW_SCENARIOS[DEFAULT_SETUP_COMPLETION_PREVIEW_SCENARIO_ID];
};
