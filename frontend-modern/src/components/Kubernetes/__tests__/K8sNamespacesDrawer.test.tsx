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

  it('navigates to canonical pod workloads for the cluster', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(makeResponse([{ namespace: 'default' }]));

    render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    await waitFor(() => {
      expect(screen.getByText('default')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Open All Pods' }));

    expect(navigateMock).toHaveBeenCalledWith('/workloads?type=pod&context=cluster-a');
  });

  it('navigates to canonical pod workloads scoped by namespace', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(makeResponse([{ namespace: 'kube-system' }]));

    render(() => <K8sNamespacesDrawer cluster="cluster-a" />);

    await waitFor(() => {
      expect(screen.getByText('kube-system')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Open Pods' }));

    expect(navigateMock).toHaveBeenCalledWith(
      '/workloads?type=pod&context=cluster-a&namespace=kube-system',
    );
  });

  it('does not navigate when cluster is empty', async () => {
    render(() => <K8sNamespacesDrawer cluster="" />);

    await fireEvent.click(screen.getByRole('button', { name: 'Open All Pods' }));

    expect(navigateMock).not.toHaveBeenCalled();
  });
});
