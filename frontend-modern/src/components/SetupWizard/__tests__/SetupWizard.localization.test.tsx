import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import RuntimeHome from '@/pages/RuntimeHome';
import { SetupCompletionPanel } from '../SetupCompletionPanel';
import { SecurityStep } from '../steps/SecurityStep';
import { WelcomeStep } from '../steps/WelcomeStep';
import type { WizardState } from '../SetupWizard';

const apiFetchJSONMock = vi.fn();

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
  setApiToken: vi.fn(),
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
}));

vi.mock('@/utils/toast', () => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

const baseState: WizardState = {
  username: 'admin',
  password: 'password',
  apiToken: 'setup-token',
};

describe('localized setup wizard journey', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiFetchJSONMock.mockResolvedValue({
      bootstrapTokenPath: '/etc/pulse/.bootstrap_token',
      isDocker: false,
      inContainer: false,
      lxcCtid: '',
      dockerContainerName: '',
    });
  });

  afterEach(() => {
    cleanup();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('renders the welcome step in Spanish while preserving product and API tokens', async () => {
    setActiveLocale('es');

    render(() => (
      <WelcomeStep
        onNext={vi.fn()}
        bootstrapToken=""
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/security/status');
    });

    expect(screen.getByText('Bienvenido a Pulse')).toBeInTheDocument();
    expect(screen.getByText('Desbloquear configuración')).toBeInTheDocument();
    expect(screen.getByText(/Conecta una API de plataforma/)).toBeInTheDocument();
    expect(screen.getByText('La telemetría de uso está activada por defecto')).toBeInTheDocument();
    expect(screen.getByText('sudo pulse bootstrap-token')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Verificar token de bootstrap →' }),
    ).toBeInTheDocument();
  });

  it('renders the security step in German', () => {
    setActiveLocale('de');

    render(() => (
      <SecurityStep
        state={baseState}
        updateState={vi.fn()}
        bootstrapToken="bootstrap-token"
        onComplete={vi.fn()}
        onBack={vi.fn()}
      />
    ));

    expect(screen.getByText('Admin-Konto erstellen')).toBeInTheDocument();
    expect(screen.getByText('Automatisch erzeugen')).toBeInTheDocument();
    expect(screen.getByText('Auf dem naechsten Bildschirm')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Konto erstellen und fortfahren →' }),
    ).toBeInTheDocument();
  });

  it('renders the completion handoff in Spanish for the empty first-source state', () => {
    setActiveLocale('es');

    render(() => (
      <SetupCompletionPanel
        state={baseState}
        onComplete={vi.fn()}
        connectedResourcesOverride={[]}
      />
    ));

    expect(screen.getByText('Elige tu primera fuente de infraestructura')).toBeInTheDocument();
    expect(screen.getByText('Credenciales que debes guardar ahora')).toBeInTheDocument();
    expect(screen.getByText('Opciones de fuente')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Agregar infraestructura' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Instalar Pulse Agent' })).toBeInTheDocument();
  });

  it('renders the connected monitoring handoff in German without translating resource names', () => {
    setActiveLocale('de');

    render(() => (
      <SetupCompletionPanel
        state={baseState}
        onComplete={vi.fn()}
        connectedResourcesOverride={[
          {
            id: 'agent-1',
            type: 'agent',
            name: 'Tower',
            displayName: 'Tower',
            platformId: 'tower',
            platformType: 'agent',
            sourceType: 'agent',
            status: 'online',
            lastSeen: 123,
            agent: { agentId: 'agent-1' },
          },
        ]}
      />
    ));

    expect(screen.getByText('Erstes ueberwachtes System verbunden')).toBeInTheDocument();
    expect(screen.getByText('Verbunden (1 System)')).toBeInTheDocument();
    expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    expect(screen.getAllByRole('button', { name: 'Infrastruktur oeffnen' }).length).toBeGreaterThan(
      0,
    );
  });

  it('renders the runtime home handoff in Spanish', () => {
    setActiveLocale('es');

    render(() => <RuntimeHome />);

    expect(screen.getByText('Abriendo el espacio de trabajo...')).toBeInTheDocument();
  });
});
