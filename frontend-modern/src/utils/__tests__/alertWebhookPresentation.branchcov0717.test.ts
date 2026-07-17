import { describe, expect, it } from 'vitest';

import {
  getAlertWebhookCustomFieldInputs,
  getAlertWebhookMutationSuccess,
  getAlertWebhookNamePlaceholder,
  getAlertWebhookSetupInstructionsTitle,
  getAlertWebhookSubmitLabel,
  getAlertWebhookSummaryLabel,
  getAlertWebhookTestLabel,
  getAlertWebhookToggleAllLabel,
  getAlertWebhookToggleLabel,
  getAlertWebhookUrlPlaceholder,
  normalizeAlertWebhookCustomFields,
} from '@/utils/alertWebhookPresentation';

// Supplemental branch coverage for alertWebhookPresentation.
// Targets the simple getter functions (fn9-fn16, fn21) that were never invoked
// by the existing tests, plus three previously-uncovered branches:
//   - b33: the `?? ''` default in getAlertWebhookCustomFieldInputs (preset key
//          absent from the normalized existing map).
//   - b39: the `&&` right operand / true arm of the pushover app_token -> token
//          migration in normalizeAlertWebhookCustomFields.
//   - b42: the analogous user_token -> user migration arm.
// Each new function exercised below drives BOTH arms of every ternary, ||, and
// switch it contains so that covering them never leaves a fresh uncovered arm.

