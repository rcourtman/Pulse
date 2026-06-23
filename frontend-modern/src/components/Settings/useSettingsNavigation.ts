import { createEffect, createSignal, on } from 'solid-js';
import {
  presentationPolicyIsReadOnly,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';
import { resolveCanonicalSelfHostedBillingHref } from '@/utils/pricingHandoff';
import { buildInfrastructureWorkspacePath } from './infrastructureWorkspaceModel';
import {
  EXTERNAL_AGENT_SETUP_PATH,
  SETTINGS_API_ACCESS_PATH,
  isExternalAgentSetupHash,
} from '@/routing/resourceLinks';
import {
  DEFAULT_SETTINGS_TAB,
  deriveTabFromPath,
  deriveTabFromQuery,
  isAISettingsOAuthCallbackQuery,
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
          const shouldPreserveQuery =
            resolvedTab === 'system-ai' && isAISettingsOAuthCallbackQuery(search);
          const targetHref = shouldPreserveQuery ? `${target}${search}${hash}` : target;
          const currentHref = `${path}${search}${hash}`;

          if (targetHref !== currentHref) {
            navigate(targetHref, { replace: true, scroll: false });
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

        if (path === SETTINGS_API_ACCESS_PATH && isExternalAgentSetupHash(hash)) {
          navigate(EXTERNAL_AGENT_SETUP_PATH, {
            replace: true,
            scroll: false,
          });
          return;
        }

        if (
          sessionPresentationPolicyResolved() &&
          presentationPolicyIsReadOnly() &&
          path.startsWith('/settings/infrastructure')
        ) {
          navigate(buildInfrastructureWorkspacePath(), {
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
