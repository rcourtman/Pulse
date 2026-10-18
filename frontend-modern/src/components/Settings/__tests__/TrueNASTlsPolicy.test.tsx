import { fireEvent, render, renderHook, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { TrueNASAPI } from '@/api/truenas';
import { TrueNASCredentialSlot } from '../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { useTrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';

vi.mock('@/api/truenas');

const saved = {
  id: 'synthetic-nas',
  name: 'NAS fixture',
  host: 'nas.example.invalid',
  port: 443,
  apiKey: '********',
  username: 'fixture',
  useHttps: true,
  insecureSkipVerify: false,
  enabled: true,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
};

describe('mounted TrueNAS form TLS policy', () => {
  it('describes draft trust without testing, mutating or claiming saved/verified trust', () => {
    const { result: state } = renderHook(() => useTrueNASSettingsPanelState({ canLoad: () => false }));
    render(() => (
      <TrueNASCredentialSlot
        state={state}
        editingConnection={saved}
        onCancel={vi.fn()}
        onSaved={vi.fn()}
      />
    ));
    const policy = screen.getByRole('status', { name: 'TrueNAS form TLS policy' });
    const pin = screen.getByLabelText('TLS fingerprint');
    const skip = screen.getByRole('checkbox', { name: 'Skip TLS verification' });
    expect(policy).toHaveTextContent('Trusted certificate chain required');
    expect(policy).toHaveAttribute('aria-live', 'polite');
    expect(policy).toHaveAttribute('aria-atomic', 'true');
    expect(screen.getByText(/Form settings only/)).toBeInTheDocument();
    expect(pin).toHaveAccessibleDescription(/Independently verify a replacement/);
    fireEvent.input(pin, { target: { value: 'ab'.repeat(32) } });
    expect(policy).toHaveTextContent('Certificate chain and fingerprint required');
    fireEvent.click(skip);
    expect(policy).toHaveTextContent('Certificate fingerprint required');
    expect(policy).toHaveTextContent('hostname and expiry checks are skipped');
    expect(screen.queryByText('TLS verification disabled.')).not.toBeInTheDocument();
    fireEvent.input(pin, { target: { value: '' } });
    expect(policy).toHaveTextContent('Certificate identity not verified');
    fireEvent.click(skip);
    expect(policy).toHaveTextContent('Trusted certificate chain required');
    fireEvent.input(screen.getByLabelText('Host'), { target: { value: 'http://nas.example.invalid' } });
    expect(policy).toHaveTextContent('HTTP does not protect credentials');
    expect(screen.getByRole('checkbox', { name: 'Use HTTPS' })).toBeChecked();
    expect(screen.getByLabelText('Host')).toHaveAccessibleDescription(/scheme in Host takes precedence/);
    expect(skip).toHaveAccessibleDescription(/Neither a fingerprint/);
    expect(state.form()).toMatchObject({ host: 'http://nas.example.invalid', port: '443', apiKey: '', fingerprint: '', useHttps: true, insecureSkipVerify: false });
    expect(TrueNASAPI.testConnection).not.toHaveBeenCalled();
    expect(TrueNASAPI.previewConnection).not.toHaveBeenCalled();
    expect(TrueNASAPI.createConnection).not.toHaveBeenCalled();
    expect(TrueNASAPI.updateConnection).not.toHaveBeenCalled();
  });
});
