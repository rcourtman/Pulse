import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createRoot, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { setKioskMode } from '@/utils/url';
import { PlatformSectionTabs, createPlatformTablePreview } from '../sharedPlatformPage';

afterEach(() => {
  setKioskMode(false);
  window.sessionStorage.removeItem('pulse_kiosk_mode');
  cleanup();
  window.history.replaceState({}, '', '/');
});

const SECTION_TABS = [
  { id: 'overview', label: 'Overview', path: '/proxmox/overview' },
  { id: 'storage', label: 'Storage', path: '/proxmox/storage' },
  { id: 'backups', label: 'Backups', path: '/proxmox/backups' },
] as const;

const renderSections = (active: () => (typeof SECTION_TABS)[number]['id'] = () => 'overview') =>
  render(() => (
    <Router>
      <Route
        path="/"
        component={() => (
          <PlatformSectionTabs tabs={SECTION_TABS} active={active()} ariaLabel="Proxmox sections" />
        )}
      />
    </Router>
  ));

const sectionRail = () => screen.queryByRole('navigation', { name: 'Proxmox sections' });

describe('PlatformSectionTabs in kiosk mode', () => {
  it('stays unmounted while kiosk mode is on and returns when it ends', () => {
    setKioskMode(true);
    renderSections();

    expect(sectionRail()).toBeNull();
    expect(screen.queryByRole('link', { name: 'Storage' })).toBeNull();

    setKioskMode(false);

    expect(sectionRail()).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Storage' })).toHaveAttribute(
      'href',
      '/proxmox/storage',
    );
    expect(screen.getByRole('link', { name: 'Overview' })).toHaveAttribute('aria-current', 'page');
  });

  it('removes a rendered rail when kiosk mode starts and mounts a fresh one when it ends', () => {
    renderSections();
    const before = sectionRail();
    expect(before).toBeInTheDocument();

    setKioskMode(true);
    expect(sectionRail()).toBeNull();
    expect(before?.isConnected).toBe(false);

    setKioskMode(false);
    const after = sectionRail();
    expect(after).toBeInTheDocument();
    expect(after).not.toBe(before);
  });

  it('keeps following the active section through the kiosk wrapper', () => {
    const [active, setActive] = createSignal<(typeof SECTION_TABS)[number]['id']>('overview');
    renderSections(active);
    expect(screen.getByRole('link', { name: 'Overview' })).toHaveAttribute('aria-current', 'page');

    setActive('storage');
    expect(screen.getByRole('link', { name: 'Storage' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Overview' })).not.toHaveAttribute('aria-current');

    setKioskMode(true);
    setActive('backups');
    setKioskMode(false);
    expect(screen.getByRole('link', { name: 'Backups' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Storage' })).not.toHaveAttribute('aria-current');
  });

  it('keeps the phone scroll controls working on the rail mounted after kiosk mode ends', async () => {
    renderSections();
    setKioskMode(true);
    setKioskMode(false);

    const navigation = screen.getByRole('navigation', { name: 'Proxmox sections' });
    const scrollBy = vi.fn();
    Object.defineProperties(navigation, {
      clientWidth: { configurable: true, value: 220 },
      scrollWidth: { configurable: true, value: 520 },
      scrollLeft: { configurable: true, writable: true, value: 0 },
      scrollBy: { configurable: true, value: scrollBy },
    });

    window.dispatchEvent(new Event('resize'));
    const scrollRight = await screen.findByRole('button', {
      name: 'Proxmox sections: scroll right',
    });
    scrollRight.click();
    expect(scrollBy).toHaveBeenCalledWith({ left: 160, behavior: 'smooth' });
  });
});

describe('createPlatformTablePreview in kiosk mode', () => {
  const allRows = Array.from({ length: 50 }, (_, index) => `pve${index + 1}`);

  // Effects settle once the owner returns, so each test keeps the root alive and
  // disposes it itself instead of asserting inside the root callback.
  const mountPreview = (rows: () => readonly string[] = () => allRows) =>
    createRoot((dispose) => ({
      preview: createPlatformTablePreview({ rows, limit: () => 12 }),
      dispose,
    }));

  it('shows every row with no toggle while kiosk mode is on', () => {
    setKioskMode(true);
    const { preview, dispose } = mountPreview();

    expect(preview.visibleRows()).toHaveLength(50);
    expect(preview.visibleRows().at(-1)).toBe('pve50');
    expect(preview.canExpand()).toBe(false);
    preview.toggle();
    expect(preview.expanded()).toBe(false);
    expect(preview.visibleRows()).toHaveLength(50);
    dispose();
  });

  it('returns to the capped preview when kiosk mode ends without an expansion', () => {
    const { preview, dispose } = mountPreview();
    expect(preview.visibleRows()).toHaveLength(12);
    expect(preview.canExpand()).toBe(true);

    setKioskMode(true);
    expect(preview.visibleRows()).toHaveLength(50);
    expect(preview.canExpand()).toBe(false);

    setKioskMode(false);
    expect(preview.visibleRows()).toHaveLength(12);
    expect(preview.canExpand()).toBe(true);
    dispose();
  });

  it('keeps a deliberate expansion across a kiosk round trip', () => {
    const { preview, dispose } = mountPreview();
    preview.toggle();
    expect(preview.expanded()).toBe(true);

    setKioskMode(true);
    expect(preview.expanded()).toBe(true);
    expect(preview.canExpand()).toBe(false);
    expect(preview.visibleRows()).toHaveLength(50);

    setKioskMode(false);
    expect(preview.expanded()).toBe(true);
    expect(preview.canExpand()).toBe(true);
    expect(preview.visibleRows()).toHaveLength(50);

    preview.toggle();
    expect(preview.expanded()).toBe(false);
    expect(preview.visibleRows()).toHaveLength(12);
    dispose();
  });

  it('still drops an expansion once the rows no longer exceed the cap, in or out of kiosk', () => {
    const [rows, setRows] = createSignal<readonly string[]>(allRows);
    const { preview, dispose } = mountPreview(rows);
    preview.toggle();
    expect(preview.expanded()).toBe(true);

    setKioskMode(true);
    setRows(allRows.slice(0, 5));
    expect(preview.expanded()).toBe(false);
    expect(preview.visibleRows()).toHaveLength(5);

    setKioskMode(false);
    setRows(allRows);
    expect(preview.visibleRows()).toHaveLength(12);
    expect(preview.canExpand()).toBe(true);
    dispose();
  });
});
