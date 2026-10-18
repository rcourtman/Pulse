import { Show, type Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { kubernetesScopeDisplayLabel, kubernetesScopeLabel } from './kubernetesPageModel';

/**
 * A Kubernetes scope cell. With one cluster in view it shows the namespace
 * alone (the cluster name would repeat on every row), keeps the full
 * cluster/namespace scope on the title, and gives assistive technology the full
 * scope in place of the shortened text.
 */
export const KubernetesScopeText: Component<{
  resource: Resource;
  singleCluster: boolean;
  class?: string;
}> = (props) => {
  const full = () => kubernetesScopeLabel(props.resource);
  const shown = () => kubernetesScopeDisplayLabel(props.resource, props.singleCluster);
  return (
    <span class={props.class} title={full()} data-kubernetes-scope>
      <span aria-hidden={shown() !== full() ? 'true' : undefined}>{shown()}</span>
      <Show when={shown() !== full()}>
        <span class="sr-only">{full()}</span>
      </Show>
    </span>
  );
};

export default KubernetesScopeText;
