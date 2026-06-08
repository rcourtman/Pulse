// Apply already-sanitized markdown HTML to a live container by morphing the DOM
// in place instead of replacing innerHTML wholesale.
//
// Streaming markdown grows append-mostly: on each paced render tick only the
// trailing content actually changes, while every earlier paragraph, list item,
// and table row stays identical. Setting `innerHTML` rebuilds the entire prose
// subtree every tick, so the whole answer flickers and reflows as it streams.
//
// This walks old and new trees in parallel and only touches what changed:
// identical nodes are left alone (no repaint), same-tag nodes are morphed in
// place (recursing into their children, so a growing <ol>/<table> keeps its
// earlier <li>/<tr> and only rebuilds the last one), and genuinely different
// nodes are replaced. The result is that a streaming answer rebuilds the last
// block instead of the whole document.
//
// SECURITY: `html` MUST already be sanitized (renderMarkdown runs DOMPurify).
// This helper only parses that trusted-by-construction HTML into an inert
// <template> and reconciles the resulting nodes into the DOM — it performs no
// sanitization of its own and must not be fed raw model/user HTML.
export function morphMarkdownInto(container: HTMLElement, html: string): void {
  const template = document.createElement('template');
  template.innerHTML = html;
  morphChildren(container, Array.from(template.content.childNodes));
}

// Reconcile `parent`'s children against `newNodes` (a snapshot taken before any
// mutation, since moving a node into the live tree removes it from the template).
function morphChildren(parent: Node, newNodes: Node[]): void {
  for (let i = 0; i < newNodes.length; i++) {
    const newNode = newNodes[i];
    const oldNode = parent.childNodes[i];

    if (!oldNode) {
      parent.appendChild(newNode);
      continue;
    }
    if (oldNode.isEqualNode(newNode)) {
      continue; // structurally identical — keep the existing node untouched
    }
    if (canMorphInPlace(oldNode, newNode)) {
      morphNode(oldNode, newNode);
    } else {
      parent.replaceChild(newNode, oldNode);
    }
  }

  // Drop any trailing old nodes the new render no longer has.
  while (parent.childNodes.length > newNodes.length) {
    parent.removeChild(parent.childNodes[parent.childNodes.length - 1]);
  }
}

// A node can be updated in place (rather than wholesale replaced) when it is the
// same kind of node — same element tag, or both character data (text/comment).
function canMorphInPlace(oldNode: Node, newNode: Node): boolean {
  if (oldNode.nodeType !== newNode.nodeType) return false;
  if (oldNode.nodeType === Node.ELEMENT_NODE) {
    return (oldNode as Element).tagName === (newNode as Element).tagName;
  }
  return oldNode.nodeType === Node.TEXT_NODE || oldNode.nodeType === Node.COMMENT_NODE;
}

function morphNode(oldNode: Node, newNode: Node): void {
  if (oldNode.nodeType !== Node.ELEMENT_NODE) {
    if (oldNode.nodeValue !== newNode.nodeValue) {
      oldNode.nodeValue = newNode.nodeValue;
    }
    return;
  }
  syncAttributes(oldNode as Element, newNode as Element);
  morphChildren(oldNode, Array.from(newNode.childNodes));
}

function syncAttributes(oldEl: Element, newEl: Element): void {
  for (const attr of Array.from(oldEl.attributes)) {
    if (!newEl.hasAttribute(attr.name)) oldEl.removeAttribute(attr.name);
  }
  for (const attr of Array.from(newEl.attributes)) {
    if (oldEl.getAttribute(attr.name) !== attr.value) {
      oldEl.setAttribute(attr.name, attr.value);
    }
  }
}
