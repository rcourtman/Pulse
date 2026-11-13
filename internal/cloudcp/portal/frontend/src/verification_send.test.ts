import { describe, expect, it } from 'vitest';
import { createVerificationSendWindow, trimVerificationEmail, verificationEmailKey } from './verification_send';

describe('page-local verification send window', function() {
  it.each([
    [' \tBuyer@EXAMPLE.COM\u0085', 'buyer@example.com'],
    ['\u2003USER@EXAMPLE.COM\u3000', 'user@example.com'],
    ['K@example.com', 'K@example.com'],
    ['İ@example.com', 'İ@example.com'],
    ['\uFEFFBuyer@example.com', '\uFEFFbuyer@example.com'],
    ['name+tag@example.com', 'name+tag@example.com'],
    ['n.ame@example.com', 'n.ame@example.com'],
  ])('keeps service mailbox identity for %s', function(email, key) {
    expect(verificationEmailKey(email)).toBe(key);
    expect(trimVerificationEmail(email).includes('Buyer') || !email.includes('Buyer')).toBe(true);
  });

  it('reserves synchronously across purposes and keeps different addresses independent', function() {
    var time = 0;
    var window = createVerificationSendWindow(() => time);
    var first = window.begin('OWNER@example.com')!;
    expect(window.begin(' owner@EXAMPLE.COM ')).toBeNull();
    expect(window.begin('other@example.com')).not.toBeNull();
    time = 90_000;
    expect(window.state('owner@example.com').pending).toBe(true);
    window.settle(first, true);
    expect(window.state('owner@example.com').seconds).toBe(60);
    time += 59_999;
    expect(window.begin('owner@example.com')).toBeNull();
    time += 1;
    expect(window.begin('owner@example.com')).not.toBeNull();
  });

  it('never conflates Unicode case lookalikes, dot or plus variants', function() {
    var window = createVerificationSendWindow();
    ['k@example.com', 'K@example.com', 'i@example.com', 'İ@example.com',
      'n.ame@example.com', 'name@example.com', 'name+tag@example.com',
    ].forEach(email => expect(window.begin(email)).not.toBeNull());
  });

  it('releases a definite rejection and retires stale lease completion', function() {
    var window = createVerificationSendWindow(() => 0);
    var first = window.begin('owner@example.com')!;
    window.settle(first, false);
    var second = window.begin('owner@example.com')!;
    window.settle(first, true);
    expect(window.state('owner@example.com').pending).toBe(true);
    window.clear();
    expect(window.active()).toBe(false);
    window.settle(second, true);
    expect(window.active()).toBe(false);
    expect(window.begin('owner@example.com')).not.toBeNull();
  });

  it('reclaims expired mailbox identities without starting requests', function() {
    var time = 0;
    var window = createVerificationSendWindow(() => time);
    var first = window.begin('owner@example.com')!;
    window.settle(first, true);
    time = 60_000;
    expect(window.active()).toBe(false);
    expect(window.state('owner@example.com')).toEqual({ pending: false, seconds: 0 });
    expect(window.begin('')).toBeNull();
  });
});
