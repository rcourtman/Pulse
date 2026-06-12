import { Accessor, createMemo, createSignal } from 'solid-js';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import {
  getInfrastructureReadOnlyHeaderDescription,
  getSettingsHeaderMeta,
} from './settingsHeaderMeta';
import { isInfrastructureSettingsTab, type SettingsTab } from './settingsNavigationModel';

interface UseSettingsShellStateParams {
  activeTab: Accessor<SettingsTab>;
}

export function useSettingsShellState({ activeTab }: UseSettingsShellStateParams) {
  const headerMeta = createMemo(() => {
    const tab = activeTab();
    const headerMetaByTab = getSettingsHeaderMeta();
    if (isInfrastructureSettingsTab(tab) && presentationPolicyIsReadOnly()) {
      return {
        title: headerMetaByTab['infrastructure-systems'].title,
        description: getInfrastructureReadOnlyHeaderDescription(),
      };
    }

    if (isInfrastructureSettingsTab(tab)) {
      return {
        title: headerMetaByTab['infrastructure-systems'].title,
        description: headerMetaByTab['infrastructure-systems'].description,
      };
    }

    return (
      headerMetaByTab[tab] ?? {
        title: 'Settings',
        description: 'Manage Pulse configuration.',
      }
    );
  });

  // Sidebar always starts expanded for discoverability (issue #764)
  // Users can collapse during session but it resets on page reload
  const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = createSignal(false);
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);
  const [searchQuery, setSearchQuery] = createSignal('');

  return {
    headerMeta,
    sidebarCollapsed,
    setSidebarCollapsed,
    isMobileMenuOpen,
    setIsMobileMenuOpen,
    showPasswordModal,
    setShowPasswordModal,
    searchQuery,
    setSearchQuery,
  };
}
