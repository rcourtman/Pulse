import { describe, expect, it } from 'vitest';
import {
  filterAssistantSlashCommands,
  getAssistantSlashCommandTokens,
  parseAssistantSlashCommand,
} from '../assistantSlashCommands';

describe('assistantSlashCommands', () => {
  it('maps OpenCode-style session commands to local Assistant actions', () => {
    expect(parseAssistantSlashCommand('/new')).toBe('new');
    expect(parseAssistantSlashCommand('/clear')).toBe('new');
    expect(parseAssistantSlashCommand('/sessions')).toBe('sessions');
    expect(parseAssistantSlashCommand('/resume')).toBe('sessions');
    expect(parseAssistantSlashCommand('/continue')).toBe('sessions');
    expect(parseAssistantSlashCommand('/models')).toBe('models');
    expect(parseAssistantSlashCommand('/model')).toBe('models');
    expect(parseAssistantSlashCommand('/mo')).toBe('models');
    expect(parseAssistantSlashCommand('/copy')).toBe('copy');
    expect(parseAssistantSlashCommand('/export')).toBe('export');
    expect(parseAssistantSlashCommand('/fork')).toBe('fork');
    expect(parseAssistantSlashCommand('/undo')).toBe('undo');
    expect(parseAssistantSlashCommand('/redo')).toBe('redo');
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

  it('filters commands by canonical name, alias, and description', () => {
    expect(filterAssistantSlashCommands('').map((command) => command.name)).toEqual([
      'new',
      'sessions',
      'models',
      'copy',
      'export',
      'fork',
      'undo',
      'redo',
    ]);
    expect(filterAssistantSlashCommands('resume').map((command) => command.name)).toEqual([
      'sessions',
    ]);
    expect(filterAssistantSlashCommands('provider').map((command) => command.name)).toEqual([
      'models',
    ]);
  });

  it('exposes canonical and alias tokens for the picker', () => {
    const sessions = filterAssistantSlashCommands('resume')[0];
    expect(getAssistantSlashCommandTokens(sessions)).toEqual(['sessions', 'resume', 'continue']);
    const models = filterAssistantSlashCommands('mo')[0];
    expect(getAssistantSlashCommandTokens(models)).toEqual(['models', 'model', 'mo']);
  });
});
