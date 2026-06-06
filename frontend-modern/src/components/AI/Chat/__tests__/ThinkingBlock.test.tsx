import { describe, expect, it, afterEach } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { ThinkingBlock } from '../ThinkingBlock';

afterEach(cleanup);

describe('ThinkingBlock', () => {
  it('renders a neutral completed-thinking status without reasoning content', () => {
    render(() => <ThinkingBlock content="We need to inspect the prompt before answering." />);

    expect(screen.getByRole('status')).toHaveTextContent('Thinking complete');
    expect(screen.queryByText(/inspect the prompt/i)).not.toBeInTheDocument();
  });

  it('renders a neutral streaming-thinking status without reasoning content', () => {
    render(() => <ThinkingBlock content="Hidden provider reasoning" isStreaming={true} />);

    expect(screen.getByRole('status')).toHaveTextContent('Thinking...');
    expect(screen.queryByText(/Hidden provider reasoning/i)).not.toBeInTheDocument();
  });

  it('marks the icon as active while streaming', () => {
    const { container } = render(() => <ThinkingBlock content="hidden" isStreaming={true} />);

    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  it('uses provider reasoning summary metadata without showing the reasoning body', () => {
    render(() => (
      <ThinkingBlock
        content={'**Inspecting storage state**\n\nHidden provider reasoning body.'}
        startedAt={1_000}
        updatedAt={4_250}
      />
    ));

    expect(screen.getByRole('status')).toHaveTextContent('Thought: Inspecting storage state · 3s');
    expect(screen.queryByText(/Hidden provider reasoning body/i)).not.toBeInTheDocument();
  });

  it('shows summary metadata while streaming without exposing raw reasoning text', () => {
    render(() => (
      <ThinkingBlock
        content={'**Checking resource context**\n\nNeed to inspect hidden state.'}
        isStreaming={true}
        startedAt={Date.now()}
      />
    ));

    expect(screen.getByRole('status')).toHaveTextContent('Thinking: Checking resource context');
    expect(screen.queryByText(/Need to inspect hidden state/i)).not.toBeInTheDocument();
  });
});
