import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DiagnosticsData } from '@/components/Settings/diagnosticsModel';

type UseDiagnosticsPanelStateModule = typeof import('../useDiagnosticsPanelState');
type URLStaticWithBlobMethods = {
  createObjectURL?: typeof URL.createObjectURL;
  revokeObjectURL?: typeof URL.revokeObjectURL;
};

const createDiagnosticsData = (): DiagnosticsData =>
  ({
    version: '6.0.0',
    runtime: 'go',
    uptime: 3600,
    nodes: [
      {
        id: 'node-a',
        name: 'pve-01',
        host: '10.0.0.5',
        type: 'pve',
        authMethod: 'token',
        connected: false,
        error: 'dial tcp 10.0.0.5:8006: connect: connection refused',
      },
    ],
    pbs: [],
    system: {
      os: 'linux',
      arch: 'amd64',
      goVersion: 'go1.25',
      numCPU: 8,
      numGoroutine: 32,
      memoryMB: 128,
    },
    discovery: {
      enabled: true,
      configuredSubnet: '10.0.0.0/24',
      activeSubnet: '10.0.1.0/24',
      environmentOverride: 'PULSE_DISCOVERY_SUBNET=10.0.2.0/24',
      subnetAllowlist: ['10.0.0.0/24'],
      subnetBlocklist: [],
    },
    errors: ['probe failed for 10.0.0.10 after timeout'],
    commercialFunnel: {
      enabled: true,
      summary: { pricing_viewed: 4, checkout_clicked: 1 },
    },
    infrastructureOnboarding: {
      enabled: true,
      summary: { opened: 4, credentials_opened: 1 },
      platforms: [{ key: 'truenas', catalog_selected: 2, credentials_opened: 1 }],
    },
  }) as DiagnosticsData;

const readBlobText = async (blob: Blob): Promise<string> =>
  await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read exported diagnostics'));
    reader.readAsText(blob);
  });

const currentExportDate = (): string => new Date().toISOString().split('T')[0];

