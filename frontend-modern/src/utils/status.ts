import type { Node, VM, Container } from '@/types/api';

const ONLINE_STATUS = 'online';
const RUNNING_STATUS = 'running';

export function isNodeOnline(node: Partial<Node> | undefined | null): boolean {
  if (!node) return false;
  if (node.status !== ONLINE_STATUS) return false;
  if ((node.uptime ?? 0) <= 0) return false;
  const connection = (node as Node).connectionHealth;
  if (connection === 'offline' || connection === 'error') return false;
  return true;
}

export function isGuestRunning(
  guest: Partial<VM | Container> | undefined | null,
  parentNodeOnline = true,
): boolean {
  if (!guest) return false;
  if (!parentNodeOnline) return false;
  return guest.status === RUNNING_STATUS;
}

export function shouldDisplayGuestMetrics(
  guest: Partial<VM | Container> | undefined | null,
  parentNodeOnline = true,
): boolean {
  return isGuestRunning(guest, parentNodeOnline);
}
