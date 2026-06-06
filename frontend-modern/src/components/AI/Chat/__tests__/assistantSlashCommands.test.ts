import { describe, expect, it } from 'vitest';
import { parseAssistantSlashCommand } from '../assistantSlashCommands';

describe('assistantSlashCommands', () => {
  it('maps OpenCode-style session commands to local Assistant actions', () => {
    expect(parseAssistantSlashCommand('/new')).toBe('new');
    expect(parseAssistantSlashCommand('/clear')).toBe('new');
    expect(parseAssistantSlashCommand('/sessions')).toBe('sessions');
    expect(parseAssistantSlashCommand('/resume')).toBe('sessions');
    expect(parseAssistantSlashCommand('/continue')).toBe('sessions');
    expect(parseAssistantSlashCommand('/models')).toBe('models');
    expect(parseAssistantSlashCommand('/model')).toBe('models');
    expect(parseAssistantSlashCommand('/copy')).toBe('copy');
    expect(parseAssistantSlashCommand('/export')).toBe('export');
    expect(parseAssistantSlashCommand('/fork')).toBe('fork');
  });

  it('normalizes casing and surrounding whitespace', () => {
    expect(parseAssistantSlashCommand('  /CoPy  ')).toBe('copy');
  });

  it('leaves normal prompts and unknown slash text untouched', () => {
    expect(parseAssistantSlashCommand('count devices')).toBeNull();
    expect(parseAssistantSlashCommand('/unknown')).toBeNull();
    expect(parseAssistantSlashCommand('/ copy')).toBeNull();
    expect(parseAssistantSlashCommand('/copy this sentence')).toBeNull();
  });
});