describe('useDiagnosticsPanelState', () => {
  let useDiagnosticsPanelState: UseDiagnosticsPanelStateModule['useDiagnosticsPanelState'];
  let apiFetchJSONMock: ReturnType<typeof vi.fn>;
  let showErrorMock: ReturnType<typeof vi.fn>;
  let showSuccessMock: ReturnType<typeof vi.fn>;
  let createObjectURLMock: ReturnType<typeof vi.fn>;
  let revokeObjectURLMock: ReturnType<typeof vi.fn>;
  let anchorClickMock: ReturnType<typeof vi.fn>;
  let createdAnchor: HTMLAnchorElement | null;
  let originalCreateObjectURL: typeof URL.createObjectURL | undefined;
  let originalRevokeObjectURL: typeof URL.revokeObjectURL | undefined;

  beforeEach(async () => {
    vi.resetModules();

    apiFetchJSONMock = vi.fn();
    showErrorMock = vi.fn();
    showSuccessMock = vi.fn();
    createObjectURLMock = vi.fn(() => 'blob:diagnostics-export');
    revokeObjectURLMock = vi.fn();
    anchorClickMock = vi.fn();
    createdAnchor = null;
    originalCreateObjectURL = URL.createObjectURL;
    originalRevokeObjectURL = URL.revokeObjectURL;

    vi.doMock('@/utils/apiClient', () => ({
      apiFetchJSON: apiFetchJSONMock,
    }));

    vi.doMock('@/utils/toast', () => ({
      showError: showErrorMock,
      showSuccess: showSuccessMock,
    }));

    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURLMock,
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURLMock,
    });

    const createElement = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation(((tagName: string) => {
      const element = createElement(tagName);
      if (tagName.toLowerCase() === 'a') {
        createdAnchor = element as HTMLAnchorElement;
        createdAnchor.click = anchorClickMock;
      }
      return element;
    }) as typeof document.createElement);

    ({ useDiagnosticsPanelState } = await import('../useDiagnosticsPanelState'));
  });

  afterEach(() => {
    if (originalCreateObjectURL) {
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        value: originalCreateObjectURL,
      });
    } else {
      delete (URL as unknown as URLStaticWithBlobMethods).createObjectURL;
    }
    if (originalRevokeObjectURL) {
      Object.defineProperty(URL, 'revokeObjectURL', {
        configurable: true,
        value: originalRevokeObjectURL,
      });
    } else {
      delete (URL as unknown as URLStaticWithBlobMethods).revokeObjectURL;
    }
    vi.restoreAllMocks();
    vi.resetModules();
  });

  it('exports full diagnostics without internal analytics fields', async () => {
    const diagnosticsData = createDiagnosticsData();
    apiFetchJSONMock.mockResolvedValue(diagnosticsData);

    const { result } = renderHook(() => useDiagnosticsPanelState());

    await result.runDiagnostics();

    await waitFor(() => expect(result.diagnosticsData()).not.toBeNull());
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/diagnostics');
    expect(showSuccessMock).toHaveBeenCalledWith('Diagnostics completed');
    expect(result.diagnosticsData()).not.toHaveProperty('commercialFunnel');
    expect(result.diagnosticsData()).not.toHaveProperty('infrastructureOnboarding');

    await result.exportDiagnostics(false);

    expect(createObjectURLMock).toHaveBeenCalledOnce();
    expect(anchorClickMock).toHaveBeenCalledOnce();
    expect(createdAnchor?.download).toBe(`pulse-diagnostics-full-${currentExportDate()}.json`);
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:diagnostics-export');
    expect(showSuccessMock).toHaveBeenCalledWith('Diagnostics exported (full)');

    const payload = JSON.parse(
      await readBlobText(createObjectURLMock.mock.calls[0][0] as Blob),
    ) as DiagnosticsData & Record<string, unknown>;

    expect(payload.commercialFunnel).toBeUndefined();
    expect(payload.infrastructureOnboarding).toBeUndefined();
    expect(payload.nodes[0].host).toBe('10.0.0.5');
  });

  it('exports sanitized diagnostics without internal analytics fields', async () => {
    apiFetchJSONMock.mockResolvedValue(createDiagnosticsData());

    const { result } = renderHook(() => useDiagnosticsPanelState());

    await result.runDiagnostics();
    await waitFor(() => expect(result.diagnosticsData()).not.toBeNull());

    await result.exportDiagnostics(true);

    expect(anchorClickMock).toHaveBeenCalledOnce();
    expect(createdAnchor?.download).toBe(`pulse-diagnostics-sanitized-${currentExportDate()}.json`);
    expect(showSuccessMock).toHaveBeenCalledWith('Diagnostics exported (sanitized)');

    const payload = JSON.parse(
      await readBlobText(createObjectURLMock.mock.calls[0][0] as Blob),
    ) as DiagnosticsData & Record<string, unknown>;

    expect(payload.nodes[0]).toEqual(
      expect.objectContaining({
        id: 'node-1',
        name: 'node-1',
        host: 'node-1',
      }),
    );
    expect(payload.discovery).toEqual(
      expect.objectContaining({
        configuredSubnet: '[REDACTED_SUBNET]',
        activeSubnet: '[REDACTED_SUBNET]',
      }),
    );
    expect(payload.errors).toEqual(['probe failed for [REDACTED_IP] after timeout']);
    expect(payload.commercialFunnel).toBeUndefined();
    expect(payload.infrastructureOnboarding).toBeUndefined();
  });

  it('blocks export until diagnostics have been run', async () => {
    const { result } = renderHook(() => useDiagnosticsPanelState());

    await result.exportDiagnostics(true);

    expect(showErrorMock).toHaveBeenCalledWith('Run diagnostics first');
    expect(createObjectURLMock).not.toHaveBeenCalled();
  });
});
