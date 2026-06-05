import { describe, expect, it } from 'vitest';
import { stripAssistantOutputArtifacts } from '../assistantOutputHygiene';

describe('stripAssistantOutputArtifacts', () => {
  it('strips plain Pulse function-call leaks while preserving prose before them', () => {
    const result = stripAssistantOutputArtifacts(
      'I will inspect the device nodes.\npulse_read(target_host="current_resource", command="ls /dev | wc -l")',
    );

    expect(result).toEqual({
      text: 'I will inspect the device nodes.',
      stripped: true,
    });
  });

  it('strips JSON tool-call leaks', () => {
    const result = stripAssistantOutputArtifacts(
      'Looking it up now.\n{"name":"pulse_query","parameters":{"action":"list"}}',
    );

    expect(result.text).toBe('Looking it up now.');
    expect(result.stripped).toBe(true);
  });

  it('leaves ordinary prose and unrelated function calls alone', () => {
    expect(stripAssistantOutputArtifacts('Call helper(target="x") in the example.')).toEqual({
      text: 'Call helper(target="x") in the example.',
      stripped: false,
    });
  });
});
