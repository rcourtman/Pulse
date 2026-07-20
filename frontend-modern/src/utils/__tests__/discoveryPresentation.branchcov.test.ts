import { describe, expect, it } from 'vitest';

import type { DiscoveryFact, ResourceDiscovery } from '@/types/discovery';
import {
  getDiscoveryIdentifiedSummary,
  getDiscoverySuggestedURLReason,
  getDiscoveryURLSuggestionSourceLabel,
  getNetworkDiscoverySectionPresentation,
  hasMeaningfulDiscoveryContext,
} from '@/utils/discoveryPresentation';

// Baseline discovery record carries no meaningful signal: every "unknown" /
// empty value, no ports, paths, facts, url or probe. hasMeaningfulDiscoveryContext
// returns false for it, so individual tests add exactly one signal to isolate a branch.
const makeDiscovery = (overrides: Partial<ResourceDiscovery> = {}): ResourceDiscovery =>
  ({
    id: 'disc-1',
    resource_type: 'docker',
    resource_id: 'res-1',
    target_id: 'agent-1',
    hostname: 'host',
    service_type: 'unknown',
    service_name: 'unknown',
    service_version: 'unknown',
    category: 'unknown',
    cli_access: '',
    facts: [],
    config_paths: [],
    data_paths: [],
    log_paths: [],
    ports: [],
    user_notes: '',
    user_secrets: {},
    confidence: 0,
    ai_reasoning: '',
    discovered_at: '',
    updated_at: '',
    scan_duration: 0,
    ...overrides,
  }) as ResourceDiscovery;

// Default fact is meaningful (value 'running', category 'service'): a record that
// carries only this fact has meaningful context. Tests override one field at a time
// to steer isMeaningfulDiscoveryFact down each rejection branch.
const makeFact = (overrides: Partial<DiscoveryFact> = {}): DiscoveryFact => ({
  category: 'service',
  key: 'status',
  value: 'running',
  source: 'systemd',
  confidence: 1,
  discovered_at: '2026-06-01T00:00:00Z',
  ...overrides,
});

describe('getDiscoveryURLSuggestionSourceLabel (branch coverage)', () => {
  it.each<[string, string]>([
    ['service_default_match', 'Known service default'],
    ['service_default_variation_match', 'Known service variant'],
    ['web_port_inference', 'Detected web port'],
    ['host_management_profile_proxmox_node', 'Proxmox node profile'],
    ['host_management_profile_linked_proxmox_node', 'Proxmox node profile'],
    ['host_management_profile_pve', 'Proxmox node profile'],
    ['host_management_profile_pbs', 'Proxmox Backup profile'],
    ['host_management_profile_pmg', 'Proxmox Mail Gateway profile'],
    ['host_management_profile_nas', 'NAS node profile'],
    ['unrecognized_code', 'Discovery heuristic'],
    ['', 'Discovery heuristic'],
  ])('maps %p to %p', (code, label) => {
    expect(getDiscoveryURLSuggestionSourceLabel(code)).toBe(label);
  });

  it('treats null and undefined codes as the empty default branch', () => {
    expect(getDiscoveryURLSuggestionSourceLabel(null)).toBe('Discovery heuristic');
    expect(getDiscoveryURLSuggestionSourceLabel(undefined)).toBe('Discovery heuristic');
  });

  it('trims surrounding whitespace before matching a known case', () => {
    expect(getDiscoveryURLSuggestionSourceLabel('  web_port_inference  ')).toBe(
      'Detected web port',
    );
    expect(getDiscoveryURLSuggestionSourceLabel('\tservice_default_match\n')).toBe(
      'Known service default',
    );
  });
});

