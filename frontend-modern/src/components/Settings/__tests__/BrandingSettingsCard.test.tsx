import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import {
  BRAND_LOGO_MAX_BYTES,
  BrandingSettingsCard,
  brandLogoFormatForFile,
  brandingLogoPreview,
} from '../BrandingSettingsCard';

vi.mock('@/stores/license', () => ({
  hasFeature: () => true,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({ href: '/settings/billing', external: false }),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => false,
}));

function renderCard() {
  const [displayName, setDisplayName] = createSignal('');
  const [logoBase64, setLogoBase64] = createSignal('');
  const [logoFormat, setLogoFormat] = createSignal<'' | 'png' | 'jpg' | 'jpeg' | 'gif'>('');
  const [changed, setChanged] = createSignal(false);

  render(() => (
    <BrandingSettingsCard
      displayName={displayName}
      setDisplayName={setDisplayName}
      logoBase64={logoBase64}
      setLogoBase64={setLogoBase64}
      logoFormat={logoFormat}
      setLogoFormat={setLogoFormat}
      setHasUnsavedChanges={setChanged}
    />
  ));

  return { displayName, logoBase64, logoFormat, changed };
}

describe('BrandingSettingsCard', () => {
  it('normalizes supported image formats and previews plain base64', () => {
    expect(brandLogoFormatForFile({ type: 'image/jpeg', name: 'brand.bin' })).toBe('jpg');
    expect(brandLogoFormatForFile({ type: '', name: 'brand.GIF' })).toBe('gif');
    expect(brandLogoFormatForFile({ type: 'image/svg+xml', name: 'brand.svg' })).toBeNull();
    expect(brandingLogoPreview('YWJj', 'png')).toBe('data:image/png;base64,YWJj');
  });

  it('updates the application name and marks settings dirty', async () => {
    const state = renderCard();

    await fireEvent.input(screen.getByLabelText('Application name'), {
      target: { value: 'Acme Operations' },
    });

    expect(state.displayName()).toBe('Acme Operations');
    expect(state.changed()).toBe(true);
    expect(screen.getByText('Acme Operations')).toBeInTheDocument();
  });

  it('loads a bounded PNG and allows removing it', async () => {
    const state = renderCard();
    const file = new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'brand.png', {
      type: 'image/png',
    });

    await fireEvent.change(screen.getByLabelText('Header logo'), {
      target: { files: [file] },
    });

    await waitFor(() => expect(state.logoFormat()).toBe('png'));
    expect(state.logoBase64()).toMatch(/^data:image\/png;base64,/);
    expect(screen.getByTestId('branding-logo-preview')).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Remove logo' }));
    expect(state.logoBase64()).toBe('');
    expect(state.logoFormat()).toBe('');
  });

  it('rejects files larger than the persisted inline-logo boundary', async () => {
    const state = renderCard();
    const file = new File([new Uint8Array(BRAND_LOGO_MAX_BYTES + 1)], 'too-large.png', {
      type: 'image/png',
    });

    await fireEvent.change(screen.getByLabelText('Header logo'), {
      target: { files: [file] },
    });

    expect(await screen.findByRole('alert')).toHaveTextContent('36 KB or smaller');
    expect(state.logoBase64()).toBe('');
  });
});
