import type { VM, Container } from '@/types/api';

export type ComparisonOperator = '>' | '<' | '>=' | '<=' | '=' | '==';
export type LogicalOperator = 'AND' | 'OR';

export interface MetricCondition {
  field:
    | 'cpu'
    | 'memory'
    | 'disk'
    | 'diskRead'
    | 'diskWrite'
    | 'networkIn'
    | 'networkOut'
    | 'uptime';
  operator: ComparisonOperator;
  value: number;
}

export interface TextCondition {
  field: 'name' | 'node' | 'vmid' | 'tags';
  value: string;
}

// New interfaces for stackable filters
export interface ParsedFilter {
  type: 'metric' | 'text' | 'raw';
  rawText?: string;
  field?: string;
  operator?: ComparisonOperator;
  value?: string | number;
}

export interface FilterStack {
  filters: ParsedFilter[];
  operators: LogicalOperator[]; // Operators between filters (length = filters.length - 1)
  logicalOperator?: LogicalOperator; // Deprecated, kept for compatibility
}

// Parse a single filter from a search term
export function parseFilter(term: string): ParsedFilter {
  term = term.trim();

  // Try to parse metric condition (e.g., "cpu>80", "size>1000000000")
  const metricMatch = term.match(/^(\w+)\s*(>|<|>=|<=|=|==)\s*(\d+(?:\.\d+)?)$/i);
  if (metricMatch) {
    const [, field, operator, value] = metricMatch;
    const parsedValue = parseFloat(value);
    if (isNaN(parsedValue)) {
      return {
        type: 'raw',
        rawText: term,
      };
    }

    return {
      type: 'metric',
      field: field.toLowerCase(),
      operator: operator as ComparisonOperator,
      value: parsedValue,
    };
  }

  // Try to parse text condition (e.g., "name:prod", "tags:production", "storage:local", "type:VM")
  const textMatch = term.match(/^(\w+)\s*:\s*(.+)$/i);
  if (textMatch) {
    const [, field, value] = textMatch;
    return {
      type: 'text',
      field: field.toLowerCase(),
      value: value.trim(),
    };
  }

  // Otherwise treat as raw text search
  return {
    type: 'raw',
    rawText: term,
  };
}

// Parse multiple filters from a search string
export function parseFilterStack(searchString: string): FilterStack {
  const trimmed = searchString.trim();
  if (!trimmed) {
    return { filters: [], operators: [] };
  }

  // Split by AND/OR operators while preserving them
  const regex = /\s+(AND|OR)\s+/gi;
  const parts = trimmed.split(regex);
  const filters: ParsedFilter[] = [];
  const operators: LogicalOperator[] = [];

  // Process the parts
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i].trim();
    if (!part) continue;

    if (i % 2 === 0) {
      // Even indices are filter expressions
      const filter = parseFilter(part);
      filters.push(filter);
    } else {
      // Odd indices are operators
      operators.push(part.toUpperCase() as LogicalOperator);
    }
  }

  // For backward compatibility, include logicalOperator as the first operator
  const logicalOperator = operators.length > 0 ? operators[0] : 'AND';

  return { filters, operators, logicalOperator };
}

type FilterableItem = VM | Container;

function evaluateMetricCondition(guest: FilterableItem, condition: MetricCondition): boolean {
  let value: number;

  switch (condition.field) {
    case 'cpu':
      // CPU is stored as decimal (0-1), convert to percentage
      value = ('cpu' in guest ? guest.cpu || 0 : 0) * 100;
      break;
    case 'memory':
      value = 'memory' in guest && guest.memory ? guest.memory.usage : 0;
      break;
    case 'disk':
      value = 'disk' in guest && guest.disk ? guest.disk.usage : 0;
      break;
    case 'uptime':
      // Uptime in seconds (only for running VMs/containers)
      value =
        'status' in guest && guest.status === 'running' && 'uptime' in guest
          ? guest.uptime || 0
          : 0;
      break;
    default:
      // For backup-specific numeric fields like 'size'
      if (typeof guest === 'object' && guest !== null && condition.field in guest) {
        const fieldValue = (guest as unknown as Record<string, unknown>)[condition.field];
        if (fieldValue !== undefined) {
          value = Number(fieldValue) || 0;
        } else {
          return false;
        }
      } else {
        return false;
      }
  }

  switch (condition.operator) {
    case '>':
      return value > condition.value;
    case '<':
      return value < condition.value;
    case '>=':
      return value >= condition.value;
    case '<=':
      return value <= condition.value;
    case '=':
    case '==':
      return Math.abs(value - condition.value) < 0.01;
    default:
      return false;
  }
}

