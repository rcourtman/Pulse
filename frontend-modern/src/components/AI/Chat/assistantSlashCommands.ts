export type AssistantSlashCommandAction =
  | 'copy'
  | 'export'
  | 'fork'
  | 'models'
  | 'new'
  | 'sessions';

export interface AssistantSlashCommand {
  action: AssistantSlashCommandAction;
  aliases?: string[];
  name: string;
}

export const ASSISTANT_SLASH_COMMANDS: AssistantSlashCommand[] = [
  { name: 'new', aliases: ['clear'], action: 'new' },
  { name: 'sessions', aliases: ['resume', 'continue'], action: 'sessions' },
  { name: 'models', aliases: ['model'], action: 'models' },
  { name: 'copy', action: 'copy' },
  { name: 'export', action: 'export' },
  { name: 'fork', action: 'fork' },
];

const commandByToken = new Map<string, AssistantSlashCommandAction>(
  ASSISTANT_SLASH_COMMANDS.flatMap((command) => [
    [command.name, command.action],
    ...(command.aliases || []).map((alias) => [alias, command.action] as const),
  ]),
);

export const parseAssistantSlashCommand = (
  input: string,
): AssistantSlashCommandAction | null => {
  const trimmed = input.trim();
  if (!trimmed.startsWith('/')) return null;

  const token = trimmed.slice(1).toLowerCase();
  if (!token || /\s/.test(token)) return null;

  return commandByToken.get(token) ?? null;
};
