import { formatIdentifierLabel } from '@/utils/textPresentation';

export const getToolLabel = (name: string) => {
  if (name === 'run_command' || name === 'pulse_run_command') return 'cmd';
  if (name === 'fetch_url' || name === 'pulse_fetch_url') return 'fetch';
  if (name === 'get_infrastructure_state' || name === 'pulse_get_infrastructure_state')
    return 'infra';
  if (name === 'get_active_alerts' || name === 'pulse_get_active_alerts') return 'alerts';
  if (name === 'get_metrics_history' || name === 'pulse_get_metrics_history') return 'metrics';
  if (name === 'get_baselines' || name === 'pulse_get_baselines') return 'baselines';
  if (name === 'get_patterns' || name === 'pulse_get_patterns') return 'patterns';
  if (name === 'get_disk_health' || name === 'pulse_get_disk_health') return 'disks';
  if (name === 'get_storage' || name === 'pulse_get_storage') return 'storage';
  if (name === 'pulse_get_storage_config') return 'storage cfg';
  if (name === 'get_resource_details' || name === 'pulse_get_resource_details') return 'resource';
  if (name.includes('finding')) return 'finding';
  return formatIdentifierLabel(name, { stripPrefix: 'pulse_', maxLength: 12 });
};

const normalizedToolName = (name?: string) => (name || '').trim().replace(/^pulse_/, '');

export const pendingToolActionLabel = (name?: string) => {
  const tool = normalizedToolName(name);
  if (tool === 'run_command' || tool === 'control') return 'Writing command...';
  if (tool === 'read') return 'Preparing read...';
  if (tool === 'query') return 'Preparing query...';
  if (tool === 'fetch_url') return 'Fetching URL...';
  if (tool === 'get_infrastructure_state') return 'Reading infrastructure...';
  if (tool === 'get_active_alerts' || tool === 'alerts') return 'Reading alerts...';
  if (tool === 'get_metrics_history') return 'Reading metrics...';
  if (tool === 'get_baselines') return 'Reading baselines...';
  if (tool === 'get_patterns') return 'Reading patterns...';
  if (tool === 'get_disk_health') return 'Checking disks...';
  if (tool === 'get_storage' || tool === 'get_storage_config') return 'Reading storage...';
  if (tool === 'get_resource_details') return 'Reading resource...';
  if ((name || '').includes('finding')) return 'Preparing finding...';
  return 'Preparing tool...';
};

export const pendingToolActionState = (name?: string) => {
  const tool = normalizedToolName(name);
  if (tool === 'run_command' || tool === 'control') return 'writing';
  if (tool === 'query') return 'preparing';
  if (tool === 'fetch_url') return 'fetching';
  if (tool === 'get_disk_health') return 'checking';
  if (
    tool === 'read' ||
    tool === 'get_infrastructure_state' ||
    tool === 'get_active_alerts' ||
    tool === 'alerts' ||
    tool === 'get_metrics_history' ||
    tool === 'get_baselines' ||
    tool === 'get_patterns' ||
    tool === 'get_storage' ||
    tool === 'get_storage_config' ||
    tool === 'get_resource_details'
  ) {
    return 'reading';
  }
  return 'preparing';
};

export const isPlaceholderToolInputSummary = (summary: string) => {
  const normalized = summary.trim().toLowerCase();
  return !normalized || normalized === 'request' || normalized === 'receiving input';
};

const escapePartialJSONKey = (key: string) => key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const extractPartialJSONStringField = (rawInput: string, keys: string[]) => {
  const raw = rawInput.trim();
  if (!raw) return '';

  for (const key of keys) {
    const escapedKey = escapePartialJSONKey(key);
    const match = raw.match(new RegExp(`"${escapedKey}"\\s*:\\s*"((?:\\\\.|[^"\\\\])*)`));
    if (match?.[1]) {
      return match[1].replace(/\\"/g, '"').replace(/\\\\/g, '\\').trim();
    }
  }
  return '';
};

const stringField = (record: Record<string, unknown>, keys: string[]) => {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
    if (typeof value === 'number' && Number.isFinite(value)) {
      return String(value);
    }
  }
  return '';
};

const booleanField = (record: Record<string, unknown>, key: string) => record[key] === true;

