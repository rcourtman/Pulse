import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { SecurityStep } from '../steps/SecurityStep';
import type { WizardState } from '../SetupWizard';

const apiFetchJSONMock = vi.fn();
const setApiTokenMock = vi.fn();
const showErrorMock = vi.fn();

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
  setApiToken: (...args: unknown[]) => setApiTokenMock(...args),
}));

vi.mock('@/utils/toast', () => ({
  showError: (...args: unknown[]) => showErrorMock(...args),
}));

const baseState: WizardState = {
  username: 'admin',
  password: '',
  apiToken: '',
};

const stubCryptoRandom = () => {
  const getRandomValuesMock = vi.fn((array: Uint8Array) => {
    for (let index = 0; index < array.length; index += 1) {
      array[index] = index + 1;
    }
    return array;
  });

  vi.stubGlobal('crypto', {
    getRandomValues: getRandomValuesMock,
  });

  return getRandomValuesMock;
};

describe('SecurityStep', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    apiFetchJSONMock.mockResolvedValue({ success: true });
    stubCryptoRandom();
  });

  afterEach(() => {
    cleanup();
    sessionStorage.clear();
    vi.unstubAllGlobals();
  });

  it('hands off username and token but never persists the plaintext password', async () => {
    const updateState = vi.fn();
    const onComplete = vi.fn();

    render(() => (
      <SecurityStep
        state={baseState}
        updateState={updateState}
        bootstrapToken="bootstrap-token"
        onComplete={onComplete}
        onBack={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /Create Account & Continue/i }));

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalledWith(
        '/api/security/quick-setup',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    const [, requestInit] = apiFetchJSONMock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(String(requestInit.body)) as {
      username: string;
      password: string;
      apiToken: string;
      setupToken: string;
    };

    expect(body.username).toBe('admin');
    expect(body.password).toHaveLength(20);
    expect(body.apiToken).toHaveLength(48);
    expect(body.setupToken).toBe('bootstrap-token');
    expect(setApiTokenMock).toHaveBeenCalledWith(body.apiToken);
    // The password is kept in in-memory wizard state so the completion screen
    // can show it once for the user to save.
    expect(updateState).toHaveBeenCalledWith({
      username: 'admin',
      password: body.password,
      apiToken: body.apiToken,
    });

    const storedHandoff = JSON.parse(
      sessionStorage.getItem(STORAGE_KEYS.SETUP_HANDOFF) || '{}',
    ) as {
      username?: string;
      password?: string;
      apiToken?: string;
      createdAt?: string;
    };
    expect(storedHandoff).toMatchObject({
      username: 'admin',
      apiToken: body.apiToken,
    });
    // The plaintext admin password must never be written to browser storage
    // (code-scanning finding). It lives only in in-memory wizard state and is
    // shown once on the completion screen.
    expect(storedHandoff.password).toBeUndefined();
    expect(storedHandoff.createdAt).toEqual(expect.any(String));
    expect(onComplete).toHaveBeenCalledOnce();
    expect(showErrorMock).not.toHaveBeenCalled();
  });
});
