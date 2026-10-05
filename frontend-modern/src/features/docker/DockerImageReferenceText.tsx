import type { Component } from 'solid-js';
import { splitDockerImageReference } from './dockerImageReference';

/**
 * An image reference in a narrow cell. References that share a registry and
 * namespace (ghcr.io/pulse-demo/…, lscr.io/linuxserver/…) differ only in the
 * repository name and tag at the end, so the head truncates and the tail stays
 * visible: ghcr.io/pul…backup-coordinator:2026.04. The tail never outgrows the
 * cell: when even it does not fit, it truncates too once the head is gone. The
 * full reference stays on the title, and the two spans read as one reference
 * to assistive technology. Mirrors KubernetesNameText.
 */
export const DockerImageReferenceText: Component<{ reference: string; class?: string }> = (
  props,
) => {
  const parts = () => splitDockerImageReference(props.reference);
  // The head and tail are flex items, so a plain copy inserts a line break
  // between them and a pasted `docker pull` breaks. A selection inside one
  // reference copies without it; wider selections copy as the browser builds
  // them.
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
      title={props.reference}
      data-docker-image-reference
      onCopy={copyWithoutBreak}
    >
      <span class="truncate">{parts().head}</span>
      <span class="max-w-full shrink-0 truncate" data-docker-image-reference-tail>
        {parts().tail}
      </span>
    </span>
  );
};

export default DockerImageReferenceText;
