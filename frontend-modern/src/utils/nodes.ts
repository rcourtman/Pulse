import type { Node } from '@/types/api';

type DisplayableNode = Pick<Node, 'name'> & Partial<Pick<Node, 'displayName' | 'instance'>>;

export function getNodeDisplayName<T extends DisplayableNode>(node: T): string {
  const display = typeof node.displayName === 'string' ? node.displayName.trim() : '';
  if (display) return display;

  const instance = typeof node.instance === 'string' ? node.instance.trim() : '';
  if (instance) return instance;

  return node.name;
}

export function hasAlternateDisplayName<T extends DisplayableNode>(node: T): boolean {
  const display = typeof node.displayName === 'string' ? node.displayName.trim() : '';
  if (!display) return false;
  return display.toLowerCase() !== node.name.trim().toLowerCase();
}
