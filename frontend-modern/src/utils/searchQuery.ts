import type { VM, Container } from '@/types/api';

export type ComparisonOperator = '>' | '<' | '>=' | '<=' | '=' | '==';
export type LogicalOperator = 'AND' | 'OR';

export interface MetricCondition {
  field: 'cpu' | 'memory' | 'disk' | 'diskRead' | 'diskWrite' | 'networkIn' | 'networkOut';
  operator: ComparisonOperator;
  value: number;
}

export interface TextCondition {
  field: 'name' | 'node' | 'vmid';
  value: string;
}

export type Condition = MetricCondition | TextCondition;

export interface ParsedQuery {
  conditions: Condition[];
  logicalOperator: LogicalOperator;
  rawText?: string; // Fallback for simple text search
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
  
  // Try to parse metric condition (e.g., "cpu>80")
  const metricMatch = term.match(/^(cpu|memory|disk|diskRead|diskWrite|networkIn|networkOut)\s*(>|<|>=|<=|=|==)\s*(\d+(?:\.\d+)?)$/i);
  if (metricMatch) {
    const [, field, operator, value] = metricMatch;
    return {
      type: 'metric',
      field: field.toLowerCase(),
      operator: operator as ComparisonOperator,
      value: parseFloat(value)
    };
  }

  // Try to parse text condition (e.g., "name:prod")
  const textMatch = term.match(/^(name|node|vmid)\s*:\s*(.+)$/i);
  if (textMatch) {
    const [, field, value] = textMatch;
    return {
      type: 'text',
      field: field.toLowerCase(),
      value: value.trim()
    };
  }

  // Otherwise treat as raw text search
  return {
    type: 'raw',
    rawText: term
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

function parseCondition(conditionStr: string): Condition | null {
  // Try to parse metric condition (e.g., "cpu>80")
  const metricMatch = conditionStr.match(/^(cpu|memory|disk|diskRead|diskWrite|networkIn|networkOut)\s*(>|<|>=|<=|=|==)\s*(\d+(?:\.\d+)?)$/i);
  if (metricMatch) {
    const [, field, operator, value] = metricMatch;
    return {
      field: field.toLowerCase() as MetricCondition['field'],
      operator: operator as ComparisonOperator,
      value: parseFloat(value)
    } as MetricCondition;
  }

  // Try to parse text condition (e.g., "name:prod")
  const textMatch = conditionStr.match(/^(name|node|vmid)\s*:\s*(.+)$/i);
  if (textMatch) {
    const [, field, value] = textMatch;
    return {
      field: field.toLowerCase() as 'name' | 'node' | 'vmid',
      value: value.trim()
    } as TextCondition;
  }

  return null;
}

export function parseSearchQuery(query: string): ParsedQuery {
  query = query.trim();
  
  // Check for logical operators
  const hasAnd = /\bAND\b/i.test(query);
  const hasOr = /\bOR\b/i.test(query);
  
  // If no operators or invalid query, treat as simple text search
  if (!hasAnd && !hasOr && !query.match(/[><=:]/)) {
    return {
      conditions: [],
      logicalOperator: 'AND',
      rawText: query
    };
  }

  // Split by logical operator
  const logicalOperator: LogicalOperator = hasAnd ? 'AND' : 'OR';
  const parts = query.split(hasAnd ? /\bAND\b/i : /\bOR\b/i);
  
  const conditions: Condition[] = [];
  
  for (const part of parts) {
    const condition = parseCondition(part.trim());
    if (condition) {
      conditions.push(condition);
    }
  }

  // If no valid conditions parsed, fall back to text search
  if (conditions.length === 0) {
    return {
      conditions: [],
      logicalOperator: 'AND',
      rawText: query
    };
  }

  return {
    conditions,
    logicalOperator
  };
}

function evaluateMetricCondition(guest: VM | Container, condition: MetricCondition): boolean {
  let value: number;
  
  switch (condition.field) {
    case 'cpu':
      // CPU is stored as decimal (0-1), convert to percentage
      value = (guest.cpu || 0) * 100;
      break;
    case 'memory':
      value = guest.memory ? guest.memory.usage : 0;
      break;
    case 'disk':
      value = guest.disk ? guest.disk.usage : 0;
      break;
    default:
      return false;
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

function evaluateTextCondition(guest: VM | Container, condition: TextCondition): boolean {
  const searchValue = condition.value.toLowerCase();
  
  switch (condition.field) {
    case 'name':
      return guest.name.toLowerCase().includes(searchValue);
    case 'node':
      return guest.node.toLowerCase().includes(searchValue);
    case 'vmid':
      return guest.vmid.toString().includes(searchValue);
    default:
      return false;
  }
}

export function evaluateSearchQuery(guest: VM | Container, query: ParsedQuery): boolean {
  // If it's a simple text search
  if (query.rawText) {
    const searchTerms = query.rawText.toLowerCase().split(',').map(term => term.trim()).filter(term => term.length > 0);
    return searchTerms.some(term => 
      guest.name.toLowerCase().includes(term) ||
      guest.vmid.toString().includes(term) ||
      guest.node.toLowerCase().includes(term) ||
      guest.status.toLowerCase().includes(term)
    );
  }

  // If no conditions, match all
  if (query.conditions.length === 0) {
    return true;
  }

  // Evaluate conditions
  const results = query.conditions.map(condition => {
    if ('operator' in condition) {
      return evaluateMetricCondition(guest, condition);
    } else {
      return evaluateTextCondition(guest, condition);
    }
  });

  // Apply logical operator
  if (query.logicalOperator === 'AND') {
    return results.every(result => result);
  } else {
    return results.some(result => result);
  }
}

// Evaluate a filter stack against a guest
export function evaluateFilterStack(guest: VM | Container, stack: FilterStack): boolean {
  if (stack.filters.length === 0) {
    return true;
  }

  const results = stack.filters.map(filter => {
    if (filter.type === 'metric' && filter.field && filter.operator && filter.value !== undefined) {
      const condition: MetricCondition = {
        field: filter.field as MetricCondition['field'],
        operator: filter.operator,
        value: filter.value as number
      };
      return evaluateMetricCondition(guest, condition);
    } else if (filter.type === 'text' && filter.field && filter.value) {
      const condition: TextCondition = {
        field: filter.field as TextCondition['field'],
        value: filter.value as string
      };
      return evaluateTextCondition(guest, condition);
    } else if (filter.type === 'raw' && filter.rawText) {
      const term = filter.rawText.toLowerCase();
      return guest.name.toLowerCase().includes(term) ||
             guest.vmid.toString().includes(term) ||
             guest.node.toLowerCase().includes(term) ||
             guest.status.toLowerCase().includes(term);
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