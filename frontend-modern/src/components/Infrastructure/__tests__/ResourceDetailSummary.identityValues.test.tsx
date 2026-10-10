import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { InlineResourceSummaryTables } from '../ResourceDetailSummary';
import type { UseResourceDetailDrawerStateResult } from '../useResourceDetailDrawerState';

const drawerState = (
  overrides: Partial<Record<keyof UseResourceDetailDrawerStateResult, unknown>> = {},
) =>
  ({
    primaryIdentityRows: () => [],
    identityIpValues: () => [],
    identityAliasValues: () => [],
    aliasPreviewValues: () => [],
    hasAliasOverflow: () => false,
    identityCardHasRichData: () => true,
    sourceSummary: () => null,
    lastSeen: () => '',
    lastSeenAbsolute: () => '',
    ...overrides,
  }) as unknown as UseResourceDetailDrawerStateResult;

const pod: Resource = {
  id: 'pod-1',
  name: 'checkout-api-6c746d5bcf-c7z2p',
  displayName: 'checkout-api-6c746d5bcf-c7z2p',
  platformId: 'k8s-production-1-agent',
  platformType: 'kubernetes',
  sourceType: 'agent',
  type: 'pod',
  status: 'online',
  uptime: 3_600,
  lastSeen: Date.parse('2026-10-08T07:00:00Z'),
};

const container: Resource = {
  id: 'container-1',
  name: 'customer-portal',
  displayName: 'customer-portal',
  platformId: 'docker-host-1',
  platformType: 'docker',
  sourceType: 'agent',
  type: 'app-container',
  status: 'running',
  lastSeen: Date.parse('2026-10-08T07:00:00Z'),
  docker: {
    image: 'ghcr.io/pulse-demo/customer-portal:2026.04',
    labels: {
      'com.docker.compose.project': 'auth-service-01',
      'io.containers.capabilities': 'CHOWN,DAC_OVERRIDE,SETUID,SETGID,NET_BIND_SERVICE',
    },
    podman: {
      podName: 'customer-portal-pod',
      podId: '5ff108b94376a2c1d7e0f4b9c3a85e6d1f2b7c94a0e3d6f8b1c5a7e9d2f4b6a8',
    },
  },
};

// The span that holds a plain detail row's value, found by its full text.
const valueSpan = (section: HTMLElement, value: string): HTMLElement => {
  const span = within(section).getByText(value);
  expect(span.tagName).toBe('SPAN');
  return span;
};

const expectWrapped = (span: HTMLElement) => {
  expect(span.className).toContain('whitespace-normal');
  expect(span.className).not.toContain('truncate');
};

afterEach(cleanup);

describe('resource drawer identity values', () => {
  it('shows identity rows whole, since their distinguishing part is at the end', () => {
    render(() => (
      <InlineResourceSummaryTables
        resource={pod}
        drawer={drawerState({
          primaryIdentityRows: () => [
            { label: 'Hostname', value: 'checkout-api-6c746d5bcf-c7z2p' },
            {
              label: 'Primary ID',
              value: 'pod:k8s:k8s-production-1:pod:nginx-d2541-3-ec73ea11c6',
            },
            { label: 'Parent', value: 'k8s-cluster-afcb623d93ff2ae7' },
            { label: 'Discovery', value: 'pod:nginx-d2541-3-ec73ea11c6' },
          ],
          identityIpValues: () => ['2001:db8:85a3:0:0:8a2e:370:7334'],
        })}
        showPlatformId
      />
    ));

    const identity = screen.getByTestId('resource-identity-section');
    for (const value of [
      'checkout-api-6c746d5bcf-c7z2p',
      'pod:k8s:k8s-production-1:pod:nginx-d2541-3-ec73ea11c6',
      'k8s-cluster-afcb623d93ff2ae7',
      'pod:nginx-d2541-3-ec73ea11c6',
      'k8s-production-1-agent',
    ]) {
      expectWrapped(valueSpan(identity, value));
    }
    expectWrapped(
      valueSpan(
        screen.getByTestId('resource-current-state-section'),
        '2001:db8:85a3:0:0:8a2e:370:7334',
      ),
    );

    // Descriptive rows keep the single-line default.
    const runtime = screen.getByTestId('resource-runtime-context-section');
    expect(valueSpan(runtime, 'online').className).toContain('truncate');
  });

  it('wraps container image references, pod and compose names, and label chips', () => {
    render(() => (
      <InlineResourceSummaryTables
        resource={container}
        drawer={drawerState()}
        showPlatformId={false}
      />
    ));

    const section = screen.getByTestId('resource-docker-container-section');
    for (const value of [
      'ghcr.io/pulse-demo/customer-portal:2026.04',
      'customer-portal-pod',
      '5ff108b94376a2c1d7e0f4b9c3a85e6d1f2b7c94a0e3d6f8b1c5a7e9d2f4b6a8',
      'auth-service-01',
    ]) {
      expectWrapped(valueSpan(section, value));
    }

    // An inline-flex chip draws no ellipsis, so a truncating one would cut a
    // long label silently; the shared badge wraps it instead.
    const chip = within(section).getByTitle(
      'io.containers.capabilities: CHOWN,DAC_OVERRIDE,SETUID,SETGID,NET_BIND_SERVICE',
    );
    expect(chip.textContent).toBe(
      'io.containers.capabilities: CHOWN,DAC_OVERRIDE,SETUID,SETGID,NET_BIND_SERVICE',
    );
    expect(chip.className).toContain('wrap-anywhere');
    expect(chip.className).toContain('max-w-full');
    expect(chip.className).not.toContain('truncate');
  });
});
