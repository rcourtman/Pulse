import { describe, expect, it } from 'vitest';
import {
  filterAssistantSlashCommands,
  getAssistantSlashCommandTokens,
  parseAssistantSlashCommand,
  parseAssistantSlashCommandInput,
} from '../assistantSlashCommands';

describe('assistantSlashCommands', () => {
  it('maps OpenCode-style session commands to local Assistant actions', () => {
    expect(parseAssistantSlashCommand('/help')).toBe('help');
    expect(parseAssistantSlashCommand('/commands')).toBe('help');
    expect(parseAssistantSlashCommand('/new')).toBe('new');
    expect(parseAssistantSlashCommand('/clear')).toBe('new');
    expect(parseAssistantSlashCommand('/sessions')).toBe('sessions');
    expect(parseAssistantSlashCommand('/resume')).toBe('sessions');
    expect(parseAssistantSlashCommand('/continue')).toBe('sessions');
    expect(parseAssistantSlashCommand('/compact')).toBe('compact');
    expect(parseAssistantSlashCommand('/summarize')).toBe('compact');
    expect(parseAssistantSlashCommand('/models')).toBe('models');
    expect(parseAssistantSlashCommand('/model')).toBe('models');
    expect(parseAssistantSlashCommand('/mo')).toBe('models');
    expect(parseAssistantSlashCommand('/providers')).toBe('providers');
    expect(parseAssistantSlashCommand('/connect')).toBe('providers');
    expect(parseAssistantSlashCommand('/settings')).toBe('providers');
    expect(parseAssistantSlashCommand('/keys')).toBe('providers');
    expect(parseAssistantSlashCommand('/status')).toBe('status');
    expect(parseAssistantSlashCommand('/runtime')).toBe('status');
    expect(parseAssistantSlashCommand('/health')).toBe('status');
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

  it('parses model commands with route arguments for local composer handling', () => {
    expect(parseAssistantSlashCommandInput('/model openrouter:qwen/qwen3.7-plus')).toEqual({
      action: 'models',
      args: 'openrouter:qwen/qwen3.7-plus',
    });
    expect(parseAssistantSlashCommandInput('/models default')).toEqual({
      action: 'models',
      args: 'default',
    });
    expect(parseAssistantSlashCommandInput('/mo previous')).toEqual({
      action: 'models',
      args: 'previous',
    });
    expect(parseAssistantSlashCommand('/model openrouter:qwen/qwen3.7-plus')).toBeNull();
  });

  it('does not parse arguments for non-model commands', () => {
    expect(parseAssistantSlashCommandInput('/copy this sentence')).toBeNull();
    expect(parseAssistantSlashCommandInput('/status now')).toBeNull();
  });

  it('filters commands by canonical name, alias, and description', () => {
    expect(filterAssistantSlashCommands('').map((command) => command.name)).toEqual([
      'help',
      'new',
      'sessions',
      'compact',
      'models',
      'providers',
      'status',
      'copy',
      'export',
      'fork',
      'undo',
      'redo',
    ]);
    expect(filterAssistantSlashCommands('resume').map((command) => command.name)).toEqual([
      'sessions',
    ]);
    expect(filterAssistantSlashCommands('summarize').map((command) => command.name)).toEqual([
      'compact',
    ]);
    expect(filterAssistantSlashCommands('provider').map((command) => command.name)).toEqual([
      'providers',
      'models',
    ]);
    expect(filterAssistantSlashCommands('connect').map((command) => command.name)).toEqual([
      'providers',
    ]);
    expect(filterAssistantSlashCommands('runtime').map((command) => command.name)).toEqual([
      'status',
    ]);
    expect(filterAssistantSlashCommands('commands').map((command) => command.name)).toEqual([
      'help',
    ]);
  });

  it('filters disabled commands from the prompt slash list by default', () => {
    expect(
      filterAssistantSlashCommands('compact', undefined, {
        availability: {
          compact: {
            disabled: true,
            reason: 'Requires transcript content.',
          },
        },
      }).map((command) => command.name),
    ).toEqual([]);
  });

  it('can include disabled commands for full command help', () => {
    const commands = filterAssistantSlashCommands('compact', undefined, {
      availability: {
        compact: {
          disabled: true,
          reason: 'Requires transcript content.',
        },
      },
      includeDisabled: true,
    });

    expect(commands).toHaveLength(1);
    expect(commands[0]).toEqual(
      expect.objectContaining({
        disabled: true,
        disabledReason: 'Requires transcript content.',
        name: 'compact',
      }),
    );
  });

  it('exposes canonical and alias tokens for the picker', () => {
    const help = filterAssistantSlashCommands('commands')[0];
    expect(getAssistantSlashCommandTokens(help)).toEqual(['help', 'commands']);
    const sessions = filterAssistantSlashCommands('resume')[0];
    expect(getAssistantSlashCommandTokens(sessions)).toEqual(['sessions', 'resume', 'continue']);
    const compact = filterAssistantSlashCommands('summarize')[0];
    expect(getAssistantSlashCommandTokens(compact)).toEqual(['compact', 'summarize']);
    const models = filterAssistantSlashCommands('mo')[0];
    expect(getAssistantSlashCommandTokens(models)).toEqual(['models', 'model', 'mo']);
    const providers = filterAssistantSlashCommands('connect')[0];
    expect(getAssistantSlashCommandTokens(providers)).toEqual([
      'providers',
      'connect',
      'settings',
      'keys',
    ]);
    const status = filterAssistantSlashCommands('status')[0];
    expect(getAssistantSlashCommandTokens(status)).toEqual(['status', 'runtime', 'health']);
  });
});
