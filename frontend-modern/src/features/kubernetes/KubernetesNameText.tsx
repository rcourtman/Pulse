import type { Component } from 'solid-js';
import { splitKubernetesNameTail } from './kubernetesPageModel';

/**
 * A Kubernetes name in a narrow cell. Generated and ordinal names end in the
 * part that tells siblings apart (checkout-api-6d8f9c7b5-x7k2p,
 * prod-euw1-k8s-03), so the head truncates and the tail stays visible:
 * checkout-api-6d…-x7k2p. The tail never outgrows the cell: when even it
 * does not fit, it truncates too once the head is gone. The full name stays on
 * the title, and the two spans read as one name to assistive technology.
 */
export const KubernetesNameText: Component<{ name: string; class?: string }> = (props) => {
  const parts = () => splitKubernetesNameTail(props.name);
  // The head and tail are flex items, so a plain copy inserts a line break
  // between them and a pasted `kubectl logs` breaks. A selection inside one
  // name copies without it; wider selections copy as the browser builds them.
  const copyWithoutBreak = (event: ClipboardEvent & { currentTarget: HTMLSpanElement }) => {
    const selection = window.getSelection();
    if (!selection || selection.rangeCount === 0 || !event.clipboardData) return;
    if (!event.currentTarget.contains(selection.getRangeAt(0).commonAncestorContainer)) return;
    event.clipboardData.setData('text/plain', selection.toString().replace(/\r?\n/g, ''));
    event.preventDefault();
  };
  return (
    <span
      class={`flex min-w-0 ${props.class ?? ''}`}
      title={props.name}
      data-kubernetes-name
      onCopy={copyWithoutBreak}
    >
      <span class="truncate">{parts().head}</span>
      <span class="max-w-full shrink-0 truncate" data-kubernetes-name-tail>
        {parts().tail}
      </span>
    </span>
  );
};

export default KubernetesNameText;
