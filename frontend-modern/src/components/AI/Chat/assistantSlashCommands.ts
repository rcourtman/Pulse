import {
  AI_CHAT_DEV_STREAM_FIXTURE_ALIAS_NAMES,
  AI_CHAT_DEV_STREAM_FIXTURE_NAMES,
} from '@/api/aiChatDevStreamFixture';

export type AssistantSlashCommandAction =
  | 'compact'
  | 'copy'
  | 'export'
  | 'fixture'
  | 'fork'
  | 'help'
  | 'models'
  | 'new'
  | 'providers'
  | 'redo'
  | 'status'
  | 'undo'
  | 'sessions';

export interface AssistantSlashCommand {
  action: AssistantSlashCommandAction;
  aliases?: string[];
  acceptsArgs?: boolean;
  disabled?: boolean;
  disabledReason?: string;
  description: string;
  insertText?: string;
  keywords?: string[];
  name: string;
}

export type AssistantSlashCommandAvailability = Partial<
  Record<AssistantSlashCommandAction, { disabled: boolean; reason?: string }>
>;

export interface AssistantSlashCommandFilterOptions {
  availability?: AssistantSlashCommandAvailability;
  includeDevCommands?: boolean;
  includeDisabled?: boolean;
}

export interface AssistantSlashCommandParseOptions {
  includeDevCommands?: boolean;
}

export interface ParsedAssistantSlashCommand {
  action: AssistantSlashCommandAction;
  args: string;
}

const isAssistantDevCommandSurfaceEnabled = () =>
  import.meta.env.DEV || import.meta.env.MODE === 'test';

const includeDevCommandsForOptions = (includeDevCommands?: boolean) =>
  includeDevCommands ?? isAssistantDevCommandSurfaceEnabled();

const DEV_ASSISTANT_SLASH_COMMANDS: AssistantSlashCommand[] = [
  {
    name: 'fixture',
    aliases: ['fixtures'],
    acceptsArgs: true,
    action: 'fixture',
    description: 'Run a local stream fixture by name (/fixture provider-retry)',
    insertText: '/fixture ',
    keywords: [
      'dev',
      'local',
      'stream',
      'test',
      ...AI_CHAT_DEV_STREAM_FIXTURE_NAMES,
      ...AI_CHAT_DEV_STREAM_FIXTURE_ALIAS_NAMES,
    ],
  },
];

export const ASSISTANT_SLASH_COMMANDS: AssistantSlashCommand[] = [
  {
    name: 'help',
    aliases: ['commands'],
    action: 'help',
    description: 'Show Assistant commands',
  },
  {
    name: 'new',
    aliases: ['clear'],
    action: 'new',
    description: 'Start a blank Assistant session',
  },
  {
    name: 'sessions',
    aliases: ['resume', 'continue'],
    action: 'sessions',
    description: 'Open Assistant session history',
  },
  {
    name: 'compact',
    aliases: ['summarize'],
    action: 'compact',
    description: 'Summarize older turns and keep this session moving',
  },
  {
    name: 'models',
    aliases: ['model', 'mo'],
    acceptsArgs: true,
    action: 'models',
    description: 'Open model search or set a route (/model qwen or /model provider:model-id)',
  },
  {
    name: 'providers',
    aliases: ['connect', 'settings', 'keys'],
    action: 'providers',
    description: 'Open Assistant provider settings',
  },
  {
    name: 'status',
    aliases: ['runtime', 'health'],
    action: 'status',
    description: 'Check the selected model route',
  },
  {
    name: 'copy',
    action: 'copy',
    description: 'Copy the current transcript',
  },
  {
    name: 'export',
    action: 'export',
    description: 'Download the current transcript',
  },
  {
    name: 'fork',
    action: 'fork',
    description: 'Fork this session into a new copy',
  },
  {
    name: 'undo',
    action: 'undo',
    description: 'Restore the last prompt for editing',
  },
  {
    name: 'redo',
    action: 'redo',
    description: 'Restore the last undone turn',
  },
];

const assistantSlashCommandsForOptions = (includeDevCommands?: boolean): AssistantSlashCommand[] =>
  includeDevCommandsForOptions(includeDevCommands)
    ? [...ASSISTANT_SLASH_COMMANDS, ...DEV_ASSISTANT_SLASH_COMMANDS]
    : ASSISTANT_SLASH_COMMANDS;

