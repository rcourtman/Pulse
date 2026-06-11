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
  | 'queue'
  | 'redo'
  | 'status'
  | 'undo'
  | 'sessions';

export type AssistantSlashCommandCategory =
  | 'Session'
  | 'Model'
  | 'Transcript'
  | 'Help'
  | 'Developer';

export interface AssistantSlashCommand {
  action: AssistantSlashCommandAction;
  aliases?: string[];
  acceptsArgs?: boolean;
  category: AssistantSlashCommandCategory;
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

export interface AssistantSlashCommandGroup {
  category: AssistantSlashCommandCategory;
  items: Array<{
    command: AssistantSlashCommand;
    index: number;
  }>;
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
    category: 'Developer',
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
    name: 'new',
    aliases: ['clear'],
    action: 'new',
    category: 'Session',
    description: 'Start a blank Assistant session',
  },
  {
    name: 'sessions',
    aliases: ['resume', 'continue'],
    acceptsArgs: true,
    action: 'sessions',
    category: 'Session',
    description: 'Search or resume Assistant sessions (/sessions backup)',
  },
  {
    name: 'queue',
    aliases: ['queued', 'prompts'],
    action: 'queue',
    category: 'Session',
    description: 'Manage queued follow-ups',
    keywords: ['follow-up', 'follow-ups', 'queued', 'prompt', 'prompts'],
  },
  {
    name: 'compact',
    aliases: ['summarize'],
    action: 'compact',
    category: 'Session',
    description: 'Summarize older turns and keep this session moving',
  },
  {
    name: 'fork',
    action: 'fork',
    category: 'Session',
    description: 'Fork this session into a new copy',
  },
  {
    name: 'undo',
    action: 'undo',
    category: 'Session',
    description: 'Restore the last prompt for editing',
  },
  {
    name: 'redo',
    action: 'redo',
    category: 'Session',
    description: 'Restore the last undone turn',
  },
  {
    name: 'models',
    aliases: ['model', 'mo'],
    acceptsArgs: true,
    action: 'models',
    category: 'Model',
    description: 'Open model search or set a route (/model openrouter/qwen or provider:model-id)',
  },
  {
    name: 'providers',
    aliases: ['connect', 'settings', 'keys'],
    action: 'providers',
    category: 'Model',
    description: 'Open Assistant provider settings',
  },
  {
    name: 'status',
    aliases: ['runtime', 'health'],
    action: 'status',
    category: 'Model',
    description: 'Check the selected model route',
  },
  {
    name: 'copy',
    action: 'copy',
    category: 'Transcript',
    description: 'Copy the current transcript',
  },
  {
    name: 'export',
    action: 'export',
    category: 'Transcript',
    description: 'Download the current transcript',
  },
  {
    name: 'help',
    aliases: ['commands'],
    action: 'help',
    category: 'Help',
    description: 'Show Assistant commands',
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
    // The availability map computes a fall-through reason even when the
    // command is enabled; carrying it through made the menu show false
    // statements like "Forking is already running." on enabled commands.
    disabledReason: state.disabled ? state.reason : undefined,
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

export const groupAssistantSlashCommands = (
  commands: AssistantSlashCommand[],
): AssistantSlashCommandGroup[] => {
  const groups: AssistantSlashCommandGroup[] = [];
  for (const [index, command] of commands.entries()) {
    const lastGroup = groups[groups.length - 1];
    if (!lastGroup || lastGroup.category !== command.category) {
      groups.push({ category: command.category, items: [] });
    }
    groups[groups.length - 1].items.push({ command, index });
  }
  return groups;
};

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
