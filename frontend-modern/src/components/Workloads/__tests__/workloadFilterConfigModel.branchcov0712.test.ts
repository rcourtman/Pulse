import { describe, expect, it, vi } from 'vitest';

import {
  buildWorkloadsClusterFilterConfig,
  buildWorkloadsContainerRuntimeFilterConfig,
  buildWorkloadsHostFilterConfig,
  buildWorkloadsNamespaceFilterConfig,
  buildWorkloadsPlatformFilterConfig,
  WORKLOADS_CLUSTER_ALL_OPTION_LABEL,
  WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL,
  WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL,
  WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL,
  WORKLOADS_NAMESPACE_ALL_OPTION_LABEL,
  WORKLOADS_NODE_ALL_OPTION_LABEL,
  WORKLOADS_PLATFORM_ALL_OPTION_LABEL,
} from '../workloadFilterConfigModel';

describe('workloadFilterConfigModel branch coverage', () => {
  describe('buildWorkloadsContainerRuntimeFilterConfig', () => {
    it('returns undefined when off the workloads route and embedded scope filters are disallowed', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsContainerRuntimeFilterConfig({
          isWorkloadsRoute: false,
          viewMode: 'container',
          containerRuntime: 'docker',
          runtimeOptions: ['docker', 'podman'],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('returns undefined for non-container view modes even on the workloads route', () => {
      const onChange = vi.fn();

      for (const viewMode of ['pod', 'all', 'vm'] as const) {
        expect(
          buildWorkloadsContainerRuntimeFilterConfig({
            isWorkloadsRoute: true,
            viewMode,
            containerRuntime: 'docker',
            runtimeOptions: ['docker', 'podman'],
            onChange,
          }),
        ).toBeUndefined();
      }
    });

    it('returns undefined when fewer than two runtime options are available', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsContainerRuntimeFilterConfig({
          isWorkloadsRoute: true,
          viewMode: 'container',
          containerRuntime: 'docker',
          runtimeOptions: [],
          onChange,
        }),
      ).toBeUndefined();

      expect(
        buildWorkloadsContainerRuntimeFilterConfig({
          isWorkloadsRoute: true,
          viewMode: 'container',
          containerRuntime: 'docker',
          runtimeOptions: ['docker'],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('builds a full runtime config for the system-container view mode', () => {
      const onChange = vi.fn();

      const result = buildWorkloadsContainerRuntimeFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'system-container',
        containerRuntime: 'podman',
        runtimeOptions: ['docker', 'podman'],
        onChange,
      });

      expect(result).toEqual({
        id: 'workloads-container-runtime-filter',
        label: 'Runtime',
        value: 'podman',
        options: [
          { value: '', label: WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL },
          { value: 'docker', label: 'docker' },
          { value: 'podman', label: 'podman' },
        ],
        onChange,
      });
      expect(result?.onChange).toBe(onChange);
    });
  });

  describe('buildWorkloadsPlatformFilterConfig', () => {
    it('returns undefined when off the workloads route and embedded scope filters are disallowed', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsPlatformFilterConfig({
          isWorkloadsRoute: false,
          selectedPlatform: 'docker',
          platformOptions: [{ value: 'docker', label: 'Docker' }],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('returns undefined when no platform options are available', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsPlatformFilterConfig({
          isWorkloadsRoute: true,
          selectedPlatform: 'docker',
          platformOptions: [],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('returns undefined for a single platform option with a blank selection', () => {
      const onChange = vi.fn();
      const singleOption = [{ value: 'docker', label: 'Docker' }];

      expect(
        buildWorkloadsPlatformFilterConfig({
          isWorkloadsRoute: true,
          selectedPlatform: null,
          platformOptions: singleOption,
          onChange,
        }),
      ).toBeUndefined();

      expect(
        buildWorkloadsPlatformFilterConfig({
          isWorkloadsRoute: true,
          selectedPlatform: '   ',
          platformOptions: singleOption,
          onChange,
        }),
      ).toBeUndefined();
    });

    it('builds a config for a single platform option when a selection is present', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsPlatformFilterConfig({
          isWorkloadsRoute: true,
          selectedPlatform: 'docker',
          platformOptions: [{ value: 'docker', label: 'Docker' }],
          onChange,
        }),
      ).toEqual({
        id: 'workloads-platform-filter',
        label: 'Platform',
        value: 'docker',
        options: [
          { value: '', label: WORKLOADS_PLATFORM_ALL_OPTION_LABEL },
          { value: 'docker', label: 'Docker' },
        ],
        onChange,
      });
    });

    it('builds a config via embedded scope, coercing a null platform to an empty value', () => {
      const onChange = vi.fn();

      const result = buildWorkloadsPlatformFilterConfig({
        isWorkloadsRoute: false,
        allowEmbeddedScopeFilters: true,
        selectedPlatform: null,
        platformOptions: [
          { value: 'docker', label: 'Docker' },
          { value: 'truenas', label: 'TrueNAS' },
        ],
        onChange,
      });

      expect(result).toEqual({
        id: 'workloads-platform-filter',
        label: 'Platform',
        value: '',
        options: [
          { value: '', label: WORKLOADS_PLATFORM_ALL_OPTION_LABEL },
          { value: 'docker', label: 'Docker' },
          { value: 'truenas', label: 'TrueNAS' },
        ],
        onChange,
      });
      expect(result?.onChange).toBe(onChange);
    });
  });

  describe('buildWorkloadsHostFilterConfig', () => {
    it('returns undefined when off the workloads route and embedded scope filters are disallowed', () => {
      const onContextChange = vi.fn();
      const onNodeChange = vi.fn();

      expect(
        buildWorkloadsHostFilterConfig({
          isWorkloadsRoute: false,
          viewMode: 'vm',
          selectedKubernetesContext: null,
          kubernetesContextOptions: [],
          selectedNode: 'node-a',
          workloadNodeOptions: [{ value: 'node-a', label: 'node-a' }],
          onContextChange,
          onNodeChange,
        }),
      ).toBeUndefined();
    });

    it('builds a kubernetes context config with a null context and empty options for pod view mode', () => {
      const onContextChange = vi.fn();
      const onNodeChange = vi.fn();

      const result = buildWorkloadsHostFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'pod',
        selectedKubernetesContext: null,
        kubernetesContextOptions: [],
        selectedNode: 'ignored',
        workloadNodeOptions: [{ value: 'ignored', label: 'ignored' }],
        onContextChange,
        onNodeChange,
      });

      expect(result).toEqual({
        id: 'workloads-k8s-context-filter',
        label: WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL,
        value: '',
        options: [{ value: '', label: WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL }],
        onChange: onContextChange,
      });
      expect(result?.onChange).toBe(onContextChange);
    });

    it('builds a node config with a null node and empty options for non-pod view modes', () => {
      const onContextChange = vi.fn();
      const onNodeChange = vi.fn();

      const result = buildWorkloadsHostFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'container',
        selectedKubernetesContext: 'ignored',
        kubernetesContextOptions: ['ignored'],
        selectedNode: null,
        workloadNodeOptions: [],
        onContextChange,
        onNodeChange,
      });

      expect(result).toEqual({
        id: 'workloads-node-filter',
        label: 'Node',
        value: '',
        options: [{ value: '', label: WORKLOADS_NODE_ALL_OPTION_LABEL }],
        onChange: onNodeChange,
      });
      expect(result?.onChange).toBe(onNodeChange);
    });

    it('builds configs via embedded scope for both pod and node view modes', () => {
      const onContextChange = vi.fn();
      const onNodeChange = vi.fn();

      const podResult = buildWorkloadsHostFilterConfig({
        isWorkloadsRoute: false,
        allowEmbeddedScopeFilters: true,
        viewMode: 'pod',
        selectedKubernetesContext: 'ctx-a',
        kubernetesContextOptions: ['ctx-a'],
        selectedNode: null,
        workloadNodeOptions: [],
        onContextChange,
        onNodeChange,
      });

      expect(podResult).toEqual({
        id: 'workloads-k8s-context-filter',
        label: WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL,
        value: 'ctx-a',
        options: [
          { value: '', label: WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL },
          { value: 'ctx-a', label: 'ctx-a' },
        ],
        onChange: onContextChange,
      });

      const nodeResult = buildWorkloadsHostFilterConfig({
        isWorkloadsRoute: false,
        allowEmbeddedScopeFilters: true,
        viewMode: 'vm',
        selectedKubernetesContext: null,
        kubernetesContextOptions: [],
        selectedNode: 'node-1',
        workloadNodeOptions: [{ value: 'node-1', label: 'node-1' }],
        onContextChange,
        onNodeChange,
      });

      expect(nodeResult).toEqual({
        id: 'workloads-node-filter',
        label: 'Node',
        value: 'node-1',
        options: [
          { value: '', label: WORKLOADS_NODE_ALL_OPTION_LABEL },
          { value: 'node-1', label: 'node-1' },
        ],
        onChange: onNodeChange,
      });
    });
  });

  describe('buildWorkloadsNamespaceFilterConfig', () => {
    it('returns undefined when off the workloads route and embedded scope filters are disallowed', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsNamespaceFilterConfig({
          isWorkloadsRoute: false,
          viewMode: 'pod',
          selectedNamespace: 'default',
          namespaceOptions: ['default'],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('returns undefined for non-pod view modes', () => {
      const onChange = vi.fn();

      for (const viewMode of ['vm', 'container', 'all'] as const) {
        expect(
          buildWorkloadsNamespaceFilterConfig({
            isWorkloadsRoute: true,
            viewMode,
            selectedNamespace: 'default',
            namespaceOptions: ['default', 'kube-system'],
            onChange,
          }),
        ).toBeUndefined();
      }
    });

    it('builds a config via embedded scope, coercing a null namespace to an empty value', () => {
      const onChange = vi.fn();

      const result = buildWorkloadsNamespaceFilterConfig({
        isWorkloadsRoute: false,
        allowEmbeddedScopeFilters: true,
        viewMode: 'pod',
        selectedNamespace: null,
        namespaceOptions: ['default', 'kube-system'],
        onChange,
      });

      expect(result).toEqual({
        id: 'workloads-k8s-namespace-filter',
        label: 'Namespace',
        value: '',
        options: [
          { value: '', label: WORKLOADS_NAMESPACE_ALL_OPTION_LABEL },
          { value: 'default', label: 'default' },
          { value: 'kube-system', label: 'kube-system' },
        ],
        onChange,
      });
      expect(result?.onChange).toBe(onChange);
    });
  });

  describe('buildWorkloadsClusterFilterConfig', () => {
    it('returns undefined when off the workloads route and embedded scope filters are disallowed', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsClusterFilterConfig({
          isWorkloadsRoute: false,
          viewMode: 'vm',
          selectedCluster: 'cluster-a',
          clusterOptions: ['cluster-a'],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('returns undefined for non-vm view modes', () => {
      const onChange = vi.fn();

      for (const viewMode of ['pod', 'container', 'all'] as const) {
        expect(
          buildWorkloadsClusterFilterConfig({
            isWorkloadsRoute: true,
            viewMode,
            selectedCluster: 'cluster-a',
            clusterOptions: ['cluster-a', 'cluster-b'],
            onChange,
          }),
        ).toBeUndefined();
      }
    });

    it('returns undefined when no cluster options are available', () => {
      const onChange = vi.fn();

      expect(
        buildWorkloadsClusterFilterConfig({
          isWorkloadsRoute: true,
          viewMode: 'vm',
          selectedCluster: 'cluster-a',
          clusterOptions: [],
          onChange,
        }),
      ).toBeUndefined();
    });

    it('builds a full cluster config for vm view mode, coercing a null cluster to an empty value', () => {
      const onChange = vi.fn();

      const result = buildWorkloadsClusterFilterConfig({
        isWorkloadsRoute: true,
        viewMode: 'vm',
        selectedCluster: null,
        clusterOptions: ['cluster-a', 'cluster-b'],
        onChange,
      });

      expect(result).toEqual({
        id: 'workloads-vmware-cluster-filter',
        label: 'Cluster',
        value: '',
        options: [
          { value: '', label: WORKLOADS_CLUSTER_ALL_OPTION_LABEL },
          { value: 'cluster-a', label: 'cluster-a' },
          { value: 'cluster-b', label: 'cluster-b' },
        ],
        onChange,
      });
      expect(result?.onChange).toBe(onChange);
    });

    it('builds a cluster config via embedded scope', () => {
      const onChange = vi.fn();

      const result = buildWorkloadsClusterFilterConfig({
        isWorkloadsRoute: false,
        allowEmbeddedScopeFilters: true,
        viewMode: 'vm',
        selectedCluster: 'cluster-a',
        clusterOptions: ['cluster-a'],
        onChange,
      });

      expect(result).toEqual({
        id: 'workloads-vmware-cluster-filter',
        label: 'Cluster',
        value: 'cluster-a',
        options: [
          { value: '', label: WORKLOADS_CLUSTER_ALL_OPTION_LABEL },
          { value: 'cluster-a', label: 'cluster-a' },
        ],
        onChange,
      });
      expect(result?.onChange).toBe(onChange);
    });
  });
});
