import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { RolesPanel } from '../RolesPanel';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import type { Role } from '@/types/rbac';

// Keep the consuming panel, draft lifetime, RBACAPI and authenticated HTTP
// client connected. Only entitlement availability and HTTP data are synthetic.
vi.mock('../useRBACFeatureGateState', () => ({
  useRBACFeatureGateState: () => ({
    licenseReady: () => true,
    rbacEnabled: () => true,
    paywallVisible: () => false,
  }),
}));

const original = {
  id: 'policy/reader?estate=north#saved',
  name: 'Fleet auditor',
  description: 'Saved constrained role',
  parentId: 'viewer',
  priority: 17,
  permissions: [
    { action: 'read', resource: 'nodes' },
    {
      action: 'delete',
      resource: 'users',
      effect: 'deny',
      conditions: { owner: '${user}', tag: 'production' },
    },
    { action: 'future:inspect', resource: 'future/resource' },
  ],
};
const roleURL = `/api/admin/roles/${encodeURIComponent(original.id)}`;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const deferred = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((yes) => (resolve = yes));
  return { promise, resolve };
};
const fetchMock = vi.fn<typeof fetch>();
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => ['PUT', 'POST', 'DELETE'].includes(init?.method ?? ''));
const reads = () =>
  fetchMock.mock.calls.filter(([url, init]) => url === '/api/admin/roles' && !init?.method);
