import { createSignal } from 'solid-js';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { SettingsPageShell } from '../SettingsPageShell';
import type { SettingsNavGroup, SettingsTab } from '../settingsNavigationModel';

const TestIcon = () => <span aria-hidden="true" />;

const tabs: SettingsNavGroup['items'] = [
  { id: 'infrastructure-systems', label: 'Infrastructure', icon: TestIcon },
  { id: 'api', label: 'API Access', icon: TestIcon },
];

describe('SettingsPageShell mobile navigation', () => {
  afterEach(cleanup);

  it('updates the compact active-page label after client-side navigation', async () => {
    const [activeTab, setActiveTab] = createSignal<SettingsTab>('infrastructure-systems');
    const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
    const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
    const [searchQuery, setSearchQuery] = createSignal('');

    render(() => (
      <SettingsPageShell
        headerMeta={() => ({ title: 'Settings', description: 'Manage Pulse configuration.' })}
        hasUnsavedChanges={() => false}
        activeTabSaveBehavior={() => undefined}
        saveSettings={() => undefined}
        discardChanges={() => undefined}
        isMobileMenuOpen={mobileMenuOpen}
        setIsMobileMenuOpen={setMobileMenuOpen}
        sidebarCollapsed={sidebarCollapsed}
        setSidebarCollapsed={setSidebarCollapsed}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        filteredTabGroups={() => [{ id: 'infrastructure', label: 'Settings', items: tabs }]}
        flatTabs={() => tabs}
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        isPro={() => false}
      >
        <div>Panel content</div>
      </SettingsPageShell>
    ));

    expect(screen.getAllByText('Infrastructure')).toHaveLength(2);
    expect(screen.getAllByText('API Access')).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Settings' })).toHaveClass('min-h-11');

    const shell = document.querySelector('[data-settings-shell]');
    const navigation = document.querySelector('[data-settings-navigation]');
    const contentBody = document.querySelector('[data-settings-content-body]');
    expect(shell).toHaveClass('space-y-0', 'lg:space-y-6');
    expect(navigation).toHaveClass('max-h-[calc(100dvh-8rem)]');
    expect(contentBody).toHaveClass('min-w-0', 'p-0', 'sm:p-4', 'lg:p-5');

    setActiveTab('api');

    await waitFor(() => {
      expect(screen.getAllByText('Infrastructure')).toHaveLength(1);
      expect(screen.getAllByText('API Access')).toHaveLength(2);
    });
  });

  it('keeps the narrow content column width-constrained and at rest', () => {
    const [activeTab] = createSignal<SettingsTab>('api');
    const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
    const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
    const [searchQuery, setSearchQuery] = createSignal('');

    const { container } = render(() => (
      <SettingsPageShell
        headerMeta={() => ({ title: 'API Access', description: 'Manage API access.' })}
        hasUnsavedChanges={() => false}
        activeTabSaveBehavior={() => undefined}
        saveSettings={() => undefined}
        discardChanges={() => undefined}
        isMobileMenuOpen={mobileMenuOpen}
        setIsMobileMenuOpen={setMobileMenuOpen}
        sidebarCollapsed={sidebarCollapsed}
        setSidebarCollapsed={setSidebarCollapsed}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        filteredTabGroups={() => [{ id: 'infrastructure', label: 'Settings', items: tabs }]}
        flatTabs={() => tabs}
        activeTab={activeTab}
        setActiveTab={() => undefined}
        isPro={() => false}
      >
        <div>Panel content</div>
      </SettingsPageShell>
    ));

    const content = container.querySelector('[data-settings-content]');
    expect(content).toHaveClass(
      'min-w-0',
      'flex-1',
      'overflow-visible',
      'lg:overflow-hidden',
      'block',
    );
    expect(content).not.toHaveClass('animate-slideInRight');
    expect(content?.lastElementChild).toHaveClass('min-w-0');
    expect(content?.lastElementChild).toHaveClass('py-3', 'sm:p-6', 'lg:p-8');
    expect(content?.lastElementChild).not.toHaveClass('p-4');
  });

  it('expands a collapsed desktop sidebar before opening the phone settings index', async () => {
    const [activeTab] = createSignal<SettingsTab>('api');
    const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
    const [sidebarCollapsed, setSidebarCollapsed] = createSignal(true);
    const [searchQuery, setSearchQuery] = createSignal('');

    render(() => (
      <SettingsPageShell
        headerMeta={() => ({ title: 'API Access', description: 'Manage API access.' })}
        hasUnsavedChanges={() => false}
        activeTabSaveBehavior={() => undefined}
        saveSettings={() => undefined}
        discardChanges={() => undefined}
        isMobileMenuOpen={mobileMenuOpen}
        setIsMobileMenuOpen={setMobileMenuOpen}
        sidebarCollapsed={sidebarCollapsed}
        setSidebarCollapsed={setSidebarCollapsed}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        filteredTabGroups={() => [{ id: 'infrastructure', label: 'Settings', items: tabs }]}
        flatTabs={() => tabs}
        activeTab={activeTab}
        setActiveTab={() => undefined}
        isPro={() => false}
      >
        <div>Panel content</div>
      </SettingsPageShell>
    ));

    expect(sidebarCollapsed()).toBe(true);
    await screen.getByRole('button', { name: 'Settings' }).click();

    await waitFor(() => {
      expect(sidebarCollapsed()).toBe(false);
      expect(mobileMenuOpen()).toBe(true);
      expect(screen.getByPlaceholderText('Search settings...')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Collapse settings navigation' })).toHaveClass(
      'hidden',
      'lg:inline-flex',
    );

    await screen.getByRole('button', { name: 'Close settings navigation' }).click();
    await waitFor(() => expect(mobileMenuOpen()).toBe(false));
  });
});