function evaluateTextCondition(guest: FilterableItem, condition: TextCondition): boolean {
  const searchValue = condition.value.toLowerCase();

  switch (condition.field) {
    case 'name':
      return 'name' in guest && guest.name ? guest.name.toLowerCase().includes(searchValue) : false;
    case 'node':
      return 'node' in guest && guest.node ? guest.node.toLowerCase().includes(searchValue) : false;
    case 'vmid':
      return 'vmid' in guest && guest.vmid ? guest.vmid.toString().includes(searchValue) : false;
    case 'tags':
      // Check if guest has any tags that match the search value
      if (!('tags' in guest) || !guest.tags) return false;
      const tagsArray = Array.isArray(guest.tags)
        ? guest.tags.filter((tag): tag is string => typeof tag === 'string')
        : typeof guest.tags === 'string'
          ? guest.tags
              .split(',')
              .map((tag) => tag.trim())
              .filter((tag) => tag.length > 0)
          : [];
      if (tagsArray.length === 0) return false;
      // Support comma-separated tag searches (OR logic)
      const searchTags = searchValue
        .split(',')
        .map((t) => t.trim())
        .filter((t) => t.length > 0);
      return searchTags.some((searchTag) =>
        tagsArray.some((tag) => tag.toLowerCase().includes(searchTag.toLowerCase())),
      );
    default:
      // For backup-specific fields
      if (typeof guest === 'object' && guest !== null && condition.field in guest) {
        const fieldValue = (guest as unknown as Record<string, unknown>)[condition.field];
        if (fieldValue) {
          if (typeof fieldValue === 'string') {
            return fieldValue.toLowerCase().includes(searchValue);
          } else if (typeof fieldValue === 'number') {
            return fieldValue.toString().includes(searchValue);
          } else if (typeof fieldValue === 'boolean') {
            return fieldValue.toString() === searchValue;
          }
        }
      }
      return false;
  }
}

// Evaluate a filter stack against a guest or backup item
export function evaluateFilterStack(guest: FilterableItem, stack: FilterStack): boolean {
  if (stack.filters.length === 0) {
    return true;
  }

  const results = stack.filters.map((filter) => {
    if (filter.type === 'metric' && filter.field && filter.operator && filter.value !== undefined) {
      const condition: MetricCondition = {
        field: filter.field as MetricCondition['field'],
        operator: filter.operator,
        value: filter.value as number,
      };
      return evaluateMetricCondition(guest, condition);
    } else if (filter.type === 'text' && filter.field && filter.value) {
      // Handle tags field specifically since it might not be in the type union
      if (filter.field === 'tags') {
        const condition: TextCondition = {
          field: 'tags',
          value: filter.value as string,
        };
        return evaluateTextCondition(guest, condition);
      }
      const condition: TextCondition = {
        field: filter.field as TextCondition['field'],
        value: filter.value as string,
      };
      return evaluateTextCondition(guest, condition);
    } else if (filter.type === 'raw' && filter.rawText) {
      const term = filter.rawText.toLowerCase();
      // Check name, vmid, node, status, and tags for raw text matches
      const nameMatch =
        'name' in guest &&
        typeof guest.name === 'string' &&
        guest.name.toLowerCase().includes(term);
      const vmidMatch = 'vmid' in guest && !!guest.vmid && guest.vmid.toString().includes(term);
      const nodeMatch =
        'node' in guest &&
        typeof guest.node === 'string' &&
        guest.node.toLowerCase().includes(term);
      const statusMatch =
        'status' in guest &&
        typeof guest.status === 'string' &&
        guest.status.toLowerCase().includes(term);

      // Also check if any tags contain the search term
      const tagMatch =
        'tags' in guest && Array.isArray(guest.tags)
          ? guest.tags
              .filter((tag): tag is string => typeof tag === 'string')
              .some((tag) => tag.toLowerCase().includes(term))
          : false;

      return nameMatch || vmidMatch || nodeMatch || statusMatch || tagMatch;
    }
    return true;
  });

  // If only one filter, return its result
  if (results.length === 1) {
    return results[0];
  }

  // Apply operators between filters
  let result = results[0];
  for (let i = 0; i < stack.operators.length && i < results.length - 1; i++) {
    const operator = stack.operators[i];
    const nextResult = results[i + 1];

    if (operator === 'AND') {
      result = result && nextResult;
    } else {
      result = result || nextResult;
    }
  }

  return result;
}
