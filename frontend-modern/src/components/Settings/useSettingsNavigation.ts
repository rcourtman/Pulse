import { createEffect, createSignal, on } from 'solid-js';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { resolveCanonicalSelfHostedBillingHref } from '@/utils/pricingHandoff';
import { buildInfrastructureWorkspacePath } from './infrastructureWorkspaceModel';
import {
  DEFAULT_SETTINGS_TAB,
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  isInfrastructureSettingsTab,
  isProxmoxSettingsPath,
  resolveCanonicalSettingsPath,
  settingsAgentPath,
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
  const resolveTabPath = (tab: SettingsTab): string =>
    presentationPolicyIsReadOnly() &&
    isInfrastructureSettingsTab(tab) &&
    tab !== 'infrastructure-systems'
      ? buildInfrastructureWorkspacePath('inventory')
      : settingsTabPath(tab);
  const activeTab = (): SettingsTab => currentTab();

  const [selectedAgent, setSelectedAgent] = createSignal<AgentKey>('pve');

  const handleSelectAgent = (agent: AgentKey) => {
    setSelectedAgent(agent);
    if (currentTab() !== 'infrastructure-connections') {
      setCurrentTab('infrastructure-connections');
    }
    const target = settingsAgentPath(agent);
    if (target && location.pathname !== target) {
      navigate(target, { scroll: false });
    }
  };

  const setActiveTab = (tab: SettingsTab) => {
    if (tab === 'infrastructure-connections' && deriveAgentFromPath(location.pathname) === null) {
      setSelectedAgent('pve');
    }
    // Eagerly update tab state so UI reflects the click immediately,
    // even before the URL change triggers the sync effect.
    if (currentTab() !== tab) {
      setCurrentTab(tab);
    }
    const targetPath = resolveTabPath(tab);
    if (location.pathname !== targetPath) {
      navigate(targetPath, { scroll: false });
    }
  };

  // Keep tab state in sync with canonical URLs, while preserving old deep links as aliases.
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
          presentationPolicyIsReadOnly() &&
          (path === '/settings/infrastructure' ||
            path.startsWith('/settings/infrastructure/platforms') ||
            path.startsWith('/settings/infrastructure/install') ||
            path.startsWith('/settings/infrastructure/proxmox') ||
            path.startsWith('/settings/infrastructure/api') ||
            path.startsWith('/settings/infrastructure/truenas') ||
            path.startsWith('/settings/infrastructure/vmware') ||
            path === '/settings/workloads' ||
            path === '/settings/workloads/docker')
        ) {
          navigate(buildInfrastructureWorkspacePath('inventory'), {
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

        if (isProxmoxSettingsPath(effectivePath)) {
          const agentFromPath = deriveAgentFromPath(effectivePath) ?? 'pve';
          if (selectedAgent() !== agentFromPath) {
            setSelectedAgent(agentFromPath);
          }
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
