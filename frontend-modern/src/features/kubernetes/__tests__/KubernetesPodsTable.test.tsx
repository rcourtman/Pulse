import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesPodsTable } from '../KubernetesPodsTable';

// Generated Kubernetes names render as a truncating head and a kept tail, so
// the full name is the text of the name wrapper rather than of one text node.
const kubernetesName =
  (name: string) =>
  (_content: string, element: Element | null): boolean =>
    element?.hasAttribute('data-kubernetes-name') === true && element.textContent === name;

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
  it('keeps placement and age visible in the phone column set', () => {
    const { container } = render(() => (
      <KubernetesPodsTable
        resources={[makeResource({ id: 'checkout-api' })]}
        emptyIcon={<span />}
        emptyTitle="No pods"
        emptyDescription="No pods"
        showToolbar={false}
      />
    ));

    const headers = [...container.querySelectorAll('thead th')];
    expect(headers.find((header) => header.textContent?.includes('Pod'))).toHaveClass(
      'platform-table-mobile-w-30',
    );
    expect(headers.find((header) => header.textContent?.includes('Age'))).toHaveClass(
      'platform-table-mobile-w-10',
    );
    expect(headers.find((header) => header.textContent?.includes('Scope'))).toHaveClass(
      'platform-table-mobile-w-15',
    );
  });

  it('renders native Pod status, container readiness, ownership, and placement fields', () => {
    render(() => (
      <KubernetesPodsTable
        resources={[
          makeResource({
            id: 'checkout-api-6c746d5bcf-c7z2p',
            customUrl: 'https://checkout-pod.internal',
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
    expect(screen.getByText('Status')).toBeInTheDocument();
    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.getByText('Restarts')).toBeInTheDocument();
    expect(screen.getByText('Owner')).toBeInTheDocument();
    expect(screen.getByText('Image')).toBeInTheDocument();
    expect(screen.getByText('Age')).toBeInTheDocument();

    expect(screen.getByText(kubernetesName('checkout-api-6c746d5bcf-c7z2p'))).toBeInTheDocument();
    // One cluster in view: the scope cell shows the namespace and keeps the
    // full cluster/namespace scope on hover.
    const scopeCell = screen.getByText('services').closest('[data-kubernetes-scope]');
    expect(scopeCell).toHaveAttribute('title', 'prod-euw1/services');
    // Assistive technology gets the full scope in place of the shortened text.
    expect(screen.getByText('services')).toHaveAttribute('aria-hidden', 'true');
    expect(scopeCell?.querySelector('.sr-only')).toHaveTextContent('prod-euw1/services');
    expect(screen.getByText(kubernetesName('prod-euw1-k8s-02'))).toBeInTheDocument();
    // Narrow columns truncate the head and keep the generated tail, so
    // replicas and nodes stay distinguishable. The title keeps the full name.
    const podName = screen.getByText(kubernetesName('checkout-api-6c746d5bcf-c7z2p'));
    expect(podName).toHaveAttribute('title', 'checkout-api-6c746d5bcf-c7z2p');
    expect(podName.querySelector('[data-kubernetes-name-tail]')).toHaveTextContent('-c7z2p');
    expect(podName.querySelector('.truncate')).toHaveTextContent('checkout-api-6c746d5bcf');
    expect(
      screen
        .getByText(kubernetesName('prod-euw1-k8s-02'))
        .querySelector('[data-kubernetes-name-tail]'),
    ).toHaveTextContent('-02');
    // Raw phase is Running, but the metrics-sidecar container is not ready;
    // the status column shows the mapped label, not the phase.
    expect(screen.getByText('Not ready')).toBeInTheDocument();
    expect(screen.getByText('1/2')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Deployment/checkout-api')).toBeInTheDocument();
    expect(screen.getByText('ghcr.io/pulse-demo/checkout-api:2026.04')).toBeInTheDocument();
    expect(screen.getByText('2h')).toBeInTheDocument();
    expect(
      document.querySelector('[data-kubernetes-pod-row="checkout-api-6c746d5bcf-c7z2p"]'),
    ).not.toBeNull();
    expect(
      screen.getByRole('link', {
        name: 'Open web interface for checkout-api-6c746d5bcf-c7z2p',
      }),
    ).toHaveAttribute('href', 'https://checkout-pod.internal');
    expect(
      screen.getByText(kubernetesName('checkout-api-6c746d5bcf-c7z2p')).closest('a'),
    ).toBeNull();
  });

  it('copies a generated name without a break between its head and tail', () => {
    const { container } = render(() => (
      <KubernetesPodsTable
        resources={[
          makeResource({
            id: 'cron-nightly-backfill-28918234',
            kubernetes: {
              clusterId: 'prod-euw1',
              namespace: 'batch',
              nodeName: 'prod-euw1-k8s-02',
              podName: 'cron-nightly-backfill-28918234',
              podPhase: 'Running',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No pods"
        emptyDescription="No pods"
        showToolbar={false}
      />
    ));

    // Head and tail are flex items, so a browser copy would put a line break
    // between them; a selection inside one name copies as the name.
    const podName = screen.getByText(kubernetesName('cron-nightly-backfill-28918234'));
    expect(podName.querySelector('[data-kubernetes-name-tail]')).toHaveTextContent('-28918234');
    const copy = (selected: Node) => {
      const range = document.createRange();
      range.selectNodeContents(selected);
      window.getSelection()?.removeAllRanges();
      window.getSelection()?.addRange(range);
      const setData = vi.fn();
      const event = new Event('copy', { bubbles: true, cancelable: true });
      Object.defineProperty(event, 'clipboardData', { value: { setData } });
      podName.dispatchEvent(event);
      return { setData, prevented: event.defaultPrevented };
    };
    const inside = copy(podName);
    expect(inside.setData).toHaveBeenCalledWith('text/plain', 'cron-nightly-backfill-28918234');
    expect(inside.prevented).toBe(true);
    // A selection wider than the name (a whole row) is left to the browser.
    const wider = copy(container);
    expect(wider.setData).not.toHaveBeenCalled();
    expect(wider.prevented).toBe(false);
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
              podContainers: [{ ready: false, state: 'waiting', reason: 'CrashLoopBackOff' }],
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

    const rows = Array.from(document.querySelectorAll('[data-kubernetes-pod-row]')).map((row) =>
      row.getAttribute('data-kubernetes-pod-row'),
    );
    expect(rows).toEqual(['crashing-pod', 'not-ready-pod', 'happy-pod']);
    // The failure reason is visible cell text, not just a dot tooltip.
    expect(screen.getByText('CrashLoopBackOff')).toBeInTheDocument();
    expect(screen.getByText('Not ready')).toBeInTheDocument();
    expect(
      screen.getAllByTitle('CrashLoopBackOff').some((el) => el.classList.contains('bg-red-500')),
    ).toBe(true);
    expect(
      screen.getAllByTitle('Not ready').some((el) => el.classList.contains('bg-amber-500')),
    ).toBe(true);
    expect(
      screen.getAllByTitle('Running').some((el) => el.classList.contains('bg-emerald-500')),
    ).toBe(true);
  });

  it('keeps the cluster in the scope cell when several clusters are in view', () => {
    render(() => (
      <KubernetesPodsTable
        resources={[
          makeResource({
            id: 'eu-pod',
            kubernetes: { clusterName: 'prod-eu', namespace: 'services', podName: 'eu-pod' },
          }),
          makeResource({
            id: 'us-pod',
            kubernetes: { clusterName: 'prod-us', namespace: 'services', podName: 'us-pod' },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No pods"
        emptyDescription="No pods"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('prod-eu/services')).toBeInTheDocument();
    expect(screen.getByText('prod-us/services')).toBeInTheDocument();
  });
});
