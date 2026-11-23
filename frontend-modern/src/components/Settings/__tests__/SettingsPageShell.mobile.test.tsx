import { createSignal } from 'solid-js';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PulseLogoIcon } from '@/components/icons/PulseLogoIcon';
import { SettingsPageShell } from '../SettingsPageShell';
import type { SettingsNavGroup, SettingsTab } from '../settingsNavigationModel';

const TestIcon = () => <span aria-hidden="true" />;

const tabs: SettingsNavGroup['items'] = [
  { id: 'infrastructure-systems', label: 'Infrastructure', icon: TestIcon },
  { id: 'api', label: 'API Access', icon: TestIcon },
];

describe('SettingsPageShell mobile navigation', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

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
    expect(navigation?.tagName).toBe('NAV');
    expect(navigation).not.toHaveAttribute('aria-expanded');
    expect(navigation).toHaveClass(
      'max-h-[calc(100dvh-var(--pulse-mobile-nav-height,4rem)-4.5rem)]',
    );
    expect(screen.getByRole('heading', { name: 'Settings', level: 2 })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Collapse settings navigation' })).toHaveAttribute(
      'aria-controls',
      'settings-sidebar-menu',
    );
    expect(screen.getByRole('button', { name: 'Collapse settings navigation' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
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

  it('reveals a focused control behind the compact header without moving focus', async () => {
    const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
    const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
    const [searchQuery, setSearchQuery] = createSignal('');
    const scrollBy = vi.fn();
    render(() => (
      <div
        class="app-scroll-shell"
        ref={(el) => Object.defineProperty(el, 'scrollBy', { value: scrollBy })}
      >
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
          filteredTabGroups={() => []}
          flatTabs={() => tabs}
          activeTab={() => 'api'}
          setActiveTab={() => undefined}
          isPro={() => false}
        >
          <button type="button">Covered control</button>
          <button type="button">Visible control</button>
        </SettingsPageShell>
      </div>
    ));

    const header = screen.getByRole('heading', { name: 'API Access' }).parentElement!;
    const covered = screen.getByRole('button', { name: 'Covered control' });
    const visible = screen.getByRole('button', { name: 'Visible control' });
    vi.spyOn(header, 'offsetHeight', 'get').mockReturnValue(52);
    vi.spyOn(header, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 360, 52));
    vi.spyOn(covered, 'getBoundingClientRect').mockReturnValue(new DOMRect(8, 5, 344, 40));
    vi.spyOn(visible, 'getBoundingClientRect').mockReturnValue(new DOMRect(8, 80, 344, 40));
    const hitTest = Object.getOwnPropertyDescriptor(document, 'elementFromPoint');
    Object.defineProperty(document, 'elementFromPoint', {
      configurable: true,
      value: () => header,
    });
    try {
      covered.focus();
      await waitFor(() =>
        expect(scrollBy).toHaveBeenCalledExactlyOnceWith({ top: -51, behavior: 'instant' }),
      );
      expect(covered).toHaveFocus();

      scrollBy.mockClear();
      // Do not scroll for a focus that has moved on before the next frame.
      visible.focus();
      covered.focus();
      visible.focus();
      await new Promise((resolve) => requestAnimationFrame(resolve));
      expect(scrollBy).not.toHaveBeenCalled();
      expect(visible).toHaveFocus();
    } finally {
      if (hitTest) Object.defineProperty(document, 'elementFromPoint', hitTest);
      else Reflect.deleteProperty(document, 'elementFromPoint');
    }
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
    expect(screen.getByRole('button', { name: 'Expand settings navigation' })).toHaveAttribute(
      'aria-controls',
      'settings-sidebar-menu',
    );
    expect(screen.getByRole('button', { name: 'Expand settings navigation' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
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

  it('keeps decorative PulseLogoIcon out of the Plans & Billing button accessible name', () => {
    const [activeTab] = createSignal<SettingsTab>('system-billing');
    const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
    const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
    const [searchQuery, setSearchQuery] = createSignal('');
    const billingTabs: SettingsNavGroup['items'] = [
      { id: 'system-billing', label: 'Plans & Billing', icon: PulseLogoIcon },
    ];

    render(() => (
      <SettingsPageShell
        headerMeta={() => ({ title: 'Plans & Billing', description: 'Plan and license.' })}
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
        filteredTabGroups={() => [{ id: 'system', label: 'System', items: billingTabs }]}
        flatTabs={() => billingTabs}
        activeTab={activeTab}
        setActiveTab={() => undefined}
        isPro={() => false}
      >
        <div>Billing content</div>
      </SettingsPageShell>
    ));

    const button = screen.getByRole('button', { name: 'Plans & Billing' });
    expect(button).toBeInTheDocument();
    expect(button.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(button.querySelector('svg title')).toBeNull();
  });
});
