import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesPodsTable } from '../KubernetesPodsTable';

const makeResource = ({
  id,
  type = 'pod',
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('KubernetesPodsTable', () => {
  it('renders native Pod status, container readiness, ownership, and placement fields', () => {
    render(() => (
      <KubernetesPodsTable
        resources={[
          makeResource({
            id: 'checkout-api-6c746d5bcf-c7z2p',
            kubernetes: {
              clusterId: 'prod-euw1',
              namespace: 'services',
              nodeName: 'prod-euw1-k8s-02',
              podName: 'checkout-api-6c746d5bcf-c7z2p',
              podPhase: 'Running',
              podContainers: [
                {
                  name: 'checkout-api',
                  image: 'ghcr.io/pulse-demo/checkout-api:2026.04',
                  ready: true,
                  restartCount: 2,
                  state: 'running',
                },
                {
                  name: 'metrics-sidecar',
                  image: 'ghcr.io/pulse-demo/metrics-sidecar:1.9',
                  ready: false,
                  restartCount: 1,
                  state: 'waiting',
                },
              ],
              restarts: 3,
              ownerKind: 'Deployment',
              ownerName: 'checkout-api',
              image: 'ghcr.io/pulse-demo/checkout-api:2026.04',
              uptimeSeconds: 7_200,
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No pods"
        emptyDescription="No pods"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Pod')).toBeInTheDocument();
    expect(screen.getByText('Scope')).toBeInTheDocument();
    expect(screen.getByText('Node')).toBeInTheDocument();
    expect(screen.getByText('Phase')).toBeInTheDocument();
    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.getByText('Restarts')).toBeInTheDocument();
    expect(screen.getByText('Owner')).toBeInTheDocument();
    expect(screen.getByText('Image')).toBeInTheDocument();
    expect(screen.getByText('Age')).toBeInTheDocument();

    expect(screen.getByText('checkout-api-6c746d5bcf-c7z2p')).toBeInTheDocument();
    expect(screen.getByText('prod-euw1/services')).toBeInTheDocument();
    expect(screen.getByText('prod-euw1-k8s-02')).toBeInTheDocument();
    expect(screen.getByText('Running')).toBeInTheDocument();
    expect(screen.getByText('1/2')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Deployment/checkout-api')).toBeInTheDocument();
    expect(screen.getByText('ghcr.io/pulse-demo/checkout-api:2026.04')).toBeInTheDocument();
    expect(screen.getByText('2h')).toBeInTheDocument();
    expect(
      document.querySelector('[data-kubernetes-pod-row="checkout-api-6c746d5bcf-c7z2p"]'),
    ).not.toBeNull();
  });

  it('renders pod rows with status mapped from podPhase + container readiness, attention rows first', () => {
    render(() => (
      <KubernetesPodsTable
        resources={[
          makeResource({
            id: 'happy-pod',
            kubernetes: {
              podPhase: 'Running',
              podContainers: [{ ready: true, state: 'running' }],
            },
          }),
          makeResource({
            id: 'crashing-pod',
            kubernetes: {
              podPhase: 'Running',
              podContainers: [
                { ready: false, state: 'waiting', reason: 'CrashLoopBackOff' },
              ],
            },
          }),
          makeResource({
            id: 'not-ready-pod',
            kubernetes: {
              podPhase: 'Running',
              podContainers: [
                { ready: true, state: 'running' },
                { ready: false, state: 'running' },
              ],
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No pods"
        emptyDescription="No pods"
        showToolbar={false}
      />
    ));

    const rows = Array.from(document.querySelectorAll('[data-kubernetes-pod-row]')).map(
      (row) => row.getAttribute('data-kubernetes-pod-row'),
    );
    expect(rows).toEqual(['crashing-pod', 'not-ready-pod', 'happy-pod']);
    expect(screen.getByTitle('CrashLoopBackOff')).toHaveClass('bg-red-500');
    expect(screen.getByTitle('Not ready')).toHaveClass('bg-amber-500');
    expect(screen.getByTitle('Running')).toHaveClass('bg-emerald-500');
  });
});