describe('getDiscoverySuggestedURLReason (branch coverage)', () => {
  it('returns empty text/title when discovery is null or undefined', () => {
    expect(getDiscoverySuggestedURLReason(null)).toEqual({ text: '', title: '' });
    expect(getDiscoverySuggestedURLReason(undefined)).toEqual({ text: '', title: '' });
  });

  it('uses the source label for both text and title when only a code is present', () => {
    expect(
      getDiscoverySuggestedURLReason({ suggested_url_source_code: 'web_port_inference' }),
    ).toEqual({
      text: 'Detected web port',
      title: 'Detected web port',
    });
  });

  it('falls back to the generic heuristic title and empty text with neither code nor detail', () => {
    expect(getDiscoverySuggestedURLReason({})).toEqual({ text: '', title: 'Discovery heuristic' });
  });

  it('capitalises a lowercase detail sentence and pairs it with the default label title', () => {
    expect(
      getDiscoverySuggestedURLReason({ suggested_url_source_detail: 'guessed from http banner' }),
    ).toEqual({
      text: 'Guessed from http banner',
      title: 'Discovery heuristic: guessed from http banner',
    });
  });

  it('keeps the raw detail in the title but sentences it for the text', () => {
    // toSentence trims+capitalises the detail; the title template interpolates the raw
    // detail, so a trailing-space detail surfaces verbatim in the title only.
    expect(
      getDiscoverySuggestedURLReason({
        suggested_url_source_code: 'web_port_inference',
        suggested_url_source_detail: '  detected on 8080/tcp  ',
      }),
    ).toEqual({
      text: 'Detected on 8080/tcp',
      title: 'Detected web port:   detected on 8080/tcp  ',
    });
  });
});

describe('hasMeaningfulDiscoveryContext (branch coverage)', () => {
  it('returns false for null and undefined', () => {
    expect(hasMeaningfulDiscoveryContext(null)).toBe(false);
    expect(hasMeaningfulDiscoveryContext(undefined)).toBe(false);
  });

  it('returns false for an all-unknown, empty baseline record', () => {
    expect(hasMeaningfulDiscoveryContext(makeDiscovery())).toBe(false);
  });

  it('treats non-array port/path fields as zero-length rather than throwing', () => {
    const discovery = makeDiscovery({
      ports: undefined,
      config_paths: null as unknown as string[],
      data_paths: undefined,
      log_paths: null as unknown as string[],
    });
    expect(hasMeaningfulDiscoveryContext(discovery)).toBe(false);
  });

  it('is truthy on a meaningful service_name alone', () => {
    expect(hasMeaningfulDiscoveryContext(makeDiscovery({ service_name: 'Nginx' }))).toBe(true);
  });

  it('is truthy on a meaningful service_type alone', () => {
    expect(hasMeaningfulDiscoveryContext(makeDiscovery({ service_type: 'postgres' }))).toBe(true);
  });

  it('is truthy on a meaningful service_version alone', () => {
    expect(hasMeaningfulDiscoveryContext(makeDiscovery({ service_version: '1.2.3' }))).toBe(true);
  });

  it('is truthy on a meaningful category alone', () => {
    expect(hasMeaningfulDiscoveryContext(makeDiscovery({ category: 'database' }))).toBe(true);
  });

  it('is truthy when any port is present', () => {
    expect(
      hasMeaningfulDiscoveryContext(
        makeDiscovery({
          ports: [{ port: 5432, protocol: 'tcp', process: 'postgres', address: '0.0.0.0' }],
        }),
      ),
    ).toBe(true);
  });

  it('is truthy when only config_paths are present', () => {
    expect(
      hasMeaningfulDiscoveryContext(makeDiscovery({ config_paths: ['/etc/nginx/nginx.conf'] })),
    ).toBe(true);
  });

  it('is truthy when only data_paths are present', () => {
    expect(
      hasMeaningfulDiscoveryContext(makeDiscovery({ data_paths: ['/var/lib/postgres'] })),
    ).toBe(true);
  });

  it('is truthy when only log_paths are present', () => {
    expect(hasMeaningfulDiscoveryContext(makeDiscovery({ log_paths: ['/var/log/nginx'] }))).toBe(
      true,
    );
  });

  it('is truthy when only a suggested_url is present', () => {
    expect(
      hasMeaningfulDiscoveryContext(makeDiscovery({ suggested_url: 'https://example.local' })),
    ).toBe(true);
  });

  it('is truthy when only a suggested_availability_probe is present', () => {
    expect(
      hasMeaningfulDiscoveryContext(
        makeDiscovery({
          suggested_availability_probe: {
            protocol: 'http',
            address: '192.0.2.10',
            port: 3000,
            service_name: 'Grafana',
            reason: 'service default',
          },
        }),
      ),
    ).toBe(true);
  });
});

