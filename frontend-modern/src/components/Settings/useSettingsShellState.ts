import { Accessor, createMemo, createSignal } from 'solid-js';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import type { SettingsTab } from './settingsTypes';

interface UseSettingsShellStateParams {
  activeTab: Accessor<SettingsTab>;
}

export function useSettingsShellState({ activeTab }: UseSettingsShellStateParams) {
  const headerMeta = createMemo(
    () =>
      SETTINGS_HEADER_META[activeTab()] ?? {
        title: 'Settings',
        description: 'Manage Pulse configuration.',
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
