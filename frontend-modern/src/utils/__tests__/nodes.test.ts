import { describe, expect, it } from 'vitest';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';

describe('getNodeDisplayName', () => {
  it('returns displayName when present and non-empty', () => {
    const node = { name: 'node1', displayName: 'My Node' };
    expect(getNodeDisplayName(node)).toBe('My Node');
  });

  it('returns displayName with whitespace trimmed', () => {
    const node = { name: 'node1', displayName: '  My Node  ' };
    expect(getNodeDisplayName(node)).toBe('My Node');
  });

  it('returns name when displayName is empty string', () => {
    const node = { name: 'node1', displayName: '' };
    expect(getNodeDisplayName(node)).toBe('node1');
  });

  it('returns name when displayName is only whitespace', () => {
    const node = { name: 'node1', displayName: '   ' };
    expect(getNodeDisplayName(node)).toBe('node1');
  });

  it('returns name when displayName is undefined', () => {
    const node = { name: 'node1', displayName: undefined } as {
      name: string;
      displayName?: string;
    };
    expect(getNodeDisplayName(node)).toBe('node1');
  });

  it('returns host when name and displayName are empty', () => {
    const node = { name: '', host: '192.168.1.1' };
    expect(getNodeDisplayName(node)).toBe('192.168.1.1');
  });

  it('strips protocol from host', () => {
    const node = { name: '', host: 'https://server1.example.com' };
    expect(getNodeDisplayName(node)).toBe('server1.example.com');
  });

  it('strips port from host', () => {
    const node = { name: '', host: 'server1.example.com:8080' };
    expect(getNodeDisplayName(node)).toBe('server1.example.com');
  });

  it('strips protocol and port from host', () => {
    const node = { name: '', host: 'http://server1.example.com:443/path' };
    expect(getNodeDisplayName(node)).toBe('server1.example.com');
  });

  it('returns instance when name, displayName, and host are empty', () => {
    const node = { name: '', host: '', instance: 'qemu' };
    expect(getNodeDisplayName(node)).toBe('qemu');
  });

  it('returns instance with whitespace trimmed', () => {
    const node = { name: '', host: '', instance: '  lxc  ' };
    expect(getNodeDisplayName(node)).toBe('lxc');
  });

  it('returns empty string when all fields are empty', () => {
    const node = { name: '', host: '', instance: '' };
    expect(getNodeDisplayName(node)).toBe('');
  });

  it('returns empty string when all fields are undefined', () => {
    const node = { name: undefined, host: undefined, instance: undefined } as any;
    expect(getNodeDisplayName(node)).toBe('');
  });
});

describe('hasAlternateDisplayName', () => {
  it('returns false when displayName is empty', () => {
    const node = { name: 'node1', displayName: '' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when displayName is undefined', () => {
    const node = { name: 'node1', displayName: undefined } as {
      name: string;
      displayName?: string;
    };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when name is empty', () => {
    const node = { name: '', displayName: 'Display' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when name and displayName are identical', () => {
    const node = { name: 'node1', displayName: 'node1' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when displayName equals name after case normalization', () => {
    const node = { name: 'Node1', displayName: 'node1' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when displayName equals name after sanitization', () => {
    const node = { name: 'node_1', displayName: 'node-1' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when displayName embeds the node name', () => {
    const node = { name: 'node1', displayName: 'Friendly (node1)' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns false when first label of displayName matches name', () => {
    const node = { name: 'server', displayName: 'server.example.com' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });

  it('returns true when displayName is meaningfully different from name', () => {
    const node = { name: 'node1', displayName: 'Production Server' };
    expect(hasAlternateDisplayName(node)).toBe(true);
  });

  it('returns true when displayName differs after sanitization', () => {
    const node = { name: 'prod-server-01', displayName: 'prod-server-02' };
    expect(hasAlternateDisplayName(node)).toBe(true);
  });

  it('handles displayName with only whitespace differences', () => {
    const node = { name: 'node1', displayName: '  Node1  ' };
    expect(hasAlternateDisplayName(node)).toBe(false);
  });
});