// isMeaningfulDiscoveryFact is module-private, so it is exercised through
// hasMeaningfulDiscoveryContext using records that carry exactly one fact.
describe('isMeaningfulDiscoveryFact (via hasMeaningfulDiscoveryContext, branch coverage)', () => {
  const factOnly = (fact: DiscoveryFact): ResourceDiscovery => makeDiscovery({ facts: [fact] });

  it('rejects a fact whose value is a non-meaningful token', () => {
    // Bare stoplist tokens...
    expect(hasMeaningfulDiscoveryContext(factOnly(makeFact({ value: 'unknown' })))).toBe(false);
    expect(hasMeaningfulDiscoveryContext(factOnly(makeFact({ value: 'none' })))).toBe(false);
    expect(hasMeaningfulDiscoveryContext(factOnly(makeFact({ value: '' })))).toBe(false);
    // ...and the hyphen/underscore form of "n a", which normalises into the stoplist.
    expect(hasMeaningfulDiscoveryContext(factOnly(makeFact({ value: 'N-A' })))).toBe(false);
    expect(hasMeaningfulDiscoveryContext(factOnly(makeFact({ value: 'system_container' })))).toBe(
      false,
    );
  });

  it('treats a slash-bearing token like "n/a" as meaningful (slashes are not normalised away)', () => {
    expect(hasMeaningfulDiscoveryContext(factOnly(makeFact({ value: 'n/a' })))).toBe(true);
  });

  it('rejects a metadata-sourced status fact', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'status', source: 'metadata' }))),
    ).toBe(false);
  });

  it('rejects an availability fact whose value mentions missing', () => {
    expect(
      hasMeaningfulDiscoveryContext(
        factOnly(makeFact({ key: 'config_availability', value: 'missing config file' })),
      ),
    ).toBe(false);
  });

  it('rejects a key that starts with missing', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'missing_key', value: 'present' }))),
    ).toBe(false);
  });

  it('rejects a key that ends with missing', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'port_missing', value: 'present' }))),
    ).toBe(false);
  });

  it('rejects a value that starts with "missing "', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'cfg', value: 'missing config' }))),
    ).toBe(false);
  });

  it('rejects a value containing " missing "', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'cfg', value: 'a missing file' }))),
    ).toBe(false);
  });

  it('rejects values containing "does not exist"', () => {
    expect(
      hasMeaningfulDiscoveryContext(
        factOnly(makeFact({ key: 'cfg', value: 'file does not exist' })),
      ),
    ).toBe(false);
  });

  it('rejects values containing "not found"', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'cfg', value: 'binary not found' }))),
    ).toBe(false);
  });

  it('rejects values containing "failed"', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'cfg', value: 'command failed' }))),
    ).toBe(false);
  });

  it('rejects values containing "error"', () => {
    expect(
      hasMeaningfulDiscoveryContext(factOnly(makeFact({ key: 'cfg', value: 'connection error' }))),
    ).toBe(false);
  });

  it('rejects a fact whose category is outside the allow-list', () => {
    expect(
      hasMeaningfulDiscoveryContext(
        factOnly(makeFact({ category: 'hardware', key: 'cpu', value: 'amd ryzen' })),
      ),
    ).toBe(false);
  });

  it('accepts a meaningful fact in an allowed category', () => {
    expect(
      hasMeaningfulDiscoveryContext(
        factOnly(makeFact({ category: 'version', key: 'app_version', value: '1.2.3' })),
      ),
    ).toBe(true);
  });
});

