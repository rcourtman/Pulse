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
});
