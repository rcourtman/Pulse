import type { HostNetworkInterface, Node } from '@/types/api';
import { formatBytes } from '@/utils/format';

// What a node is, in the terms its owner uses for it: the CPU they bought,
// the RAM they installed and the ports they cabled. Built only from data the
// node already reports (Proxmox CPU info, memory total, and a linked agent's
// NICs and fans), so it adds no collection.

const cleanText = (value: string | null | undefined): string => {
  const trimmed = (value || '').trim();
  return trimmed && trimmed.toLowerCase() !== 'unknown' ? trimmed : '';
};

const positive = (value: number | null | undefined): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value > 0;

/**
 * Shortens a kernel CPU model string to the name on the box:
 * "Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz" -> "Intel Xeon E5-2680 v4",
 * "AMD Ryzen 9 5950X 16-Core Processor" -> "AMD Ryzen 9 5950X".
 */
export const shortCpuModel = (model: string | null | undefined): string =>
  cleanText(model)
    .replace(/\((?:R|TM|tm|r)\)/g, '')
    .replace(/@\s*[\d.]+\s*[GM]Hz/i, '')
    .replace(/\b\d+-Core\b/gi, '')
    .replace(/\b(?:CPU|Processor)\b/gi, '')
    .replace(/\bwith Radeon.*$/i, '')
    .replace(/\s+/g, ' ')
    .trim();

// Usable memory reads a little under the installed size (firmware and kernel
// reservations), so 125.7 GiB is a 128 GB machine to its owner. Snap only when
// the reading sits just under a common installed size; anything else is shown
// as measured.
const COMMON_MEMORY_SIZES_GIB = [
  2, 4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256, 384, 512, 768, 1024, 1536, 2048, 3072,
  4096,
];

export const installedMemoryLabel = (totalBytes: number | null | undefined): string => {
  if (!positive(totalBytes)) return '';
  const gib = totalBytes / 1024 ** 3;
  const installed = COMMON_MEMORY_SIZES_GIB.find((size) => gib <= size && gib >= size * 0.92);
  if (installed) return installed >= 1024 ? `${installed / 1024} TB` : `${installed} GB`;
  return formatBytes(totalBytes, 0);
};

const VIRTUAL_INTERFACE =
  /^(?:lo|vmbr|fwbr|fwpr|fwln|veth|tap|bond|docker|br-|virbr|cni|flannel|cali|podman|tailscale|zt|wg|tun|utun|vxlan|kube)/i;

/** Physical ports only: no loopback, bridges, bonds, VLANs or guest taps. */
export const physicalNetworkInterfaces = (
  interfaces: readonly HostNetworkInterface[] | null | undefined,
): HostNetworkInterface[] =>
  (interfaces ?? []).filter((nic) => {
    const name = cleanText(nic.name);
    return Boolean(name) && !VIRTUAL_INTERFACE.test(name) && !name.includes('.');
  });

export const formatLinkSpeed = (mbps: number): string => {
  if (mbps >= 1000) {
    const gbps = mbps / 1000;
    return `${Number.isInteger(gbps) ? gbps : gbps.toFixed(1)} Gb/s`;
  }
  return `${mbps} Mb/s`;
};

const WIRELESS_INTERFACE = /^(?:wl|wlan|wifi)/i;

export interface NodeHardwarePort {
  name: string;
  wireless: boolean;
  linkUp?: boolean;
  speedLabel?: string;
}

export interface NodeHardwareModel {
  cpu?: { name: string; threads?: number; title: string };
  memory?: { label: string; title: string };
  ports: NodeHardwarePort[];
  fans?: { count: number; title: string };
}

/**
 * The hardware reported for the node drawer: CPU, installed memory, physical
 * ports and fans.
 */
export const nodeHardwareModel = (node: Node | null | undefined): NodeHardwareModel => {
  if (!node) return { ports: [] };
  const model: NodeHardwareModel = { ports: [] };

  const name = shortCpuModel(node.cpuInfo?.model);
  const sockets = node.cpuInfo?.sockets;
  const threads = node.cpuInfo?.cores;
  if (name) {
    const fullModel = cleanText(node.cpuInfo?.model);
    model.cpu = {
      name: positive(sockets) && sockets > 1 ? `${sockets} × ${name}` : name,
      threads: positive(threads) ? threads : undefined,
      title: [
        `${fullModel}${positive(sockets) ? ` (${sockets} socket${sockets > 1 ? 's' : ''})` : ''}`,
        positive(threads) ? `${threads} threads` : '',
      ]
        .filter(Boolean)
        .join(', '),
    };
  }

  const memory = installedMemoryLabel(node.memory?.total);
  if (memory) {
    model.memory = {
      label: memory,
      title: `${memory} memory (${formatBytes(node.memory.total)} usable)`,
    };
  }

  model.ports = physicalNetworkInterfaces(node.networkInterfaces).map((port) => {
    const name = cleanText(port.name);
    // A disconnected port can still carry the driver's last negotiated speed.
    const speed = port.linkUp !== false && positive(port.speedMbps) ? port.speedMbps : undefined;
    return {
      name,
      wireless: WIRELESS_INTERFACE.test(name),
      linkUp: port.linkUp,
      speedLabel: speed === undefined ? undefined : formatLinkSpeed(speed),
    };
  });

  const fans = Object.values(node.sensors?.fanRpm ?? {}).filter((rpm) => positive(rpm));
  if (fans.length > 0) {
    const low = Math.round(Math.min(...fans));
    const high = Math.round(Math.max(...fans));
    const range =
      low === high ? low.toLocaleString() : `${low.toLocaleString()}–${high.toLocaleString()}`;
    // Keep the fan inventory stable as RPM readings change.
    const count = fans.length === 1 ? '1 fan' : `${fans.length} fans`;
    model.fans = { count: fans.length, title: `${count} at ${range} RPM` };
  }

  return model;
};
