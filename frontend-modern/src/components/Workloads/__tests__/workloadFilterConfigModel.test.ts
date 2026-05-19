import { describe, expect, it, vi } from 'vitest';

import {
  buildWorkloadsContainerRuntimeFilterConfig,
  buildWorkloadsHostFilterConfig,
  buildWorkloadsNamespaceFilterConfig,
  buildWorkloadsPlatformFilterConfig,
  buildWorkloadsPlatformFilterOptions,
  WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL,
  WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL,
  WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL,
  WORKLOADS_NAMESPACE_ALL_OPTION_LABEL,
  WORKLOADS_NODE_ALL_OPTION_LABEL,
  WORKLOADS_PLATFORM_ALL_OPTION_LABEL,
  WORKLOAD_TYPE_OPTIONS,
} from '../workloadFilterConfigModel';

describe('workloadFilterConfigModel', () => {
  it('builds container runtime config only for container workload routes', () => {
    const onChange = vi.fn();

    expect(
      buildWorkloadsContainerRuntimeFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'container',
        containerRuntime: 'docker',
        runtimeOptions: ['containerd', 'docker'],
        onChange,
      }),
    ).toMatchObject({
      id: 'workloads-container-runtime-filter',
      label: 'Runtime',
      value: 'docker',
      options: [
        { value: '', label: WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL },
        { value: 'containerd', label: 'containerd' },
        { value: 'docker', label: 'docker' },
      ],
    });

    expect(
      buildWorkloadsContainerRuntimeFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'app-container',
        containerRuntime: 'docker',
        runtimeOptions: ['docker'],
        onChange,
      }),
    ).toBeDefined();

    expect(
      buildWorkloadsContainerRuntimeFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'vm',
        containerRuntime: 'docker',
        runtimeOptions: ['docker'],
        onChange,
      }),
    ).toBeUndefined();

    expect(
      buildWorkloadsContainerRuntimeFilterConfig({
        isWorkloadsRoute: false,
        allowEmbeddedScopeFilters: true,
        viewMode: 'container',
        containerRuntime: 'podman',
        runtimeOptions: ['docker', 'podman'],
        onChange,
      }),
    ).toMatchObject({
      value: 'podman',
      options: [
        { value: '', label: WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL },
        { value: 'docker', label: 'docker' },
        { value: 'podman', label: 'podman' },
      ],
    });
  });

  it('builds host filter config from pod or node scope as appropriate', () => {
    const onContextChange = vi.fn();
    const onNodeChange = vi.fn();

    expect(
      buildWorkloadsHostFilterConfig({
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
      label: WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL,
      value: 'prod',
      options: [
        { value: '', label: WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL },
        { value: 'prod', label: 'prod' },
        { value: 'stage', label: 'stage' },
      ],
    });

    expect(
      buildWorkloadsHostFilterConfig({
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
        { value: '', label: WORKLOADS_NODE_ALL_OPTION_LABEL },
        { value: 'cluster-a-node-a', label: 'node-a' },
      ],
    });
  });

  it('builds namespace filter config only when pod namespace choices exist', () => {
    const onChange = vi.fn();

    expect(
      buildWorkloadsNamespaceFilterConfig({
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
        { value: '', label: WORKLOADS_NAMESPACE_ALL_OPTION_LABEL },
        { value: 'default', label: 'default' },
        { value: 'kube-system', label: 'kube-system' },
      ],
    });

    expect(
      buildWorkloadsNamespaceFilterConfig({
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
      buildWorkloadsPlatformFilterConfig({
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
        { value: '', label: WORKLOADS_PLATFORM_ALL_OPTION_LABEL },
        { value: 'docker', label: 'Docker' },
        { value: 'truenas', label: 'TrueNAS' },
      ],
    });
  });

  it('keeps the selected platform selectable when the current type narrows available options', () => {
    expect(
      buildWorkloadsPlatformFilterOptions('proxmox-pve', [
        { value: 'truenas', label: 'TrueNAS' },
        { value: 'docker', label: 'Docker / Podman' },
      ]),
    ).toEqual([
      { value: '', label: WORKLOADS_PLATFORM_ALL_OPTION_LABEL },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'truenas', label: 'TrueNAS' },
      { value: 'docker', label: 'Docker / Podman' },
    ]);
  });

  it('keeps workload type filter labels in the presentation model', () => {
    expect(WORKLOAD_TYPE_OPTIONS).toEqual([
      { value: 'all', label: 'All' },
      { value: 'vm', label: 'VMs' },
      { value: 'container', label: 'Containers' },
      { value: 'pod', label: 'Pods' },
    ]);
  });
});
