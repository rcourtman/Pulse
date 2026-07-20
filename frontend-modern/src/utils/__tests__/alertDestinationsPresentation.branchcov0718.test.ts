import { describe, expect, it } from 'vitest';
import {
  // Status vocabulary — the sibling test exercises getAlertDestinationsStatusLabel
  // but never imports the underlying label constants.
  ALERT_DESTINATIONS_DISABLED_LABEL,
  ALERT_DESTINATIONS_ENABLED_LABEL,
  // Panel descriptions (sibling test only asserts the two panel TITLES).
  ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION,
  // Apprise action / mode / targets copy (sibling asserts TESTING/TEST labels
  // via the helper but not the constants; mode + targets labels are unasserted).
  ALERT_DESTINATIONS_APPRISE_TEST_LABEL,
  ALERT_DESTINATIONS_APPRISE_TESTING_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL,
  ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL,
  ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_CLI,
  ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_HTTP,
  // Apprise CLI-path / server-URL / config-key / API-key / TLS / timeout
  // vocabulary blocks — none imported by the sibling test.
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP,
  ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL,
  ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HELP,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_TLS_LABEL,
  ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL,
  ALERT_DESTINATIONS_APPRISE_TLS_HELP,
  ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL,
  ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP,
  // Error-copy constants reused as comparison anchors for the new branches.
  ALERT_DESTINATIONS_APPRISE_ENABLE_FOR_TEST_ERROR,
  ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR,
  ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsLoadErrorBanner,
} from '@/utils/alertDestinationsPresentation';

// Residual branch-coverage probes for the alert-destination presentation
// module. The sibling test (alertDestinationsPresentation.test.ts) already
// exercises the boolean/enum happy arms of the helpers and the
// 'missingServerUrl' early-return of getAlertDestinationsAppriseValidationError.
// This file targets the residual:
//   (a) the two fall-through arms of getAlertDestinationsAppriseValidationError
//       that delegate to getAlertDestinationsAppriseTestError, plus the
//       defensive default when an unknown variant slips past the union, and
//   (b) the canonical UI copy constants the sibling test never imports — each
//       `export const` is its own coverage statement, so pinning them guards
//       against silent renames.

