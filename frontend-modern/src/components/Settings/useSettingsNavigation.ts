import { createEffect, createSignal, on } from 'solid-js';
import {
  presentationPolicyIsReadOnly,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';
import { resolveCanonicalSelfHostedBillingHref } from '@/utils/pricingHandoff';
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLegacyPath,
} from './infrastructureWorkspaceModel';
import {
  DEFAULT_SETTINGS_TAB,
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  isProxmoxSettingsPath,
  resolveCanonicalSettingsPath,
  settingsTabPath,
  type AgentKey,
  type SettingsTab,
} from './settingsNavigationModel';

type SettingsLocation = {
  hash: string;
  pathname: string;
  search: string;
};

type SettingsNavigate = (
  to: string,
  options?: {
    replace?: boolean;
    scroll?: boolean;
  },
) => void;

interface UseSettingsNavigationParams {
  navigate: SettingsNavigate;
  location: SettingsLocation;
}

export function useSettingsNavigation({ navigate, location }: UseSettingsNavigationParams) {
  const [currentTab, setCurrentTab] = createSignal<SettingsTab>(
    deriveTabFromPath(location.pathname),
  );
  const resolveTabPath = (tab: SettingsTab): string => settingsTabPath(tab);
  const activeTab = (): SettingsTab => currentTab();

  const [selectedAgent, setSelectedAgent] = createSignal<AgentKey>('pve');

  const handleSelectAgent = (agent: AgentKey) => {
    setSelectedAgent(agent);
  };

  const setActiveTab = (tab: SettingsTab) => {
    if (currentTab() !== tab) {
      setCurrentTab(tab);
    }
    const targetPath = resolveTabPath(tab);
    if (location.pathname !== targetPath) {
      navigate(targetPath, { scroll: false });
    }
  };

  createEffect(
    on(
      () => [location.pathname, location.search, location.hash] as const,
      ([path, search, hash]) => {
        if (path === '/settings' || path === '/settings/') {
          const queryTab = deriveTabFromQuery(search);
          const resolvedTab = queryTab ?? DEFAULT_SETTINGS_TAB;
          const target = resolveTabPath(resolvedTab);

          if (target !== path) {
            navigate(target, { replace: true, scroll: false });
            return;
          }

          if (currentTab() !== resolvedTab) {
            setCurrentTab(resolvedTab);
          }
          return;
        }

        const currentHref = `${path}${search}${hash}`;
        const canonicalBillingHref = resolveCanonicalSelfHostedBillingHref(path, search, hash);
        if (canonicalBillingHref && canonicalBillingHref !== currentHref) {
          navigate(canonicalBillingHref, {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (
          sessionPresentationPolicyResolved() &&
          presentationPolicyIsReadOnly() &&
          (path.startsWith('/settings/infrastructure') ||
            path === '/settings/workloads' ||
            path === '/settings/workloads/docker')
        ) {
          navigate(buildInfrastructureWorkspacePath(), {
            replace: true,
            scroll: false,
          });
          return;
        }

        // Sync Proxmox agent from the raw path before any redirect so deep
        // links like /platforms/proxmox/pbs still seed the correct agent.
        if (isProxmoxSettingsPath(path)) {
          const agentFromPath = deriveAgentFromPath(path) ?? 'pve';
          if (selectedAgent() !== agentFromPath) {
            setSelectedAgent(agentFromPath);
          }
        }

        const infrastructureOnboardingStep = deriveAddStepFromLegacyPath(path);
        if (infrastructureOnboardingStep) {
          navigate(buildInfrastructureOnboardingPath(infrastructureOnboardingStep), {
            replace: true,
            scroll: false,
          });
          return;
        }

        const canonicalPath = resolveCanonicalSettingsPath(path);
        if (canonicalPath && canonicalPath !== path) {
          navigate(canonicalPath, {
            replace: true,
            scroll: false,
          });
          return;
        }

        const effectivePath = canonicalPath ?? path;
        const resolved = deriveTabFromPath(effectivePath);
        if (resolved !== currentTab()) {
          setCurrentTab(resolved);
        }
      },
    ),
  );

  return {
    currentTab,
    activeTab,
    selectedAgent,
    setSelectedAgent,
    setActiveTab,
    handleSelectAgent,
  };
}
