import { describe, expect, it, vi } from 'vitest';

import {
  buildDashboardContainerRuntimeFilterConfig,
  buildDashboardHostFilterConfig,
  buildDashboardNamespaceFilterConfig,
} from '../dashboardWorkloadFilterConfigModel';

describe('dashboardWorkloadFilterConfigModel', () => {
  it('builds container runtime config only for app-container workload routes', () => {
    const onChange = vi.fn();

    expect(
      buildDashboardContainerRuntimeFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'app-container',
        containerRuntime: 'docker',
        runtimeOptions: ['containerd', 'docker'],
        onChange,
      }),
    ).toMatchObject({
      id: 'workloads-container-runtime-filter',
      label: 'Runtime',
      value: 'docker',
    });

    expect(
      buildDashboardContainerRuntimeFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'vm',
        containerRuntime: 'docker',
        runtimeOptions: ['docker'],
        onChange,
      }),
    ).toBeUndefined();
  });

  it('builds host filter config from pod or node scope as appropriate', () => {
    const onContextChange = vi.fn();
    const onNodeChange = vi.fn();

    expect(
      buildDashboardHostFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'pod',
        selectedKubernetesContext: 'prod',
        kubernetesContextOptions: ['prod', 'stage'],
        selectedNode: null,
        workloadNodeOptions: [],
        onContextChange,
        onNodeChange,
      }),
    ).toMatchObject({
      id: 'workloads-k8s-context-filter',
      label: 'Cluster',
      value: 'prod',
    });

    expect(
      buildDashboardHostFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'vm',
        selectedKubernetesContext: null,
        kubernetesContextOptions: [],
        selectedNode: 'cluster-a-node-a',
        workloadNodeOptions: [{ value: 'cluster-a-node-a', label: 'node-a' }],
        onContextChange,
        onNodeChange,
      }),
    ).toMatchObject({
      id: 'workloads-node-filter',
      label: 'Node',
      value: 'cluster-a-node-a',
    });
  });

  it('builds namespace filter config only when pod namespace choices exist', () => {
    const onChange = vi.fn();

    expect(
      buildDashboardNamespaceFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'pod',
        selectedNamespace: 'default',
        namespaceOptions: ['default', 'kube-system'],
        onChange,
      }),
    ).toMatchObject({
      id: 'workloads-k8s-namespace-filter',
      label: 'Namespace',
      value: 'default',
    });

    expect(
      buildDashboardNamespaceFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'pod',
        selectedNamespace: 'default',
        namespaceOptions: [],
        onChange,
      }),
    ).toBeUndefined();
  });
});