describe('alertDestinationsPresentation.branchcov0718', () => {
  describe('getAlertDestinationsAppriseValidationError — uncovered fall-through arms', () => {
    // The sibling test only invokes this wrapper with 'missingServerUrl'
    // (the early-return arm). The 'disabled' and 'missingTargets' variants
    // fall through to getAlertDestinationsAppriseTestError; those two
    // delegation edges are uncovered until exercised here.

    it("delegates 'disabled' to the enable-for-test error copy", () => {
      expect(getAlertDestinationsAppriseValidationError('disabled')).toBe(
        ALERT_DESTINATIONS_APPRISE_ENABLE_FOR_TEST_ERROR,
      );
      expect(getAlertDestinationsAppriseValidationError('disabled')).toBe(
        'Enable Apprise notifications before sending a test.',
      );
    });

    it("delegates 'missingTargets' to the missing-targets error copy", () => {
      expect(getAlertDestinationsAppriseValidationError('missingTargets')).toBe(
        ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR,
      );
      expect(getAlertDestinationsAppriseValidationError('missingTargets')).toBe(
        'Add at least one Apprise target to test CLI delivery.',
      );
    });

    it('routes an unexpected variant through the default (missing-targets) arm', () => {
      // The 'missingServerUrl' early-return is taken only on an exact match;
      // any other value (here a non-union string cast through the param
      // type) misses the early return and lands in
      // getAlertDestinationsAppriseTestError, whose own 'disabled' equality
      // also misses and defaults to the missing-targets copy. This exercises
      // both default arms in sequence via a single malformed input.
      const bogus = 'server-down' as unknown as Parameters<
        typeof getAlertDestinationsAppriseValidationError
      >[0];
      expect(getAlertDestinationsAppriseValidationError(bogus)).toBe(
        ALERT_DESTINATIONS_APPRISE_MISSING_TARGETS_ERROR,
      );
    });
  });

  describe('getAlertDestinationsLoadErrorBanner — boundary message inputs', () => {
    // The sibling test passes only a single non-empty webhook prefix. The
    // template composes via one interpolation branch; probe the empty-string
    // boundary and a second distinct prefix to confirm the join is a pure
    // concatenation with no special-casing.
    it('joins an empty leading message with the risk notice', () => {
      expect(getAlertDestinationsLoadErrorBanner('')).toBe(
        ` ${ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE}`,
      );
    });

    it('joins the canonical config-load prefix with the risk notice', () => {
      const prefix =
        'Unable to load notification settings. Your existing configuration could not be retrieved.';
      expect(getAlertDestinationsLoadErrorBanner(prefix)).toBe(
        `${prefix} ${ALERT_DESTINATIONS_LOAD_ERROR_RISK_NOTICE}`,
      );
    });
  });

  describe('residual canonical copy constants', () => {
    it('exposes the enabled/disabled status vocabulary', () => {
      expect(ALERT_DESTINATIONS_ENABLED_LABEL).toBe('Enabled');
      expect(ALERT_DESTINATIONS_DISABLED_LABEL).toBe('Disabled');
    });

    it('exposes the email panel description', () => {
      expect(ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION).toBe(
        'Configure SMTP delivery for alert emails.',
      );
    });

    it('exposes the apprise panel description', () => {
      expect(ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION).toBe(
        'Relay grouped alerts through Apprise by using the CLI or a remote API.',
      );
    });

    it('exposes the apprise test-action label constants', () => {
      expect(ALERT_DESTINATIONS_APPRISE_TEST_LABEL).toBe('Send test');
      expect(ALERT_DESTINATIONS_APPRISE_TESTING_LABEL).toBe('Testing…');
    });

    it('exposes the apprise delivery-mode vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_MODE_LABEL).toBe('Delivery mode');
      expect(ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL).toBe('Local Apprise CLI');
      expect(ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL).toBe('Remote Apprise API');
    });

    it('exposes the apprise targets label and per-mode help copy', () => {
      expect(ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL).toBe('Delivery targets');
      expect(ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_CLI).toBe(
        'Enter one Apprise URL per line. Commas are also supported.',
      );
      expect(ALERT_DESTINATIONS_APPRISE_TARGETS_HELP_HTTP).toBe(
        'Optional: override the URLs defined on your Apprise API instance. Leave blank to use the server defaults.',
      );
    });

    it('exposes the apprise CLI-path vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL).toBe('CLI path');
      expect(ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER).toBe('apprise');
      expect(ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP).toBe(
        'Leave blank to use the default `apprise` executable.',
      );
    });

    it('exposes the apprise server-URL vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL).toBe('Server URL');
      expect(ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER).toBe(
        'https://apprise-api.internal:8000',
      );
      expect(ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP).toBe(
        'Point to an Apprise API endpoint such as https://host:8000.',
      );
    });

    it('exposes the apprise config-key vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL).toBe('Config key (optional)');
      expect(ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER).toBe('default');
      expect(ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP).toBe(
        'Targets the /notify/<key> endpoint when provided.',
      );
    });

    it('exposes the apprise API-key vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL).toBe('API key');
      expect(ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER).toBe('Optional API key');
      expect(ALERT_DESTINATIONS_APPRISE_API_KEY_HELP).toBe(
        'Included with each request when your Apprise API requires authentication.',
      );
    });

    it('exposes the apprise API-key header vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL).toBe('API key header');
      expect(ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER).toBe('X-API-KEY');
    });

    it('exposes the apprise TLS vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_TLS_LABEL).toBe('TLS verification');
      expect(ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL).toBe('Allow self-signed certificates');
      expect(ALERT_DESTINATIONS_APPRISE_TLS_HELP).toBe(
        'Enable only when the Apprise API uses a self-signed certificate.',
      );
    });

    it('exposes the apprise timeout vocabulary', () => {
      expect(ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL).toBe('Timeout (seconds)');
      expect(ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP).toBe(
        'Maximum time to wait for Apprise to respond.',
      );
    });
  });
});