describe('getDiscoveryIdentifiedSummary (branch coverage)', () => {
  it('zeroes out non-numeric confidence and non-array port/path counts', () => {
    const summary = getDiscoveryIdentifiedSummary(
      makeDiscovery({
        service_name: 'Nginx',
        confidence: 'high' as unknown as number,
        ports: undefined,
        config_paths: null as unknown as string[],
        data_paths: undefined,
        log_paths: null as unknown as string[],
      }),
    );
    expect(summary).not.toBeNull();
    expect(summary?.confidence).toBe(0);
    expect(summary?.confidencePercent).toBe('0%');
    expect(summary?.portCount).toBe(0);
    expect(summary?.configPathCount).toBe(0);
    expect(summary?.dataPathCount).toBe(0);
    expect(summary?.logPathCount).toBe(0);
  });

  it('labels an unnamed service "Unidentified service" and trims meaningful type/version/category', () => {
    const summary = getDiscoveryIdentifiedSummary(
      makeDiscovery({
        service_name: 'unknown',
        service_type: '  nginx  ',
        service_version: '  1.25.0  ',
        category: 'web_server',
        ports: [{ port: 80, protocol: 'tcp', process: 'nginx', address: '0.0.0.0' }],
      }),
    );
    expect(summary?.serviceName).toBe('Unidentified service');
    expect(summary?.serviceType).toBe('nginx');
    expect(summary?.serviceVersion).toBe('1.25.0');
    expect(summary?.category).toBe('web_server');
    expect(summary?.portCount).toBe(1);
  });

  it('drops non-meaningful service_type/version/category even when the record is meaningful', () => {
    const summary = getDiscoveryIdentifiedSummary(
      makeDiscovery({
        service_name: 'Redis',
        service_type: 'service',
        service_version: 'unknown',
        category: 'unknown',
      }),
    );
    expect(summary?.serviceName).toBe('Redis');
    expect(summary?.serviceType).toBeUndefined();
    expect(summary?.serviceVersion).toBeUndefined();
    expect(summary?.category).toBeUndefined();
  });

  it('prefers updated_at for observedAt and falls back to discovered_at then undefined', () => {
    const both = getDiscoveryIdentifiedSummary(
      makeDiscovery({
        service_name: 'X',
        discovered_at: '2026-06-01T00:00:00Z',
        updated_at: '2026-06-02T00:00:00Z',
      }),
    );
    expect(both?.discoveredAt).toBe('2026-06-01T00:00:00Z');
    expect(both?.observedAt).toBe('2026-06-02T00:00:00Z');

    const discoveredOnly = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'X', discovered_at: '2026-06-01T00:00:00Z' }),
    );
    expect(discoveredOnly?.observedAt).toBe('2026-06-01T00:00:00Z');

    const neither = getDiscoveryIdentifiedSummary(makeDiscovery({ service_name: 'X' }));
    expect(neither?.observedAt).toBeUndefined();
    expect(neither?.discoveredAt).toBeUndefined();
  });

  it('trims cli_access and drops whitespace-only cli_access', () => {
    const trimmed = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'X', cli_access: '  docker exec -it web /bin/sh  ' }),
    );
    expect(trimmed?.cliAccess).toBe('docker exec -it web /bin/sh');

    const blank = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'X', cli_access: '   ' }),
    );
    expect(blank?.cliAccess).toBeUndefined();
  });

  it('normalises a whitespace-only suggested_url to undefined and flips hasEndpointCandidate', () => {
    const summary = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'X', suggested_url: '   ' }),
    );
    expect(summary?.suggestedUrl).toBeUndefined();
    expect(summary?.hasEndpointCandidate).toBe(false);
  });

  it('trims a present suggested_url_diagnostic and drops an empty one', () => {
    const present = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'X', suggested_url_diagnostic: '  no host candidate  ' }),
    );
    expect(present?.suggestedUrlDiagnostic).toBe('no host candidate');

    const empty = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'X', suggested_url_diagnostic: '' }),
    );
    expect(empty?.suggestedUrlDiagnostic).toBeUndefined();
  });

  it('forwards a suggested availability probe and undefined when absent', () => {
    const probe = {
      protocol: 'http',
      address: '192.0.2.10',
      port: 3000,
      service_name: 'Grafana',
      reason: 'service default',
    };
    const withProbe = getDiscoveryIdentifiedSummary(
      makeDiscovery({ service_name: 'Grafana', suggested_availability_probe: probe }),
    );
    expect(withProbe?.suggestedAvailabilityProbe).toEqual(probe);

    const withoutProbe = getDiscoveryIdentifiedSummary(makeDiscovery({ service_name: 'Grafana' }));
    expect(withoutProbe?.suggestedAvailabilityProbe).toBeUndefined();
  });
});

describe('getNetworkDiscoverySectionPresentation (branch coverage)', () => {
  it('labels the toggle "Disabled" when discovery is disabled', () => {
    const presentation = getNetworkDiscoverySectionPresentation(false);
    expect(presentation.toggleStateLabel).toBe('Disabled');
    expect(presentation).toEqual({
      headerTitle: 'Network discovery',
      headerDescription: 'Control how Pulse scans your network for Proxmox services.',
      toggleTitle: 'Automatic scanning',
      toggleDescription:
        'Enable discovery to surface Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway endpoints automatically.',
      toggleStateLabel: 'Disabled',
      scanScopeLabel: 'Scan scope',
      commonNetworksLabel: 'Common networks',
      environmentOverrideMessage:
        'Discovery settings are locked by environment variables. Update the service configuration and restart Pulse to change them here.',
    });
  });
});
