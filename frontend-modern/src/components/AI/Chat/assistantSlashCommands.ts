export type AssistantSlashCommandAction =
  | 'compact'
  | 'copy'
  | 'export'
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
  disabled?: boolean;
  disabledReason?: string;
  description: string;
  name: string;
}

export type AssistantSlashCommandAvailability = Partial<
  Record<AssistantSlashCommandAction, { disabled: boolean; reason?: string }>
>;

export interface AssistantSlashCommandFilterOptions {
  availability?: AssistantSlashCommandAvailability;
  includeDisabled?: boolean;
}

export interface ParsedAssistantSlashCommand {
  action: AssistantSlashCommandAction;
  args: string;
}

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

const commandByToken = new Map<string, AssistantSlashCommandAction>(
  ASSISTANT_SLASH_COMMANDS.flatMap((command) => [
    [command.name, command.action],
    ...(command.aliases || []).map((alias) => [alias, command.action] as const),
  ]),
);

export const parseAssistantSlashCommandInput = (
  input: string,
): ParsedAssistantSlashCommand | null => {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) return null;

  const body = trimmed.slice(1);
  const match = body.match(/^(\S+)(?:\s+([\s\S]*))?$/);
  if (!match) return null;

  const token = match[1].toLowerCase();
  if (!token) return null;
  const action = commandByToken.get(token);
  if (!action) return null;

  const args = (match[2] || '').trim();
  if (args && action !== 'models') return null;

  return { action, args };
};

export const parseAssistantSlashCommand = (input: string): AssistantSlashCommandAction | null => {
  const parsed = parseAssistantSlashCommandInput(input);
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

const commandMatchesQuery = (command: AssistantSlashCommand, query: string) => {
  if (!query) return true;
  if (command.name.toLowerCase().includes(query)) return true;
  if (command.description.toLowerCase().includes(query)) return true;
  return commandAliases(command).some((alias) => alias.toLowerCase().includes(query));
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
  limit = ASSISTANT_SLASH_COMMANDS.length,
  options: AssistantSlashCommandFilterOptions = {},
): AssistantSlashCommand[] => {
  const normalizedQuery = normalizeSlashQuery(query);
  return ASSISTANT_SLASH_COMMANDS.map((command) =>
    commandWithAvailability(command, options.availability),
  )
    .filter((command) => options.includeDisabled || !command.disabled)
    .filter((command) => commandMatchesQuery(command, normalizedQuery))
    .map((command, index) => ({ command, index }))
    .sort((left, right) => {
      const scoreDelta =
        commandMatchScore(left.command, normalizedQuery) -
        commandMatchScore(right.command, normalizedQuery);
      return scoreDelta || left.index - right.index;
    })
    .slice(0, limit)
    .map(({ command }) => command);
};