const inlineValue = (value: string, maxLength = 24) =>
  value
    .replace(/["\r\n]+/g, ' ')
    .trim()
    .substring(0, maxLength);

const targetSuffix = (record: Record<string, unknown>) => {
  const target = stringField(record, ['target_host', 'targetHost', 'resource_id', 'resourceId']);
  if (!target) return '';
  return ` on ${formatIdentifierLabel(target, { maxLength: 18 })}`;
};

const parseFunctionStyleToolInput = (input: string) => {
  const match = /^([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([\s\S]*)\)\s*$/.exec(input.trim());
  if (!match) return null;

  const args: Record<string, unknown> = {};
  const body = match[2] || '';
  let index = 0;

  const skipWhitespace = () => {
    while (index < body.length && /\s/.test(body[index])) index += 1;
  };

  const parseQuotedValue = (quote: string) => {
    index += 1;
    let value = '';
    while (index < body.length) {
      const char = body[index];
      if (char === '\\') {
        if (index + 1 < body.length) {
          value += body[index + 1];
          index += 2;
          continue;
        }
        return null;
      }
      if (char === quote) {
        index += 1;
        return value;
      }
      value += char;
      index += 1;
    }
    return null;
  };

  while (index < body.length) {
    skipWhitespace();
    if (index >= body.length) break;

    if (body[index] === ',') {
      index += 1;
      continue;
    }

    const keyMatch = /^[a-zA-Z_][a-zA-Z0-9_]*/.exec(body.slice(index));
    if (!keyMatch) return null;
    const key = keyMatch[0];
    index += key.length;

    skipWhitespace();
    if (body[index] !== '=') return null;
    index += 1;
    skipWhitespace();

    const quote = body[index];
    let value: unknown;
    if (quote === '"' || quote === "'") {
      value = parseQuotedValue(quote);
      if (value === null) return null;
    } else {
      const start = index;
      while (index < body.length && body[index] !== ',') index += 1;
      const rawValue = body.slice(start, index).trim();
      if (!rawValue) return null;
      if (rawValue === 'true') value = true;
      else if (rawValue === 'false') value = false;
      else if (rawValue === 'null') value = null;
      else if (/^-?\d+(?:\.\d+)?$/.test(rawValue)) value = Number(rawValue);
      else value = rawValue;
    }

    args[key] = value;
    skipWhitespace();
    if (index < body.length && body[index] !== ',') return null;
  }

  return { name: match[1], args };
};

const formatPartialRawInputSummary = (rawInput: string | undefined, toolName?: string) => {
  if (!rawInput?.trim()) return '';

  const action = extractPartialJSONStringField(rawInput, ['action', 'type']).toLowerCase();
  const command = extractPartialJSONStringField(rawInput, ['command', 'cmd']);
  const path = extractPartialJSONStringField(rawInput, ['path', 'file', 'file_path', 'filePath']);
  const pattern = extractPartialJSONStringField(rawInput, [
    'pattern',
    'grep',
    'grep_pattern',
    'grepPattern',
  ]);
  const query = extractPartialJSONStringField(rawInput, [
    'query',
    'search',
    'name',
    'resource_name',
  ]);
  const target = extractPartialJSONStringField(rawInput, [
    'target_host',
    'targetHost',
    'resource_id',
    'resourceId',
  ]);
  const record: Record<string, unknown> = {
    ...(action ? { action } : {}),
    ...(command ? { command } : {}),
    ...(path ? { path } : {}),
    ...(pattern ? { pattern } : {}),
    ...(query ? { query } : {}),
    ...(target ? { target_host: target } : {}),
  };
  const tool = normalizedToolName(toolName);

  if ((tool === 'read' || tool === 'run_command' || tool === 'control') && command) {
    return `$ ${inlineValue(command, 64)}${targetSuffix(record)}`;
  }
  if (tool === 'read') {
    if (action === 'file' && path) return `read ${inlineValue(path, 36)}${targetSuffix(record)}`;
    if (action === 'tail' && path) return `tail ${inlineValue(path, 36)}${targetSuffix(record)}`;
    if (action === 'find' && pattern)
      return `find "${inlineValue(pattern, 28)}"${targetSuffix(record)}`;
    if (action === 'logs') return `read logs${targetSuffix(record)}`;
  }
  if (tool === 'query') {
    if (action === 'search' && query) return `search "${inlineValue(query, 28)}"`;
    if (action === 'list') return 'list resources';
    if (action) return formatIdentifierLabel(action, { maxLength: 28 });
  }
  if (action) return formatIdentifierLabel(action, { maxLength: 28 });
  return 'receiving input';
};

const formatCommandSummary = (record: Record<string, unknown>) => {
  const command = stringField(record, ['command', 'cmd']);
  if (!command) return '';
  return `$ ${inlineValue(command, 64)}${targetSuffix(record)}`;
};

const formatPulseReadInputSummary = (record: Record<string, unknown>) => {
  const action = stringField(record, ['action', 'type']).toLowerCase();
  const path = inlineValue(stringField(record, ['path', 'file', 'file_path', 'filePath']), 36);
  const pattern = inlineValue(
    stringField(record, ['pattern', 'grep', 'grep_pattern', 'grepPattern']),
    28,
  );
  const container = inlineValue(stringField(record, ['container', 'service', 'unit']), 24);
  const source = stringField(record, ['source']).toLowerCase();

  if (action === 'exec') {
    return formatCommandSummary(record) || `run read-only command${targetSuffix(record)}`;
  }
  if (action === 'file') {
    return path ? `read ${path}${targetSuffix(record)}` : `read file${targetSuffix(record)}`;
  }
  if (action === 'tail') {
    return path ? `tail ${path}${targetSuffix(record)}` : `tail file${targetSuffix(record)}`;
  }
  if (action === 'find') {
    if (pattern && path) return `find "${pattern}" in ${path}${targetSuffix(record)}`;
    return pattern
      ? `find "${pattern}"${targetSuffix(record)}`
      : `find files${targetSuffix(record)}`;
  }
  if (action === 'logs') {
    if (container) return `logs ${container}${targetSuffix(record)}`;
    return source
      ? `${formatIdentifierLabel(source, { maxLength: 18 })} logs${targetSuffix(record)}`
      : `read logs${targetSuffix(record)}`;
  }
  return (
    formatCommandSummary(record) || (action ? formatIdentifierLabel(action, { maxLength: 28 }) : '')
  );
};

const formatQueryInputSummary = (record: Record<string, unknown>) => {
  const action = stringField(record, ['action', 'type']).toLowerCase();
  const resourceType = stringField(record, ['resource_type', 'resourceType', 'kind', 'include']);
  const listType = stringField(record, ['type', 'resource_type', 'resourceType', 'kind']);
  const query = inlineValue(stringField(record, ['query', 'search', 'name', 'resource_name']));
  const resourceId = inlineValue(stringField(record, ['resource_id', 'resourceId', 'id', 'vmid']));
  const node = inlineValue(stringField(record, ['node', 'host']));

  switch (action) {
    case 'search':
      return query ? `search "${query}"` : 'search resources';
    case 'list':
      return listType
        ? `list ${formatIdentifierLabel(listType, { maxLength: 22 })}`
        : 'list resources';
    case 'get':
      return resourceId ? `get ${resourceId}` : 'get resource';
    case 'config':
      if (resourceId && node) return `config ${resourceId} on ${node}`;
      return resourceId ? `config ${resourceId}` : 'resource config';
    case 'topology':
      return booleanField(record, 'summary_only') ? 'topology summary' : 'topology';
    case 'health':
      return 'health summary';
    default:
      if (action) return formatIdentifierLabel(action, { maxLength: 28 });
      return resourceType
        ? `query ${formatIdentifierLabel(resourceType, { maxLength: 22 })}`
        : 'query resources';
  }
};

const formatAlertsInputSummary = (record: Record<string, unknown>) => {
  const action = stringField(record, ['action']).toLowerCase();
  const severity = stringField(record, ['severity', 'level']);
  const resource = stringField(record, ['resource', 'resource_id', 'resourceId', 'name']);

  if (action === 'list') {
    return severity
      ? `list ${formatIdentifierLabel(severity, { maxLength: 18 })} alerts`
      : 'list active alerts';
  }
  if (action === 'get') {
    return resource ? `get alert for ${inlineValue(resource, 18)}` : 'get alert';
  }
  return action ? formatIdentifierLabel(action, { maxLength: 28 }) : 'read alerts';
};

const formatStructuredInputSummary = (
  record: Record<string, unknown>,
  toolName?: string,
  rawInput?: string,
) => {
  if (Object.keys(record).length === 0) {
    return formatPartialRawInputSummary(rawInput, toolName) || 'request';
  }

  const tool = normalizedToolName(toolName);
  if (tool === 'read') {
    return formatPulseReadInputSummary(record) || 'read resource';
  }
  if (tool === 'run_command' || tool === 'control') {
    return formatCommandSummary(record) || 'run command';
  }
  if (tool === 'query') {
    return formatQueryInputSummary(record);
  }
  if (tool === 'alerts') {
    return formatAlertsInputSummary(record);
  }
  if (typeof record.action === 'string' && record.action.trim()) {
    return formatIdentifierLabel(record.action, { maxLength: 28 });
  }
  if (typeof record.command === 'string' && record.command.trim()) {
    return formatIdentifierLabel(record.command, { maxLength: 28 });
  }
  return 'request';
};

const parseStructuredInputSummary = (input: string, toolName?: string, rawInput?: string) => {
  const trimmed = input.trim();
  if (!trimmed) return null;

  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return formatStructuredInputSummary(parsed as Record<string, unknown>, toolName, rawInput);
    }
  } catch {
    // Keep trying provider-style function-call input below.
  }

  const functionCall = parseFunctionStyleToolInput(trimmed);
  if (functionCall) {
    return formatStructuredInputSummary(functionCall.args, functionCall.name, rawInput);
  }

  return null;
};

export const parseToolInputSummary = (input: string, toolName?: string, rawInput?: string) => {
  const trimmed = input.trim();
  if (!trimmed) return '';

  const directSummary = parseStructuredInputSummary(trimmed, toolName, rawInput);
  if (directSummary) {
    return directSummary;
  }

  if (rawInput && rawInput.trim() && rawInput.trim() !== trimmed) {
    const rawSummary = parseStructuredInputSummary(rawInput, toolName);
    if (rawSummary && !isPlaceholderToolInputSummary(rawSummary)) {
      return rawSummary;
    }
  }

  return formatIdentifierLabel(trimmed, { maxLength: 28 });
};

export const toolValueText = (value: unknown) => {
  if (typeof value === 'string') return value;
  if (value === null || value === undefined) return '';
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
};
