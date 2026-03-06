import { Accessor, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import type { SettingsTab } from './settingsTypes';

interface UseSettingsShellStateParams {
  activeTab: Accessor<SettingsTab>;
}

export function useSettingsShellState({ activeTab }: UseSettingsShellStateParams) {
  const headerMeta = createMemo(() =>
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
  let searchInputRef: HTMLInputElement | undefined;

  const assignSearchInputRef = (el: HTMLInputElement) => {
    searchInputRef = el;
  };

  onMount(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSearchQuery('');
        searchInputRef?.blur();
        return;
      }

      if (
        document.activeElement?.tagName === 'INPUT' ||
        document.activeElement?.tagName === 'TEXTAREA'
      ) {
        return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey || e.key.length > 1) {
        if (e.key !== 'Backspace') {
          return;
        }
      }

      if (searchInputRef) {
        e.preventDefault();
        searchInputRef.focus();
        if (e.key === 'Backspace') {
          setSearchQuery((prev) => prev.slice(0, -1));
        } else {
          setSearchQuery((prev) => prev + e.key);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    onCleanup(() => window.removeEventListener('keydown', handleKeyDown));
  });

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
    assignSearchInputRef,
  };
}