describe('alertWebhookPresentation branch coverage (supplemental 0717)', () => {
  describe('getAlertWebhookSetupInstructionsTitle', () => {
    it('returns the canonical section title constant', () => {
      expect(getAlertWebhookSetupInstructionsTitle()).toBe('Setup Instructions');
    });
  });

  describe('getAlertWebhookNamePlaceholder', () => {
    it('echoes a provided template name (|| left operand)', () => {
      expect(getAlertWebhookNamePlaceholder('Discord Incidents')).toBe(
        'Discord Incidents',
      );
    });

    it('falls back to the default placeholder when the name is undefined (|| right operand)', () => {
      expect(getAlertWebhookNamePlaceholder(undefined)).toBe('My Webhook');
    });

    it('falls back to the default placeholder when the name is an empty string (falsy || arm)', () => {
      expect(getAlertWebhookNamePlaceholder('')).toBe('My Webhook');
    });
  });

  describe('getAlertWebhookUrlPlaceholder', () => {
    it('echoes a provided url pattern (|| left operand)', () => {
      expect(
        getAlertWebhookUrlPlaceholder('https://discord.com/api/webhooks/123'),
      ).toBe('https://discord.com/api/webhooks/123');
    });

    it('falls back to the default URL placeholder when undefined (|| right operand)', () => {
      expect(getAlertWebhookUrlPlaceholder(undefined)).toBe(
        'https://example.com/webhook',
      );
    });

    it('falls back to the default URL placeholder when the pattern is empty (falsy || arm)', () => {
      expect(getAlertWebhookUrlPlaceholder('')).toBe(
        'https://example.com/webhook',
      );
    });
  });

  describe('getAlertWebhookSummaryLabel', () => {
    it('interpolates both counts into the "<n> of <m> webhooks enabled" template', () => {
      expect(getAlertWebhookSummaryLabel(2, 5)).toBe('2 of 5 webhooks enabled');
    });

    it('renders the zero-enabled boundary without any special casing', () => {
      expect(getAlertWebhookSummaryLabel(0, 3)).toBe('0 of 3 webhooks enabled');
    });

    it('renders the all-enabled boundary verbatim', () => {
      expect(getAlertWebhookSummaryLabel(4, 4)).toBe('4 of 4 webhooks enabled');
    });
  });

  describe('getAlertWebhookToggleAllLabel', () => {
    it('returns "Enable All" when the group is currently disabled (true arm)', () => {
      expect(getAlertWebhookToggleAllLabel(true)).toBe('Enable All');
    });

    it('returns "Disable All" when the group is currently enabled (false arm)', () => {
      expect(getAlertWebhookToggleAllLabel(false)).toBe('Disable All');
    });
  });

  describe('getAlertWebhookToggleLabel', () => {
    it('returns "Enabled" for an on webhook (true arm)', () => {
      expect(getAlertWebhookToggleLabel(true)).toBe('Enabled');
    });

    it('returns "Disabled" for an off webhook (false arm)', () => {
      expect(getAlertWebhookToggleLabel(false)).toBe('Disabled');
    });
  });

  describe('getAlertWebhookTestLabel', () => {
    it('returns "Test" when not testing (early-return arm, !isTesting truthy)', () => {
      expect(getAlertWebhookTestLabel(false)).toBe('Test');
    });

    it('returns "Test" even when ascii is requested but the button is idle (idle dominates ascii)', () => {
      expect(getAlertWebhookTestLabel(false, true)).toBe('Test');
    });

    it('returns the Unicode ellipsis label while testing with the default ascii flag (false arm)', () => {
      // ALERT_WEBHOOK_TESTING_LABEL uses a single U+2026 character, distinct
      // from the ASCII three-dot variant asserted below.
      expect(getAlertWebhookTestLabel(true)).toBe('Testing…');
    });

    it('returns the ASCII three-dot label while testing with ascii=true (true arm)', () => {
      expect(getAlertWebhookTestLabel(true, true)).toBe('Testing...');
    });
  });

  describe('getAlertWebhookSubmitLabel', () => {
    it('returns "Update Webhook" when editing (true arm)', () => {
      expect(getAlertWebhookSubmitLabel(true)).toBe('Update Webhook');
    });

    it('returns "Add Webhook" when creating (false arm)', () => {
      expect(getAlertWebhookSubmitLabel(false)).toBe('Add Webhook');
    });
  });

  describe('getAlertWebhookMutationSuccess', () => {
    it('maps the "add" action to the add-success message', () => {
      expect(getAlertWebhookMutationSuccess('add')).toBe(
        'Webhook added successfully',
      );
    });

    it('maps the "update" action to the update-success message', () => {
      expect(getAlertWebhookMutationSuccess('update')).toBe(
        'Webhook updated successfully',
      );
    });

    it('maps the "delete" action to the delete-success message', () => {
      expect(getAlertWebhookMutationSuccess('delete')).toBe(
        'Webhook deleted successfully',
      );
    });

    it('routes any unrecognised action through the default arm to the delete-success message', () => {
      expect(
        getAlertWebhookMutationSuccess(
          'nuke' as unknown as 'add' | 'update' | 'delete',
        ),
      ).toBe('Webhook deleted successfully');
    });
  });

  describe('normalizeAlertWebhookCustomFields — pushover token/user migration arms', () => {
    it('migrates app_token to token when token is missing (b39 true arm)', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        app_token: 'app-key-123',
      });
      expect(normalized.token).toBe('app-key-123');
      expect(normalized.app_token).toBeUndefined();
    });

    it('does NOT overwrite an existing token with app_token (b40 false arm / short-circuit)', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        token: 'existing-token',
        app_token: 'legacy-app-token',
      });
      expect(normalized.token).toBe('existing-token');
      expect(normalized.app_token).toBeUndefined();
    });

    it('skips migration when app_token is whitespace-only (right operand falsy)', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        app_token: '   ',
      });
      expect(normalized.token).toBeUndefined();
      expect(normalized.app_token).toBeUndefined();
    });

    it('migrates user_token to user when user is missing (b42 true arm)', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        user_token: 'user-key-456',
      });
      expect(normalized.user).toBe('user-key-456');
      expect(normalized.user_token).toBeUndefined();
    });

    it('does NOT overwrite an existing user with user_token (false arm / short-circuit)', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        user: 'existing-user',
        user_token: 'legacy-user-token',
      });
      expect(normalized.user).toBe('existing-user');
      expect(normalized.user_token).toBeUndefined();
    });

    it('skips user migration when user_token is whitespace-only (right operand falsy)', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        user_token: '   ',
      });
      expect(normalized.user).toBeUndefined();
      expect(normalized.user_token).toBeUndefined();
    });

    it('strips both legacy keys and migrates both values in a single pass', () => {
      const normalized = normalizeAlertWebhookCustomFields('pushover', {
        app_token: 'a-tok',
        user_token: 'u-tok',
      });
      expect(normalized).toStrictEqual({ token: 'a-tok', user: 'u-tok' });
    });

    it('returns a shallow copy unchanged for non-pushover services (early return)', () => {
      const fields = { token: 'should-stay', custom: 'x' };
      const normalized = normalizeAlertWebhookCustomFields('slack', fields);
      expect(normalized).toStrictEqual(fields);
      expect(normalized).not.toBe(fields);
    });

    it('treats Pushover with mixed case/whitespace as pushover (trim().toLowerCase())', () => {
      const normalized = normalizeAlertWebhookCustomFields('  PushOver  ', {
        app_token: 'mixed',
      });
      expect(normalized.token).toBe('mixed');
      expect(normalized.app_token).toBeUndefined();
    });
  });

  describe('getAlertWebhookCustomFieldInputs — preset value default arm (b33)', () => {
    it('defaults each pushover preset value to empty string when no existing values are supplied (?? right operand)', () => {
      const inputs = getAlertWebhookCustomFieldInputs('pushover', {});
      expect(inputs).toHaveLength(2);
      expect(inputs).toStrictEqual([
        {
          key: 'token',
          value: '',
          label: 'Application Token',
          placeholder: 'Your Pushover application token',
          required: true,
        },
        {
          key: 'user',
          value: '',
          label: 'User Key',
          placeholder: 'Primary user key or group key',
          required: true,
        },
      ]);
    });

    it('pulls each preset value from the normalized existing map when present (?? left operand)', () => {
      const inputs = getAlertWebhookCustomFieldInputs('pushover', {
        token: 'tok-1',
        user: 'usr-1',
      });
      expect(inputs.map((i) => [i.key, i.value])).toStrictEqual([
        ['token', 'tok-1'],
        ['user', 'usr-1'],
      ]);
    });

    it('appends extra non-preset fields after the pushover presets (extras filter arm)', () => {
      const inputs = getAlertWebhookCustomFieldInputs('pushover', {
        token: 'tok',
        user: 'usr',
        priority: '1',
      });
      // Presets first, then the filtered extras (priority is not a preset key).
      expect(inputs.map((i) => i.key)).toStrictEqual([
        'token',
        'user',
        'priority',
      ]);
      const priority = inputs.find((i) => i.key === 'priority');
      expect(priority).toStrictEqual({ key: 'priority', value: '1' });
    });

    it('migrates pushover app_token/user_token before building preset inputs', () => {
      // Confirms the composition: getAlertWebhookCustomFieldInputs runs the
      // normalizer first, so app_token feeds the token preset's value.
      const inputs = getAlertWebhookCustomFieldInputs('pushover', {
        app_token: 'via-app-token',
        user_token: 'via-user-token',
      });
      expect(inputs.map((i) => [i.key, i.value])).toStrictEqual([
        ['token', 'via-app-token'],
        ['user', 'via-user-token'],
      ]);
    });

    it('returns bare {key,value} inputs for a service with no presets (presets-absent branch)', () => {
      const inputs = getAlertWebhookCustomFieldInputs('discord', {
        sound: 'ping',
        priority: '5',
      });
      expect(inputs).toStrictEqual([
        { key: 'sound', value: 'ping' },
        { key: 'priority', value: '5' },
      ]);
    });

    it('defaults to an empty existing map and yields no inputs for a preset-less service', () => {
      expect(getAlertWebhookCustomFieldInputs('slack')).toStrictEqual([]);
    });
  });
});
