import { describe, expect, it } from 'vitest';
import {
  appendVisibleTextBeforeAssistantOutputArtifacts,
  createAssistantOutputArtifactStreamState,
  flushPendingAssistantOutputText,
  stripAssistantOutputArtifacts,
} from '../assistantOutputHygiene';

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

  it('strips pulse-like tool calls even when the frontend has not learned a new tool name yet', () => {
    const result = stripAssistantOutputArtifacts(
      'Checking it now.\npulse_future_tool(target_host="current_resource")',
    );

    expect(result.text).toBe('Checking it now.');
    expect(result.stripped).toBe(true);
  });

  it('leaves ordinary prose and unrelated function calls alone', () => {
    expect(stripAssistantOutputArtifacts('Call helper(target="x") in the example.')).toEqual({
      text: 'Call helper(target="x") in the example.',
      stripped: false,
    });
  });

  it('holds split tool-name prefixes until the next stream delta proves the shape', () => {
    const state = createAssistantOutputArtifactStreamState();

    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'I will check pu')).toEqual({
      text: 'I will check ',
      stripped: false,
    });
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'lse_read(target_host="current_resource", command="lsblk")',
      ),
    ).toEqual({
      text: '',
      stripped: true,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });

  it('releases a held prefix when the next delta proves it is normal prose', () => {
    const state = createAssistantOutputArtifactStreamState();

    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'The p')).toEqual({
      text: 'The ',
      stripped: false,
    });
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'latform is healthy.')).toEqual({
      text: 'platform is healthy.',
      stripped: false,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });
});
