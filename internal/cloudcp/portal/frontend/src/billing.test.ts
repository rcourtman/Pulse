import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PortalAPIError, type PortalAPI } from './api';
import { installBillingRuntime } from './billing';
import { createPortalRuntime } from './runtime';
import type { PortalBillingFlowID, PortalBootstrapData } from './types';

const flows = {
  manage: { prefix: 'manage-inline', path: '/v1/manage/request' },
  retrieve: { prefix: 'retrieve-inline', path: '/v1/retrieve-license/request' },
  export: { prefix: 'data-export', path: '/v1/gdpr/request-export' },
  delete: { prefix: 'data-delete', path: '/v1/gdpr/request-delete' },
};
const ids = Object.keys(flows) as PortalBillingFlowID[];
function deferred() {
  var resolve!: (value: unknown) => void;
  var reject!: (error: Error) => void;
  var promise = new Promise(function(done, fail) { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}
async function settle() { for (var i = 0; i < 8; i++) await Promise.resolve(); }

describe('commercial verification runtime', function() {
  var time: number;
  var listeners: Array<[EventTarget, string, EventListenerOrEventListenerObject, boolean | AddEventListenerOptions | undefined]>;
  beforeEach(function() {
    vi.useFakeTimers();
    time = 0;
    vi.spyOn(performance, 'now').mockImplementation(() => time);
    document.body.innerHTML = ids.map(function(id) {
      var prefix = flows[id].prefix;
      return '<div id="' + (id === 'manage' ? 'manage-billing-root'
        : id === 'retrieve' ? 'retrieve-billing-root' : 'data-' + id + '-root') + '"></div>';
    }).join('');
    listeners = [];
    [document, window].forEach(function(target) {
      var add = target.addEventListener.bind(target);
      vi.spyOn(target, 'addEventListener').mockImplementation(function(type, listener, options) {
        listeners.push([target, type, listener, options]);
        add(type, listener, options);
      });
    });
  });
  afterEach(function() {
    window.dispatchEvent(new Event('pagehide'));
    listeners.forEach(function([target, type, listener, options]) { target.removeEventListener(type, listener, options); });
    vi.useRealTimers();
    vi.restoreAllMocks();
  });
  function mount() {
    var runtime = createPortalRuntime({
      authenticated: true, email: 'owner@example.com', accounts: [],
      has_self_hosted_commercial: true, bootstrap_path: '',
    });
    var post = vi.fn().mockResolvedValue({});
    installBillingRuntime({ store: runtime.store, api: { postCommercialJSON: post } as unknown as PortalAPI });
    return { store: runtime.store, post };
  }
  function node<T extends HTMLElement = HTMLElement>(id: string) { return document.getElementById(id) as T; }
  function input(id: string, value: string) {
    var field = node<HTMLInputElement>(id);
    field.value = value;
    field.dispatchEvent(new Event('input', { bubbles: true }));
  }
  function click(id: string) {
    var event = new MouseEvent('click', { bubbles: true, cancelable: true });
    node(id).dispatchEvent(event);
    return event;
  }
  function send(id: PortalBillingFlowID) { click(flows[id].prefix + '-request'); }
  function advance(ms: number) { time += ms; vi.advanceTimersByTime(ms); }

  it.each(ids.flatMap(first => ids.map(second => [first, second] as const)))(
    'shares a mailbox send window from %s to %s without hiding the earlier code',
    async function(first, second) {
      var { store, post } = mount();
      send(first); await settle();
      input(flows[first].prefix + '-code', '123456');
      send(second);
      if (first !== 'retrieve') click(flows[first].prefix + '-resend');
      expect(post).toHaveBeenCalledTimes(1);
      expect(store.getBillingState().flows[first].codeValue).toBe('123456');
      expect(store.getBillingState().flows[first].status.message).toContain('If eligible');
      expect(store.getBillingState().flows[first].status.message).not.toMatch(/code (?:was )?sent/i);
      expect(node<HTMLButtonElement>(flows[second].prefix + '-request').disabled).toBe(true);
      if (first !== second) expect(store.getBillingState().flows[second].step2Visible).toBe(false);
      advance(60_000);
      expect(post).toHaveBeenCalledTimes(1);
      expect(node<HTMLButtonElement>(flows[second].prefix + '-request').disabled).toBe(false);
      send(second); await settle();
      expect(post).toHaveBeenCalledTimes(2);
      expect(post.mock.calls[1]).toEqual([flows[second].path, { email: 'owner@example.com' }]);
    }
  );

  it('serializes pending sends, resend and confirm; then preserves only deliberate requests', async function() {
    var { store, post } = mount();
    send('delete'); await settle();
    input('data-delete-code', '123456');
    node<HTMLInputElement>('data-delete-confirm-check').checked = true;
    node('data-delete-confirm-check').dispatchEvent(new Event('change', { bubbles: true }));
    // Local suppression must not clear code or consent.
    send('delete');
    expect(store.getBillingState().flows.delete.checkboxChecked).toBe(true);
    advance(60_000);
    var pending = deferred();
    post.mockReturnValueOnce(pending.promise);
    click('data-delete-resend');
    var hash = location.hash;
    expect(click('data-delete-resend').defaultPrevented).toBe(true);
    send('manage');
    click('data-delete-confirm');
    expect(post).toHaveBeenCalledTimes(2);
    expect(store.getBillingState().flows.delete.codeValue).toBe('');
    expect(store.getBillingState().flows.delete.checkboxChecked).toBe(false);
    expect(location.hash).toBe(hash);
    pending.resolve({}); await settle();
    expect(store.getBillingState().flows.delete.pendingEmail).toBe('owner@example.com');
    expect(store.getBillingState().flows.delete.status.message).toContain('latest code for this action');
  });

  it.each(ids)('keeps %s mailbox corrections independent and ignores a late old reply', async function(id) {
    var { store, post } = mount();
    var old = deferred();
    post.mockReturnValueOnce(old.promise);
    send(id);
    input(flows[id].prefix + '-email', 'corrected@example.com');
    send(id); await settle();
    expect(post).toHaveBeenCalledTimes(2);
    expect(store.getBillingState().flows[id].pendingEmail).toBe('corrected@example.com');
    old.resolve({}); await settle();
    expect(store.getBillingState().flows[id].pendingEmail).toBe('corrected@example.com');
    expect(store.getBillingState().flows[id].request.pending).toBe(false);
  });

  it.each([0, 502, 503, 504])('retains an uncertain %i request without retrying or claiming delivery', async function(status) {
    var { store, post } = mount();
    post.mockRejectedValueOnce(new PortalAPIError('synthetic uncertain outcome', status));
    send('retrieve'); await settle();
    expect(store.getBillingState().flows.retrieve.status.message).toContain('outcome is unknown');
    expect(store.getBillingState().flows.retrieve.step2Visible).toBe(true);
    expect(store.getBillingState().flows.retrieve.pendingEmail).toBe('owner@example.com');
    expect(node<HTMLButtonElement>('retrieve-inline-request').disabled).toBe(true);
    send('export');
    expect(post).toHaveBeenCalledTimes(1);
    input('retrieve-inline-code', '123456');
    post.mockResolvedValueOnce({ license: { token: 'fixture-only', email: 'owner@example.com' } });
    click('retrieve-inline-confirm'); await settle();
    expect(post.mock.calls[1]).toEqual(['/v1/retrieve-license', { email: 'owner@example.com', code: '123456' }]);
    expect(store.getBillingState().flows.retrieve.result).toMatchObject({ token: 'fixture-only' });
  });

  it.each([400, 429, 500])('keeps a definite %i rejection and deliberate retry available', async function(status) {
    var { store, post } = mount();
    post.mockRejectedValueOnce(new PortalAPIError('synthetic rejection', status));
    send('export'); await settle();
    expect(store.getBillingState().flows.export.status).toMatchObject({ error: true, message: 'synthetic rejection' });
    expect(store.getBillingState().flows.export.step2Visible).toBe(false);
    expect(node<HTMLButtonElement>('data-export-request').disabled).toBe(false);
    expect(post).toHaveBeenCalledTimes(1);
    send('export'); await settle();
    expect(post).toHaveBeenCalledTimes(2);
  });

  it('invalidates a different visible mailbox, code and destructive consent without replacing the email input', async function() {
    var { store, post } = mount();
    send('delete'); await settle();
    input('data-delete-code', '123456');
    node<HTMLInputElement>('data-delete-confirm-check').checked = true;
    node('data-delete-confirm-check').dispatchEvent(new Event('change', { bubbles: true }));
    var field = node<HTMLInputElement>('data-delete-email');
    field.focus();
    input('data-delete-email', 'corrected@example.com');
    expect(node('data-delete-email')).toBe(field);
    expect(document.activeElement).toBe(field);
    expect(store.getBillingState().flows.delete).toMatchObject({
      emailValue: 'corrected@example.com', pendingEmail: '', codeValue: '', checkboxChecked: false, step2Visible: false,
    });
    expect(node<HTMLInputElement>('data-delete-confirm-check').checked).toBe(false);
    expect(node('data-delete-step2').hidden).toBe(true);
    click('data-delete-confirm');
    expect(post).toHaveBeenCalledTimes(1);
    expect(node<HTMLButtonElement>('data-delete-request').disabled).toBe(false);
    // Returning to the original mailbox retains its pacing, not its old consent.
    input('data-delete-email', 'OWNER@example.com');
    expect(node<HTMLButtonElement>('data-delete-request').disabled).toBe(true);
    expect(store.getBillingState().flows.delete.checkboxChecked).toBe(false);
  });

  it('holds the displayed target while confirm is pending and blocks new same-mailbox sends', async function() {
    var { store, post } = mount();
    send('delete'); await settle();
    input('data-delete-code', '123456');
    node<HTMLInputElement>('data-delete-confirm-check').checked = true;
    var pending = deferred(); post.mockReturnValueOnce(pending.promise);
    click('data-delete-confirm');
    expect(node<HTMLInputElement>('data-delete-email').disabled).toBe(true);
    input('data-delete-email', 'wrong@example.com');
    expect(node<HTMLInputElement>('data-delete-email').value).toBe('owner@example.com');
    expect(store.getBillingState().flows.delete.emailValue).toBe('owner@example.com');
    advance(60_000);
    click('data-delete-confirm'); send('export');
    expect(post).toHaveBeenCalledTimes(2);
    pending.resolve({ message: 'Synthetic confirmation', deleted_count: 0 }); await settle();
    expect(store.getBillingState().flows.delete.pendingEmail).toBe('');
    expect(node<HTMLInputElement>('data-delete-email').disabled).toBe(false);
  });

  it('updates only send nodes on ticks, keeping focused code and consent stable', async function() {
    var { post } = mount();
    send('delete'); await settle();
    input('data-delete-code', '123456');
    node<HTMLInputElement>('data-delete-confirm-check').checked = true;
    var code = node<HTMLInputElement>('data-delete-code');
    code.focus();
    advance(2000);
    expect(node('data-delete-code')).toBe(code);
    expect(document.activeElement).toBe(code);
    expect(code.value).toBe('123456');
    expect(node<HTMLInputElement>('data-delete-confirm-check').checked).toBe(true);
    expect(node('data-delete-request').textContent).toContain('58s');
    expect(post).toHaveBeenCalledTimes(1);
  });

  it.each(['pagehide', 'anonymous', 'different-account', 'different-api'])(
    'retires mailbox pacing/private flows and fences late responses at %s',
    async function(boundary) {
      var { store, post } = mount();
      var pending = deferred(); post.mockReturnValueOnce(pending.promise);
      send('retrieve');
      store.getBillingState().flows.export.result = { private: 'fixture-only' };
      store.getBillingState().flows.delete.checkboxChecked = true;
      if (boundary === 'pagehide') window.dispatchEvent(new Event('pagehide'));
      else {
        var next: Partial<PortalBootstrapData> = { ...store.getBootstrap() };
        if (boundary === 'anonymous') { next.authenticated = false; next.email = ''; }
        if (boundary === 'different-account') next.email = 'new-owner@example.com';
        if (boundary === 'different-api') next.commercial_api_base_url = '/different-api';
        store.setBootstrap(next);
      }
      pending.resolve({}); await settle();
      ids.forEach(function(id) {
        expect(store.getBillingState().flows[id]).toMatchObject({
          pendingEmail: '', codeValue: '', result: null, checkboxChecked: false, step2Visible: false,
        });
      });
      if (boundary === 'pagehide') {
        send('retrieve');
        expect(post).toHaveBeenCalledTimes(1);
        window.dispatchEvent(new Event('pageshow'));
        input('retrieve-inline-email', 'owner@example.com');
      }
      if (boundary === 'anonymous') input('retrieve-inline-email', 'owner@example.com');
      send('retrieve'); await settle();
      expect(post).toHaveBeenCalledTimes(2);
    }
  );

  it('keeps ASCII aliases in one window but Unicode/dot/plus addresses independent', async function() {
    var { post } = mount();
    send('manage'); await settle();
    input('retrieve-inline-email', ' OWNER@EXAMPLE.COM ');
    send('retrieve'); expect(post).toHaveBeenCalledTimes(1);
    for (var email of ['K@example.com', 'k@example.com', 'name@example.com', 'n.ame@example.com', 'name+tag@example.com']) {
      input('retrieve-inline-email', email); send('retrieve'); await settle();
    }
    expect(post).toHaveBeenCalledTimes(6);
  });
});
