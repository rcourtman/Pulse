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

  it('suppresses compacted internal prose before raw function-call leaks', () => {
    const result = stripAssistantOutputArtifacts(
      'I\'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.pulse_read(target_host="current_resource", command="lsblk")',
    );

    expect(result.text).toBe('');
    expect(result.stripped).toBe(true);
  });

  it('suppresses content-channel reasoning preludes before raw function-call leaks', () => {
    const result = stripAssistantOutputArtifacts(
      'Thinking\nWe need to interpret the user question and count the device nodes.\npulse_read(target_host="current_resource", command="ls /dev | wc -l")',
    );

    expect(result).toEqual({
      text: '',
      stripped: true,
    });
  });

  it('leaves ordinary prose and unrelated function calls alone', () => {
    expect(stripAssistantOutputArtifacts('Call helper(target="x") in the example.')).toEqual({
      text: 'Call helper(target="x") in the example.',
      stripped: false,
    });
  });

  it('normalizes internal tool identifiers in visible assistant prose', () => {
    expect(
      stripAssistantOutputArtifacts(
        'The burst fixture completed a fast `pulse_read` command and kept the tool transition visible.',
      ),
    ).toEqual({
      text: 'The burst fixture completed a fast read command and kept the tool transition visible.',
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

  it('holds compacted prose so a split tool leak never flashes in the transcript', () => {
    const state = createAssistantOutputArtifactStreamState();
    const first =
      "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.";

    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, first)).toEqual({
      text: '',
      stripped: false,
    });
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'pulse_read(target_host="current_resource", command="lsblk")',
      ),
    ).toEqual({
      text: '',
      stripped: true,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });

  it('drops held compacted prose when no tool leak follows before stream end', () => {
    const state = createAssistantOutputArtifactStreamState();
    const content =
      "Thisisbadmodelspacingbutitistheactualanswerbecauseitneverturnsintoatoolcall.";

    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, content)).toEqual({
      text: '',
      stripped: false,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });

  it('holds content-channel reasoning preludes so they never flash before tool leaks', () => {
    const state = createAssistantOutputArtifactStreamState();

    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'Thinking\nWe need to count device nodes before answering.',
      ),
    ).toEqual({
      text: '',
      stripped: false,
    });
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        '\npulse_read(target_host="current_resource", command="ls /dev | wc -l")',
      ),
    ).toEqual({
      text: '',
      stripped: true,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });

  it('drops held content-channel reasoning if no answer text follows before stream end', () => {
    const state = createAssistantOutputArtifactStreamState();

    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'Thinking\nWe need to inspect the prompt before answering.',
      ),
    ).toEqual({
      text: '',
      stripped: false,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });

  it('releases normal prose that only starts with a similar word', () => {
    const state = createAssistantOutputArtifactStreamState();

    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Think')).toEqual({
      text: '',
      stripped: false,
    });
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, ' about the result this way.')).toEqual({
      text: 'Think about the result this way.',
      stripped: false,
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

  it('normalizes streamed tool identifiers without flashing a split code token', () => {
    const state = createAssistantOutputArtifactStreamState();

    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'The fast `pulse_')).toEqual({
      text: 'The fast ',
      stripped: false,
    });
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'read` command kept the tool transition visible.',
      ),
    ).toEqual({
      text: 'read command kept the tool transition visible.',
      stripped: false,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });
});
