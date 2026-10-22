import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

const navigateMock = vi.fn();
const apiFetchJSONMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

import { K8sNamespacesDrawer } from '@/components/Kubernetes/K8sNamespacesDrawer';

const makeResponse = (namespaces: Array<{ namespace: string }>) => ({
  cluster: 'cluster-a',
  data: namespaces.map((row) => ({
    namespace: row.namespace,
    pods: { total: 3, online: 3, warning: 0, offline: 0, unknown: 0 },
    deployments: { total: 1, online: 1, warning: 0, offline: 0, unknown: 0 },
  })),
});

describe('K8sNamespacesDrawer', () => {
  beforeEach(() => {
    navigateMock.mockReset();
    apiFetchJSONMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('opens the Workloads tab scoped to the whole cluster', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(makeResponse([{ namespace: 'default' }]));

    render(() => <K8sNamespacesDrawer cluster="cluster-a" scopeCluster="cluster-a-id" />);

    await waitFor(() => {
      expect(screen.getByText('default')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Open all workloads' }));

    expect(navigateMock).toHaveBeenCalledWith('/kubernetes/workloads?cluster=cluster-a-id');
  });

  it('opens the workloads scoped to the cluster and namespace of the row', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(
      makeResponse([{ namespace: 'kube-system' }, { namespace: 'default' }]),
    );

    render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    await waitFor(() => {
      expect(screen.getByText('kube-system')).toBeInTheDocument();
    });

    // Each row's action names its namespace, so a screen reader can tell the
    // rows' buttons apart. Pods and deployments share the Workloads tab.
    await fireEvent.click(screen.getByRole('button', { name: 'Open workloads in kube-system' }));
    expect(navigateMock).toHaveBeenLastCalledWith(
      '/kubernetes/workloads?cluster=cluster-a&namespace=kube-system',
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Open workloads in default' }));
    expect(navigateMock).toHaveBeenLastCalledWith(
      '/kubernetes/workloads?cluster=cluster-a&namespace=default',
    );
  });

  it('shows the failure state when the namespaces read fails', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(new Error('namespaces endpoint unavailable'));

    render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load namespaces');
    expect(screen.getByRole('alert')).toHaveTextContent('namespaces endpoint unavailable');
  });

  it('wraps long namespace names instead of truncating them', async () => {
    const longName = 'team-payments-reconciliation-batch-processing-production';
    apiFetchJSONMock.mockResolvedValueOnce(makeResponse([{ namespace: longName }]));

    render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    const name = await screen.findByText(longName);
    expect(name).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(name).not.toHaveClass('truncate');
  });

  it('does not repeat the tab label in a nested card header', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(makeResponse([{ namespace: 'default' }]));

    render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    await screen.findByText('default');
    expect(screen.queryByText('Scope Pods and Deployments by namespace')).toBeNull();
  });

  it('uses the shared platform table spacing and divider contract', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(makeResponse([{ namespace: 'default' }]));
    const { container } = render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    await waitFor(() => {
      expect(screen.getByText('default')).toBeInTheDocument();
    });

    expect(container.querySelector('table')).toHaveClass('table-fixed');
    expect(container.querySelector('thead tr')).toHaveClass('bg-surface-alt');
    expect(container.querySelector('tbody')).toHaveClass('divide-y', 'divide-border');
    expect(container.querySelector('tbody')).not.toHaveClass('divide-border-subtle');
    expect(container.querySelector('th')).toHaveClass('px-1.5', 'py-0.5');
  });

  it('does not navigate when cluster is empty', async () => {
    render(() => <K8sNamespacesDrawer cluster="" />);

    await fireEvent.click(screen.getByRole('button', { name: 'Open all workloads' }));

    expect(navigateMock).not.toHaveBeenCalled();
  });
});
