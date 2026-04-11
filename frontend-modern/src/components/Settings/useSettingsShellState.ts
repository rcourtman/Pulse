import { Accessor, createMemo, createSignal } from 'solid-js';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import type { SettingsTab } from './settingsNavigationModel';

interface UseSettingsShellStateParams {
  activeTab: Accessor<SettingsTab>;
}

export function useSettingsShellState({ activeTab }: UseSettingsShellStateParams) {
  const headerMeta = createMemo(
    () => {
      const tab = activeTab();
      if (tab === 'infrastructure-operations' && presentationPolicyIsReadOnly()) {
        return {
          title: 'Infrastructure Operations',
          description:
            'Review the current monitored-system inventory, reporting posture, and connected platform coverage. Setup changes stay unavailable in this read-only session.',
        };
      }

      return (
        SETTINGS_HEADER_META[tab] ?? {
          title: 'Settings',
          description: 'Manage Pulse configuration.',
        }
      );
    },
  );

  // Sidebar always starts expanded for discoverability (issue #764)
  // Users can collapse during session but it resets on page reload
  const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = createSignal(
    typeof window !== 'undefined' ? window.innerWidth < 1024 : false,
  );
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
