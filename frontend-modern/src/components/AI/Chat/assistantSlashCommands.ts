export type AssistantSlashCommandAction =
  | 'copy'
  | 'export'
  | 'fork'
  | 'help'
  | 'models'
  | 'new'
  | 'redo'
  | 'status'
  | 'undo'
  | 'sessions';

export interface AssistantSlashCommand {
  action: AssistantSlashCommandAction;
  aliases?: string[];
  description: string;
  name: string;
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
    name: 'models',
    aliases: ['model', 'mo'],
    action: 'models',
    description: 'Choose the model and provider route',
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

export const parseAssistantSlashCommand = (input: string): AssistantSlashCommandAction | null => {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) return null;

  const token = trimmed.slice(1).toLowerCase();
  if (!token || /\s/.test(token)) return null;

  return commandByToken.get(token) ?? null;
};

const normalizeSlashQuery = (query: string) => query.trim().toLowerCase();

const commandAliases = (command: AssistantSlashCommand): string[] => command.aliases || [];

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
): AssistantSlashCommand[] => {
  const normalizedQuery = normalizeSlashQuery(query);
  return ASSISTANT_SLASH_COMMANDS.filter((command) => commandMatchesQuery(command, normalizedQuery))
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
