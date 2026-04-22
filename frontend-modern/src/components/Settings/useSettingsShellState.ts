import { Accessor, createMemo, createSignal } from 'solid-js';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import { isInfrastructureSettingsTab, type SettingsTab } from './settingsNavigationModel';

interface UseSettingsShellStateParams {
  activeTab: Accessor<SettingsTab>;
}

export function useSettingsShellState({ activeTab }: UseSettingsShellStateParams) {
  const headerMeta = createMemo(() => {
    const tab = activeTab();
    if (isInfrastructureSettingsTab(tab) && presentationPolicyIsReadOnly()) {
      return {
        title: 'Infrastructure',
        description:
          'Review the current top-level monitored systems and reporting posture. Setup changes stay unavailable in this read-only session.',
      };
    }

    if (isInfrastructureSettingsTab(tab)) {
      return {
        title: 'Infrastructure',
        description: SETTINGS_HEADER_META['infrastructure-systems'].description,
      };
    }

    return (
      SETTINGS_HEADER_META[tab] ?? {
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
