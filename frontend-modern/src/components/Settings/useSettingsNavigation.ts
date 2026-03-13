import { createEffect, createSignal, on } from 'solid-js';
import {
  DEFAULT_SETTINGS_TAB,
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  resolveCanonicalSettingsPath,
  settingsAgentPath,
  settingsTabPath,
  type AgentKey,
  type SettingsTab,
} from './settingsRouting';

type SettingsDispatchableTab = Exclude<SettingsTab, 'proxmox'>;

type SettingsLocation = {
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
  const activeTab = (): SettingsDispatchableTab => {
    const tab = currentTab();
    return tab === 'proxmox' ? 'agents' : tab;
  };

  const [selectedAgent, setSelectedAgent] = createSignal<AgentKey>('pve');

  const handleSelectAgent = (agent: AgentKey) => {
    setSelectedAgent(agent);
    if (currentTab() !== 'proxmox') {
      setCurrentTab('proxmox');
    }
    const target = settingsAgentPath(agent);
    if (target && location.pathname !== target) {
      navigate(target, { scroll: false });
    }
  };

  const setActiveTab = (tab: SettingsTab) => {
    if (tab === 'proxmox' && deriveAgentFromPath(location.pathname) === null) {
      setSelectedAgent('pve');
    }
    // Eagerly update tab state so UI reflects the click immediately,
    // even before the URL change triggers the sync effect.
    if (currentTab() !== tab) {
      setCurrentTab(tab);
    }
    const targetPath = settingsTabPath(tab);
    if (location.pathname !== targetPath) {
      navigate(targetPath, { scroll: false });
    }
  };

  // Keep tab state in sync with canonical URLs, while preserving old deep links as aliases.
  createEffect(
    on(
      () => [location.pathname, location.search] as const,
      ([path, search]) => {
        if (path === '/settings' || path === '/settings/') {
          const queryTab = deriveTabFromQuery(search);
          const resolvedTab = queryTab ?? DEFAULT_SETTINGS_TAB;

          if (queryTab) {
            const target = settingsTabPath(resolvedTab);
            if (target !== path) {
              navigate(target, { replace: true, scroll: false });
              return;
            }
          }

          if (currentTab() !== resolvedTab) {
            setCurrentTab(resolvedTab);
          }

          if (resolvedTab === 'proxmox') {
            setSelectedAgent('pve');
          }
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

        if (resolved === 'proxmox') {
          const agentFromPath = deriveAgentFromPath(effectivePath);
          setSelectedAgent(agentFromPath ?? 'pve');
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