let saved: Role[];
let respond: () => Promise<Response>;

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID('estate/north');
  setApiToken('synthetic-role-feedback-token');
  document.cookie = 'pulse_csrf=synthetic-role-feedback-csrf; Path=/';
  saved = [structuredClone(original)];
  respond = async () => json({ error: 'Synthetic refusal' }, 422);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url, init) => {
    if (url === '/api/admin/roles' && !init?.method) return json(saved);
    if (
      (url === roleURL && init?.method === 'PUT') ||
      (url === '/api/admin/roles' && init?.method === 'POST')
    )
      return respond();
    throw new Error(`Unexpected synthetic role-feedback request: ${init?.method ?? 'GET'} ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function mount() {
  const [user, setUser] = createSignal('reader-a');
  const [allowed, setAllowed] = createSignal(true);
  const view = render(() => <RolesPanel currentUser={user()} canManage={allowed()} />);
  return { view, setUser, setAllowed };
}
async function openEdit() {
  fireEvent.click(
    await screen.findByRole('button', {
      name: `Edit role ${original.name} (Role ID: ${original.id})`,
    }),
  );
  const dialog = await screen.findByRole('dialog');
  within(dialog).getByLabelText('Description', { exact: true }).focus();
  return dialog;
}
function save(dialog: HTMLElement, operation = 'Update Role') {
  const button = within(dialog).getByRole('button', { name: operation, exact: true });
  button.focus();
  fireEvent.click(button);
}
async function focusedNotice(dialog: HTMLElement) {
  const notice = await within(dialog).findByRole('alert');
  await waitFor(() => expect(notice).toHaveFocus());
  return notice;
}

describe('role feedback through the consuming editor and real HTTP client', () => {
  it.each([400, 403, 409, 422, 429])(
    'reveals a definite %s refusal locally without changing the saved draft or replaying it',
    async (status) => {
      mount();
      respond = async () => json({ error: 'Synthetic refusal' }, status);
      const dialog = await openEdit();
      const scroller = within(dialog).getByLabelText('Description', { exact: true }).parentElement!
        .parentElement!.parentElement!;
      Object.defineProperty(scroller, 'scrollHeight', { configurable: true, value: 1234 });
      const outer = dialog.parentElement!.parentElement!;
      outer.scrollTop = 37;
      const focus = vi.spyOn(HTMLElement.prototype, 'focus');
      fireEvent.input(within(dialog).getByLabelText('Role Name'), {
        target: { value: 'Rejected draft' },
      });
      save(dialog);
      const notice = await focusedNotice(dialog);
      expect(notice).toHaveTextContent('Failed to save role');
      expect(notice).toHaveAttribute('tabindex', '-1');
      expect(focus).toHaveBeenCalledWith({ preventScroll: true });
      expect(scroller.scrollTop).toBe(1234);
      expect(outer.scrollTop).toBe(37);
      expect(writes()).toHaveLength(1);
      expect(reads()).toHaveLength(1);
      const [url, init] = writes()[0];
      expect(url).toBe(roleURL);
      expect(init).toMatchObject({
        method: 'PUT',
        headers: {
          'X-Pulse-Org-ID': 'estate/north',
          'X-API-Token': 'synthetic-role-feedback-token',
          'X-CSRF-Token': 'synthetic-role-feedback-csrf',
        },
      });
      expect(JSON.parse(String(init?.body))).toEqual({ ...original, name: 'Rejected draft' });
      expect(saved).toEqual([original]);
      expect(within(dialog).getByLabelText('Role Name')).toHaveValue('Rejected draft');
      expect(within(dialog).getByLabelText('Permission 3 resource')).toHaveValue('future/resource');
      for (const name of ['Close role editor', 'Cancel', 'Update Role'])
        expect(within(dialog).getByRole('button', { name, exact: true })).toBeEnabled();
      const description = within(dialog).getByLabelText('Description', { exact: true });
      description.focus();
      fireEvent.input(description, { target: { value: 'Still editable' } });
      await Promise.resolve();
      expect(description).toHaveFocus();
      expect(writes()).toHaveLength(1);
    },
  );

  it('reveals repeated identical missing-ID validation without mounting a new notice or sending HTTP', async () => {
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'New Role' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.input(within(dialog).getByLabelText('Role Name'), {
      target: { value: 'New auditor' },
    });
    save(dialog, 'Create Role');
    const notice = await focusedNotice(dialog);
    expect(notice).toHaveTextContent('ID and Name are required');
    within(dialog).getByLabelText('Description', { exact: true }).focus();
    save(dialog, 'Create Role');
    expect(await focusedNotice(dialog)).toBe(notice);
    expect(writes()).toHaveLength(0);
    expect(reads()).toHaveLength(1);
    expect(within(dialog).getByLabelText('Role Name')).toHaveValue('New auditor');
  });

  it('reveals a second deliberate refused save, but never repeats a pending write', async () => {
    mount();
    const dialog = await openEdit();
    save(dialog);
    await focusedNotice(dialog);
    const pending = deferred();
    respond = () => pending.promise;
    save(dialog);
    await waitFor(() => expect(writes()).toHaveLength(2));
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Saving...' })).toBeDisabled();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Saving...' }));
    expect(writes()).toHaveLength(2);
    pending.resolve(json({ error: 'Synthetic refusal' }, 422));
    await focusedNotice(dialog);
    expect(reads()).toHaveLength(1);
  });

  it('leaves unknown-outcome focus and read-only saved-inventory recovery with the existing owner', async () => {
    mount();
    respond = async () => json({ error: 'Synthetic response unavailable' }, 503);
    const dialog = await openEdit();
    save(dialog);
    const check = await screen.findByRole('button', { name: 'Check saved roles' });
    const notice = screen.getByRole('alert');
    await waitFor(() => expect(notice).toHaveFocus());
    expect(dialog).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New Role' })).toBeDisabled();
    fireEvent.click(check);
    await waitFor(() => expect(reads()).toHaveLength(2));
    expect(writes()).toHaveLength(1);
  });

  it('cannot focus a replacement reader editor after an old request finally refuses', async () => {
    const { setUser } = mount();
    const pending = deferred();
    respond = () => pending.promise;
    const old = await openEdit();
    save(old);
    await waitFor(() => expect(writes()).toHaveLength(1));
    setUser('reader-b');
    await waitFor(() => expect(old).not.toBeInTheDocument());
    const current = await openEdit();
    const description = within(current).getByLabelText('Description', { exact: true });
    description.focus();
    pending.resolve(json({ error: 'Late synthetic refusal' }, 422));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(description).toHaveFocus();
    expect(within(current).queryByRole('alert')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
  });

  it('does not resurrect feedback after permission withdrawal or unmount', async () => {
    const { view, setAllowed } = mount();
    const pending = deferred();
    respond = () => pending.promise;
    const dialog = await openEdit();
    save(dialog);
    await waitFor(() => expect(writes()).toHaveLength(1));
    setAllowed(false);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    view.unmount();
    const target = document.createElement('button');
    document.body.append(target);
    target.focus();
    pending.resolve(json({ error: 'Late synthetic refusal' }, 422));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(target).toHaveFocus();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
    target.remove();
  });

  it('keeps successful readback on the saved inventory rather than revealing obsolete errors', async () => {
    mount();
    respond = async () => {
      saved = [{ ...original, name: 'Saved auditor' }];
      return json(saved[0]);
    };
    const dialog = await openEdit();
    fireEvent.input(within(dialog).getByLabelText('Role Name'), {
      target: { value: 'Saved auditor' },
    });
    save(dialog);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    await waitFor(() => expect(reads()).toHaveLength(2));
    expect(await screen.findByText('Saved auditor')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
  });
});