const commandEntriesForOptions = (
  includeDevCommands?: boolean,
): Array<readonly [string, AssistantSlashCommandAction]> =>
  assistantSlashCommandsForOptions(includeDevCommands).flatMap((command) => [
    [command.name, command.action] as const,
    ...(command.aliases || []).map((alias) => [alias, command.action] as const),
  ]);

const commandByToken = (includeDevCommands?: boolean) =>
  new Map<string, AssistantSlashCommandAction>(commandEntriesForOptions(includeDevCommands));

const commandForAction = (action: AssistantSlashCommandAction, includeDevCommands?: boolean) =>
  assistantSlashCommandsForOptions(includeDevCommands).find((command) => command.action === action);

export const parseAssistantSlashCommandInput = (
  input: string,
  options: AssistantSlashCommandParseOptions = {},
): ParsedAssistantSlashCommand | null => {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) return null;

  const body = trimmed.slice(1);
  const match = body.match(/^(\S+)(?:\s+([\s\S]*))?$/);
  if (!match) return null;

  const token = match[1].toLowerCase();
  if (!token) return null;
  const action = commandByToken(options.includeDevCommands).get(token);
  if (!action) return null;

  const args = (match[2] || '').trim();
  const command = commandForAction(action, options.includeDevCommands);
  if (args && !command?.acceptsArgs) return null;

  return { action, args };
};

export const parseAssistantSlashCommand = (
  input: string,
  options: AssistantSlashCommandParseOptions = {},
): AssistantSlashCommandAction | null => {
  const parsed = parseAssistantSlashCommandInput(input, options);
  if (!parsed || parsed.args) return null;
  return parsed.action;
};

const normalizeSlashQuery = (query: string) => query.trim().toLowerCase();

const commandAliases = (command: AssistantSlashCommand): string[] => command.aliases || [];

const commandWithAvailability = (
  command: AssistantSlashCommand,
  availability?: AssistantSlashCommandAvailability,
): AssistantSlashCommand => {
  const state = availability?.[command.action];
  if (!state) return command;
  return {
    ...command,
    disabled: state.disabled,
    disabledReason: state.reason,
  };
};

const keywordMatchesQuery = (keyword: string, query: string) => {
  const normalizedKeyword = keyword.toLowerCase();
  return (
    normalizedKeyword === query ||
    normalizedKeyword.startsWith(`${query}-`) ||
    normalizedKeyword.startsWith(`${query}_`)
  );
};

const commandMatchesQuery = (command: AssistantSlashCommand, query: string) => {
  if (!query) return true;
  if (command.name.toLowerCase().includes(query)) return true;
  if (command.description.toLowerCase().includes(query)) return true;
  if (commandAliases(command).some((alias) => alias.toLowerCase().includes(query))) return true;
  return (command.keywords || []).some((keyword) => keywordMatchesQuery(keyword, query));
};

const commandMatchScore = (command: AssistantSlashCommand, query: string) => {
  if (!query) return 0;
  if (command.name.toLowerCase().startsWith(query)) return 0;
  if (commandAliases(command).some((alias) => alias.toLowerCase().startsWith(query))) return 1;
  if (command.name.toLowerCase().includes(query)) return 2;
  if (commandAliases(command).some((alias) => alias.toLowerCase().includes(query))) return 3;
  return 4;
};

export const getAssistantSlashCommandTokens = (command: AssistantSlashCommand): string[] => [
  command.name,
  ...commandAliases(command),
];

export const filterAssistantSlashCommands = (
  query: string,
  limit?: number,
  options: AssistantSlashCommandFilterOptions = {},
): AssistantSlashCommand[] => {
  const normalizedQuery = normalizeSlashQuery(query);
  const commands = assistantSlashCommandsForOptions(options.includeDevCommands);
  return commands
    .map((command) => commandWithAvailability(command, options.availability))
    .filter((command) => options.includeDisabled || !command.disabled)
    .filter((command) => commandMatchesQuery(command, normalizedQuery))
    .map((command, index) => ({ command, index }))
    .sort((left, right) => {
      const scoreDelta =
        commandMatchScore(left.command, normalizedQuery) -
        commandMatchScore(right.command, normalizedQuery);
      return scoreDelta || left.index - right.index;
    })
    .slice(0, limit ?? commands.length)
    .map(({ command }) => command);
};
