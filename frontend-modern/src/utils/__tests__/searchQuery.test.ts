import { describe, expect, it } from 'vitest';
import {
  parseFilter,
  parseFilterStack,
  evaluateFilterStack,
} from '@/utils/searchQuery';
import type { VM, Container } from '@/types/api';

describe('parseFilter', () => {
  it('parses metric conditions with > operator', () => {
    const result = parseFilter('cpu>80');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>',
      value: 80,
    });
  });

  it('parses metric conditions with < operator', () => {
    const result = parseFilter('memory<50');
    expect(result).toEqual({
      type: 'metric',
      field: 'memory',
      operator: '<',
      value: 50,
    });
  });

  it('parses metric conditions with >= operator', () => {
    const result = parseFilter('cpu>=50');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>=',
      value: 50,
    });
  });

  it('parses metric conditions with <= operator', () => {
    const result = parseFilter('disk<=90');
    expect(result).toEqual({
      type: 'metric',
      field: 'disk',
      operator: '<=',
      value: 90,
    });
  });

  it('parses metric conditions with = operator', () => {
    const result = parseFilter('cpu=100');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '=',
      value: 100,
    });
  });

  it('parses metric conditions with == operator', () => {
    const result = parseFilter('cpu==50');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '==',
      value: 50,
    });
  });

  it('parses metric conditions with decimal values', () => {
    const result = parseFilter('cpu>50.5');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>',
      value: 50.5,
    });
  });

  it('parses text conditions with colon', () => {
    const result = parseFilter('name:prod');
    expect(result).toEqual({
      type: 'text',
      field: 'name',
      value: 'prod',
    });
  });

  it('parses text conditions with tags field', () => {
    const result = parseFilter('tags:production');
    expect(result).toEqual({
      type: 'text',
      field: 'tags',
      value: 'production',
    });
  });

  it('parses text conditions with type field', () => {
    const result = parseFilter('type:VM');
    expect(result).toEqual({
      type: 'text',
      field: 'type',
      value: 'VM',
    });
  });

  it('treats unknown patterns as raw text', () => {
    const result = parseFilter('mysearch');
    expect(result).toEqual({
      type: 'raw',
      rawText: 'mysearch',
    });
  });

  it('trims whitespace from input', () => {
    const result = parseFilter('  cpu>80  ');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>',
      value: 80,
    });
  });

  it('is case-insensitive for field names', () => {
    const result = parseFilter('CPU>80');
    expect(result).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>',
      value: 80,
    });
  });
});

describe('parseFilterStack', () => {
  it('returns empty stack for empty string', () => {
    const result = parseFilterStack('');
    expect(result.filters).toEqual([]);
    expect(result.operators).toEqual([]);
  });

  it('returns empty stack for whitespace only', () => {
    const result = parseFilterStack('   ');
    expect(result.filters).toEqual([]);
    expect(result.operators).toEqual([]);
  });

  it('parses single filter without operator', () => {
    const result = parseFilterStack('cpu>80');
    expect(result.filters).toHaveLength(1);
    expect(result.filters[0]).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>',
      value: 80,
    });
    expect(result.operators).toHaveLength(0);
  });

  it('parses multiple filters with AND operator', () => {
    const result = parseFilterStack('cpu>80 AND memory<50');
    expect(result.filters).toHaveLength(2);
    expect(result.filters[0]).toEqual({
      type: 'metric',
      field: 'cpu',
      operator: '>',
      value: 80,
    });
    expect(result.filters[1]).toEqual({
      type: 'metric',
      field: 'memory',
      operator: '<',
      value: 50,
    });
    expect(result.operators).toEqual(['AND']);
  });

  it('parses multiple filters with OR operator', () => {
    const result = parseFilterStack('name:prod OR name:staging');
    expect(result.filters).toHaveLength(2);
    expect(result.operators).toEqual(['OR']);
  });

  it('handles mixed case operators', () => {
    const result = parseFilterStack('cpu>80 and memory<50');
    expect(result.operators).toEqual(['AND']);
  });

  it('parses complex filter stack', () => {
    const result = parseFilterStack('cpu>80 AND name:prod OR tags:production');
    expect(result.filters).toHaveLength(3);
    expect(result.operators).toEqual(['AND', 'OR']);
  });
});

