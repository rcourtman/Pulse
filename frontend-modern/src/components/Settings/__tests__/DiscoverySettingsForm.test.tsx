import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DiscoverySettingsForm } from '../DiscoverySettingsForm';

const parseSubnetList = (value: string) => {
  const seen = new Set<string>();
  return value
    .split(',')
    .map((token) => token.trim())
    .filter((token) => {
      if (!token || token.toLowerCase() === 'auto' || seen.has(token)) {
        return false;
      }
      seen.add(token);
      return true;
    });
};

const normalizeSubnetList = (value: string) => parseSubnetList(value).join(', ');

function renderDiscoverySettingsForm(options: { initialMode?: 'auto' | 'custom' } = {}) {
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal(true);
  const [discoveryMode, setDiscoveryMode] = createSignal<'auto' | 'custom'>(
    options.initialMode ?? 'auto',
  );
  const [discoverySubnetDraft, setDiscoverySubnetDraft] = createSignal('');
  const [lastCustomSubnet, setLastCustomSubnet] = createSignal('');
  const [discoverySubnetError, setDiscoverySubnetError] = createSignal<string | undefined>();
  const [savingDiscoverySettings] = createSignal(false);
  const [envOverrides] = createSignal<Record<string, boolean>>({});

  const handleDiscoveryModeChange = vi.fn(async (mode: 'auto' | 'custom') => {
    setDiscoveryMode(mode);
    if (mode === 'auto') {
      setDiscoverySubnetDraft('');
    } else {
      setDiscoverySubnetDraft(normalizeSubnetList(lastCustomSubnet()));
    }
  });
  const commitDiscoverySubnet = vi.fn(async (value: string) => {
    setDiscoverySubnetDraft(normalizeSubnetList(value));
    return true;
  });

  render(() => (
    <DiscoverySettingsForm
      discoveryEnabled={discoveryEnabled}
      discoveryMode={discoveryMode}
      discoverySubnetDraft={discoverySubnetDraft}
      discoverySubnetError={discoverySubnetError}
      savingDiscoverySettings={savingDiscoverySettings}
      envOverrides={envOverrides}
      handleDiscoveryEnabledChange={async (enabled) => {
        setDiscoveryEnabled(enabled);
        return true;
      }}
      handleDiscoveryModeChange={handleDiscoveryModeChange}
      setDiscoveryMode={setDiscoveryMode}
      setDiscoverySubnetDraft={setDiscoverySubnetDraft}
      setDiscoverySubnetError={setDiscoverySubnetError}
      setLastCustomSubnet={setLastCustomSubnet}
      commitDiscoverySubnet={commitDiscoverySubnet}
      parseSubnetList={parseSubnetList}
      normalizeSubnetList={normalizeSubnetList}
      isValidCIDR={(value) => parseSubnetList(value).length > 0}
      currentDraftSubnetValue={() => discoverySubnetDraft()}
    />
  ));

  return {
    commitDiscoverySubnet,
    discoveryMode,
    discoverySubnetDraft,
    handleDiscoveryModeChange,
  };
}

describe('DiscoverySettingsForm', () => {
  afterEach(() => cleanup());

  it('switches scan scope when the custom subnets row is clicked', async () => {
    const { discoveryMode, handleDiscoveryModeChange } = renderDiscoverySettingsForm();

    const customScope = screen.getByRole('radio', { name: /Custom subnets \(targeted\)/i });
    fireEvent.click(customScope);

    await waitFor(() => expect(handleDiscoveryModeChange).toHaveBeenCalledWith('custom'));
    expect(discoveryMode()).toBe('custom');
    expect(customScope).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByRole('button', { name: '192.168.1.0/24' })).toBeInTheDocument();
  });

  it('commits a common custom subnet chip from the scan scope control', async () => {
    const { commitDiscoverySubnet, discoverySubnetDraft } = renderDiscoverySettingsForm({
      initialMode: 'custom',
    });

    fireEvent.click(screen.getByRole('button', { name: '192.168.0.0/24' }));

    await waitFor(() => expect(commitDiscoverySubnet).toHaveBeenCalledWith('192.168.0.0/24'));
    expect(discoverySubnetDraft()).toBe('192.168.0.0/24');
    expect(screen.getByLabelText('Discovery subnets')).toHaveValue('192.168.0.0/24');
  });
});
