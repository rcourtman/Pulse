import { describe, expect, it, vi } from 'vitest';

import {
  buildDashboardContainerRuntimeFilterConfig,
  buildDashboardHostFilterConfig,
  buildDashboardNamespaceFilterConfig,
  buildDashboardPlatformFilterConfig,
  DASHBOARD_CONTAINER_RUNTIME_ALL_OPTION_LABEL,
  DASHBOARD_KUBERNETES_CONTEXT_ALL_OPTION_LABEL,
  DASHBOARD_KUBERNETES_CONTEXT_FILTER_LABEL,
  DASHBOARD_NAMESPACE_ALL_OPTION_LABEL,
  DASHBOARD_NODE_ALL_OPTION_LABEL,
  DASHBOARD_PLATFORM_ALL_OPTION_LABEL,
  DASHBOARD_WORKLOAD_TYPE_OPTIONS,
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
      options: [
        { value: '', label: DASHBOARD_CONTAINER_RUNTIME_ALL_OPTION_LABEL },
        { value: 'containerd', label: 'containerd' },
        { value: 'docker', label: 'docker' },
      ],
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
      label: DASHBOARD_KUBERNETES_CONTEXT_FILTER_LABEL,
      value: 'prod',
      options: [
        { value: '', label: DASHBOARD_KUBERNETES_CONTEXT_ALL_OPTION_LABEL },
        { value: 'prod', label: 'prod' },
        { value: 'stage', label: 'stage' },
      ],
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
      options: [
        { value: '', label: DASHBOARD_NODE_ALL_OPTION_LABEL },
        { value: 'cluster-a-node-a', label: 'node-a' },
      ],
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
      options: [
        { value: '', label: DASHBOARD_NAMESPACE_ALL_OPTION_LABEL },
        { value: 'default', label: 'default' },
        { value: 'kube-system', label: 'kube-system' },
      ],
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

  it('builds platform filter config when multiple platform scopes exist', () => {
    const onChange = vi.fn();

    expect(
      buildDashboardPlatformFilterConfig({
        isWorkloadsRoute: true,
        selectedPlatform: 'truenas',
        platformOptions: [
          { value: 'docker', label: 'Docker' },
          { value: 'truenas', label: 'TrueNAS' },
        ],
        onChange,
      }),
    ).toMatchObject({
      id: 'workloads-platform-filter',
      label: 'Platform',
      value: 'truenas',
      options: [
        { value: '', label: DASHBOARD_PLATFORM_ALL_OPTION_LABEL },
        { value: 'docker', label: 'Docker' },
        { value: 'truenas', label: 'TrueNAS' },
      ],
    });
  });

  it('keeps workload type filter labels in the presentation model', () => {
    expect(DASHBOARD_WORKLOAD_TYPE_OPTIONS).toEqual([
      { value: 'all', label: 'All' },
      { value: 'vm', label: 'VMs' },
      { value: 'system-container', label: 'System containers' },
      { value: 'app-container', label: 'App containers' },
      { value: 'pod', label: 'Pods' },
    ]);
  });
});