describe('evaluateFilterStack', () => {
  const createVM = (overrides: Partial<VM> = {}): VM => ({
    id: 'vm-1',
    vmid: 100,
    name: 'test-vm',
    node: 'node1',
    instance: 'qemu',
    status: 'running',
    type: 'qemu',
    cpu: 0.5,
    cpus: 2,
    memory: { usage: 1024, total: 2048 },
    disk: { usage: 50, total: 100 },
    networkIn: 100,
    networkOut: 200,
    diskRead: 300,
    diskWrite: 400,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: ['production', 'web'],
    lock: '',
    lastSeen: '',
    ...overrides,
  });

  const createContainer = (overrides: Partial<Container> = {}): Container => ({
    id: 'ct-1',
    vmid: 200,
    name: 'test-container',
    node: 'node1',
    instance: 'lxc',
    status: 'running',
    type: 'lxc',
    cpu: 0.3,
    cpus: 1,
    memory: { usage: 512, total: 1024 },
    disk: { usage: 30, total: 50 },
    networkIn: 50,
    networkOut: 100,
    diskRead: 150,
    diskWrite: 200,
    uptime: 7200,
    template: false,
    lastBackup: 0,
    tags: ['development', 'api'],
    lock: '',
    lastSeen: '',
    ...overrides,
  });

  it('returns true for empty filter stack', () => {
    const vm = createVM();
    const result = evaluateFilterStack(vm, { filters: [], operators: [] });
    expect(result).toBe(true);
  });

  describe('metric conditions', () => {
    it('evaluates cpu > threshold', () => {
      const vm = createVM({ cpu: 0.8 }); // 80%
      const stack = parseFilterStack('cpu>50');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates cpu < threshold (false)', () => {
      const vm = createVM({ cpu: 0.3 }); // 30%
      const stack = parseFilterStack('cpu>50');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(false);
    });

    it('evaluates memory usage threshold (absolute value)', () => {
      const vm = createVM({ memory: { usage: 1800, total: 2000 } });
      const stack = parseFilterStack('memory<2000');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true); // 1800 < 2000
    });

    it('evaluates disk usage threshold', () => {
      const vm = createVM({ disk: { usage: 80, total: 100 } });
      const stack = parseFilterStack('disk<70');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(false); // 80% > 70%
    });

    it('evaluates uptime threshold', () => {
      const vm = createVM({ status: 'running', uptime: 10000 });
      const stack = parseFilterStack('uptime>3600');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('returns 0 for uptime when not running', () => {
      const vm = createVM({ status: 'stopped', uptime: 0 });
      const stack = parseFilterStack('uptime>3600');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(false);
    });
  });

  describe('text conditions', () => {
    it('evaluates name field', () => {
      const vm = createVM({ name: 'production-web-01' });
      const stack = parseFilterStack('name:web');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates node field', () => {
      const vm = createVM({ node: 'cluster-node-1' });
      const stack = parseFilterStack('node:cluster');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates vmid field', () => {
      const vm = createVM({ vmid: 100 });
      const stack = parseFilterStack('vmid:100');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates tags field with array', () => {
      const vm = createVM({ tags: ['production', 'web', 'api'] });
      const stack = parseFilterStack('tags:web');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates tags field with string', () => {
      const vm = createVM({ tags: 'production,web,api' });
      const stack = parseFilterStack('tags:api');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates comma-separated tag search (OR logic)', () => {
      const vm = createVM({ tags: ['production', 'web'] });
      const stack = parseFilterStack('tags:staging,development');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(false);
    });
  });

  describe('raw text conditions', () => {
    it('matches raw text in name', () => {
      const vm = createVM({ name: 'my-production-vm' });
      const stack = parseFilterStack('production');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('matches raw text in status', () => {
      const vm = createVM({ status: 'running' });
      const stack = parseFilterStack('running');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('matches raw text in tags', () => {
      const vm = createVM({ tags: ['production', 'web'] });
      const stack = parseFilterStack('production');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });
  });

  describe('logical operators', () => {
    it('evaluates AND operator', () => {
      const vm = createVM({ cpu: 0.8, name: 'prod-vm' });
      const stack = parseFilterStack('cpu>50 AND name:prod');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates AND with one false condition', () => {
      const vm = createVM({ cpu: 0.3, name: 'prod-vm' });
      const stack = parseFilterStack('cpu>50 AND name:prod');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(false);
    });

    it('evaluates OR operator', () => {
      const vm = createVM({ cpu: 0.3, name: 'prod-vm' });
      const stack = parseFilterStack('cpu>50 OR name:prod');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(true);
    });

    it('evaluates OR with both false', () => {
      const vm = createVM({ cpu: 0.3, name: 'dev-vm' });
      const stack = parseFilterStack('cpu>50 OR name:prod');
      const result = evaluateFilterStack(vm, stack);
      expect(result).toBe(false);
    });
  });
});
