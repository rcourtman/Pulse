import { createEffect, createSignal, on } from 'solid-js';
import {
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  settingsTabPath,
  type AgentKey,
  type SettingsTab,
} from './settingsRouting';

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
  const [currentTab, setCurrentTab] = createSignal<SettingsTab>(deriveTabFromPath(location.pathname));
  const activeTab = () => currentTab();

  const [selectedAgent, setSelectedAgent] = createSignal<AgentKey>('pve');

  const agentPaths: Record<AgentKey, string> = {
    pve: '/settings/infrastructure/pve',
    pbs: '/settings/infrastructure/pbs',
    pmg: '/settings/infrastructure/pmg',
  };

  const handleSelectAgent = (agent: AgentKey) => {
    setSelectedAgent(agent);
    if (currentTab() !== 'proxmox') {
      setCurrentTab('proxmox');
    }
    const target = agentPaths[agent];
    if (target && location.pathname !== target) {
      navigate(target, { scroll: false });
    }
  };

  const setActiveTab = (tab: SettingsTab) => {
    if (tab === 'proxmox' && deriveAgentFromPath(location.pathname) === null) {
      setSelectedAgent('pve');
    }
    const targetPath = settingsTabPath(tab);
    if (location.pathname !== targetPath) {
      navigate(targetPath, { scroll: false });
      return;
    }
    if (currentTab() !== tab) {
      setCurrentTab(tab);
    }
  };

  // Keep tab state in sync with URL and handle /settings redirect without flicker.
  createEffect(
    on(
      () => [location.pathname, location.search] as const,
      ([path, search]) => {
        if (path === '/settings' || path === '/settings/') {
          const queryTab = deriveTabFromQuery(search);
          const resolvedTab = queryTab ?? 'proxmox';

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

        if (path.startsWith('/settings/agent-hub')) {
          navigate(path.replace('/settings/agent-hub', '/settings/infrastructure'), {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (path.startsWith('/settings/servers')) {
          navigate(path.replace('/settings/servers', '/settings/infrastructure'), {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (path.startsWith('/settings/containers')) {
          navigate(path.replace('/settings/containers', '/settings/workloads/docker'), {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (
          path.startsWith('/settings/linuxServers') ||
          path.startsWith('/settings/windowsServers') ||
          path.startsWith('/settings/macServers')
        ) {
          navigate('/settings/workloads', {
            replace: true,
            scroll: false,
          });
          return;
        }

        const resolved = deriveTabFromPath(path);
        if (resolved !== currentTab()) {
          setCurrentTab(resolved);
        }

        if (resolved === 'proxmox') {
          const agentFromPath = deriveAgentFromPath(path);
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
