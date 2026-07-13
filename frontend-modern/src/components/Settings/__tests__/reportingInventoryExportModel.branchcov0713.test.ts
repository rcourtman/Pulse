/**
 * Branch-coverage tests for the still-uncovered branches of
 * `parseVMInventoryExportDefinition` in reportingInventoryExportModel.
 *
 * The sibling `reportingInventoryExportModel.test.ts` already exercises the
 * happy-path parse and the empty-columns (`columns.length === 0`) arm. This
 * file targets every other guard: the top-level non-object rejection, each
 * field-level typeof / literal / Array.isArray arm in the big validation OR
 * chain, and each per-column guard inside the `.map` (null / primitive /
 * bad key / bad label / bad description), plus a multi-column round-trip to
 * drive the map's iteration arm with more than one element.
 *
 * Import path mirrors the sibling test (relative `../reportingInventoryExportModel`).
 */
import { describe, expect, it } from 'vitest';
import { parseVMInventoryExportDefinition } from '../reportingInventoryExportModel';

const INVALID = 'Invalid VM inventory export definition payload';
const expectInvalid = (input: unknown) =>
  expect(() => parseVMInventoryExportDefinition(input)).toThrow(INVALID);

// ---- Fixtures ---------------------------------------------------------------
// Plain (non-`as const`) builders so each test can spread + override a single
// field to drive exactly one branch under test. The parser's input is typed
// `unknown`, so malformed overrides need no cast.

const baseColumn = () => ({
  key: 'pool',
  label: 'Pool',
  description: 'Canonical Proxmox pool membership.',
});

const baseDefinition = () => ({
  id: 'vm_inventory',
  title: 'VM Inventory Export',
  description: 'Current-state VM inventory',
  format: 'csv',
  exportEndpoint: '/api/admin/reports/inventory/vms/export',
  filenamePrefix: 'vm-inventory',
  filenameDateStyle: 'utc_yyyymmdd',
  columns: [baseColumn()],
});

// ---- Top-level non-object guard --------------------------------------------

describe('parseVMInventoryExportDefinition — top-level input guard', () => {
  it('throws on a falsy input (null / undefined / empty-string / 0 / false)', () => {
    expectInvalid(null);
    expectInvalid(undefined);
    expectInvalid('');
    expectInvalid(0);
    expectInvalid(false);
  });

  it('throws on a truthy non-object primitive (string / number / boolean)', () => {
    expectInvalid('not-an-object');
    expectInvalid(42);
    expectInvalid(true);
  });
});

// ---- Field-level validation OR chain ---------------------------------------

describe('parseVMInventoryExportDefinition — field validation guards', () => {
  it('throws when id is not a string', () => {
    expectInvalid({ ...baseDefinition(), id: 123 });
  });

  it('throws when title is not a string', () => {
    expectInvalid({ ...baseDefinition(), title: null });
  });

  it('throws when description is not a string', () => {
    expectInvalid({ ...baseDefinition(), description: 7 });
  });

  it('throws when format is not exactly "csv" (wrong string and non-string arms)', () => {
    expectInvalid({ ...baseDefinition(), format: 'xlsx' });
    expectInvalid({ ...baseDefinition(), format: undefined });
    expectInvalid({ ...baseDefinition(), format: 1 });
  });

  it('throws when exportEndpoint is not a string', () => {
    expectInvalid({ ...baseDefinition(), exportEndpoint: null });
  });

  it('throws when filenamePrefix is not a string', () => {
    expectInvalid({ ...baseDefinition(), filenamePrefix: 5 });
  });

  it('throws when filenameDateStyle is not exactly "utc_yyyymmdd"', () => {
    expectInvalid({ ...baseDefinition(), filenameDateStyle: 'local_yyyy_mm_dd' });
    expectInvalid({ ...baseDefinition(), filenameDateStyle: undefined });
  });

  it('throws when columns is not an array (null and non-array arms)', () => {
    expectInvalid({ ...baseDefinition(), columns: null });
    expectInvalid({ ...baseDefinition(), columns: 'not-an-array' });
    expectInvalid({ ...baseDefinition(), columns: { length: 1 } });
  });
});

// ---- Per-column guards inside .map -----------------------------------------

describe('parseVMInventoryExportDefinition — per-column guards', () => {
  it('throws when a column entry is null (falsy arm)', () => {
    expectInvalid({ ...baseDefinition(), columns: [null] });
  });

  it('throws when a column entry is a primitive (typeof !== "object" arm)', () => {
    expectInvalid({ ...baseDefinition(), columns: ['not-an-object'] });
    expectInvalid({ ...baseDefinition(), columns: [42] });
  });

  it('throws when a column key is not a string', () => {
    expectInvalid({
      ...baseDefinition(),
      columns: [{ ...baseColumn(), key: 5 }],
    });
  });

  it('throws when a column label is not a string', () => {
    expectInvalid({
      ...baseDefinition(),
      columns: [{ ...baseColumn(), label: null }],
    });
  });

  it('throws when a column description is not a string', () => {
    expectInvalid({
      ...baseDefinition(),
      columns: [{ ...baseColumn(), description: true }],
    });
  });

  it('rejects the payload on the first bad column even when later columns are valid', () => {
    expectInvalid({
      ...baseDefinition(),
      columns: [null, baseColumn()],
    });
  });
});

// ---- Happy-path round-trip (multi-column map iteration) ---------------------

describe('parseVMInventoryExportDefinition — happy path', () => {
  it('round-trips a multi-column payload unchanged and hardcodes format "csv"', () => {
    const input = {
      ...baseDefinition(),
      columns: [
        { key: 'pool', label: 'Pool', description: 'Canonical Proxmox pool membership.' },
        { key: 'node', label: 'Node', description: 'Hosting Proxmox node.' },
        { key: 'status', label: 'Status', description: 'Current power state.' },
      ],
    };
    const parsed = parseVMInventoryExportDefinition(input);
    expect(parsed).toStrictEqual({
      id: 'vm_inventory',
      title: 'VM Inventory Export',
      description: 'Current-state VM inventory',
      format: 'csv',
      exportEndpoint: '/api/admin/reports/inventory/vms/export',
      filenamePrefix: 'vm-inventory',
      filenameDateStyle: 'utc_yyyymmdd',
      columns: [
        { key: 'pool', label: 'Pool', description: 'Canonical Proxmox pool membership.' },
        { key: 'node', label: 'Node', description: 'Hosting Proxmox node.' },
        { key: 'status', label: 'Status', description: 'Current power state.' },
      ],
    });
  });

  it('preserves empty-string text fields verbatim (string-typed but empty)', () => {
    const parsed = parseVMInventoryExportDefinition({
      ...baseDefinition(),
      id: '',
      title: '',
      description: '',
      exportEndpoint: '',
      filenamePrefix: '',
      columns: [{ key: '', label: '', description: '' }],
    });
    expect(parsed.id).toBe('');
    expect(parsed.title).toBe('');
    expect(parsed.description).toBe('');
    expect(parsed.exportEndpoint).toBe('');
    expect(parsed.filenamePrefix).toBe('');
    expect(parsed.columns[0]).toEqual({ key: '', label: '', description: '' });
  });
});
