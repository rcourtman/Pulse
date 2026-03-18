import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { QuestionCard } from '../QuestionCard';
import type { PendingQuestion, Question } from '../types';

afterEach(cleanup);

function makeQuestion(overrides?: Partial<Question>): Question {
  return {
    id: 'q1',
    type: 'text',
    question: 'What is the hostname?',
    ...overrides,
  };
}

function makePendingQuestion(overrides?: Partial<PendingQuestion>): PendingQuestion {
  return {
    questionId: 'pq-1',
    questions: [makeQuestion()],
    ...overrides,
  };
}

describe('QuestionCard', () => {
  it('renders the header "Question from Pulse Assistant"', () => {
    render(() => (
      <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
    ));

    expect(screen.getByText('Question from Pulse Assistant')).toBeInTheDocument();
  });

  it('renders the question text', () => {
    render(() => (
      <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
    ));

    expect(screen.getByText('What is the hostname?')).toBeInTheDocument();
  });

  it('renders Submit Answer and Skip buttons', () => {
    render(() => (
      <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
    ));

    expect(screen.getByText('Submit Answer')).toBeInTheDocument();
    expect(screen.getByText('Skip')).toBeInTheDocument();
  });

  it('renders a text input for type "text" questions', () => {
    render(() => (
      <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
    ));

    expect(screen.getByPlaceholderText('Type your answer...')).toBeInTheDocument();
  });

  it('renders the question header when provided', () => {
    render(() => (
      <QuestionCard
        question={makePendingQuestion({
          questions: [makeQuestion({ header: 'Network Configuration' })],
        })}
        onAnswer={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.getByText('Network Configuration')).toBeInTheDocument();
  });

  it('does not render a header when not provided', () => {
    render(() => (
      <QuestionCard
        question={makePendingQuestion({
          questions: [makeQuestion({ header: undefined })],
        })}
        onAnswer={vi.fn()}
        onSkip={vi.fn()}
      />
    ));

    expect(screen.queryByText('Network Configuration')).not.toBeInTheDocument();
  });

  describe('text input', () => {
    it('submit button is disabled when text input is empty', () => {
      render(() => (
        <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
      ));

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).toBeDisabled();
    });

    it('submit button is enabled after typing an answer', () => {
      render(() => (
        <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
      ));

      const input = screen.getByPlaceholderText('Type your answer...');
      fireEvent.input(input, { target: { value: 'my-host' } });

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).not.toBeDisabled();
    });

    it('calls onAnswer with the typed value when submitted', () => {
      const onAnswer = vi.fn();
      render(() => (
        <QuestionCard question={makePendingQuestion()} onAnswer={onAnswer} onSkip={vi.fn()} />
      ));

      const input = screen.getByPlaceholderText('Type your answer...');
      fireEvent.input(input, { target: { value: 'web-server-01' } });
      fireEvent.click(screen.getByText('Submit Answer'));

      expect(onAnswer).toHaveBeenCalledOnce();
      expect(onAnswer).toHaveBeenCalledWith([{ id: 'q1', value: 'web-server-01' }]);
    });

    it('submit button stays disabled for whitespace-only input', () => {
      render(() => (
        <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={vi.fn()} />
      ));

      const input = screen.getByPlaceholderText('Type your answer...');
      fireEvent.input(input, { target: { value: '   ' } });

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).toBeDisabled();
    });
  });

  describe('select input', () => {
    const selectQuestion = makePendingQuestion({
      questions: [
        makeQuestion({
          id: 'q-select',
          type: 'select',
          question: 'Choose an action',
          options: [
            { label: 'Restart', value: 'restart', description: 'Restart the service' },
            { label: 'Stop', value: 'stop' },
          ],
        }),
      ],
    });

    it('renders option labels', () => {
      render(() => <QuestionCard question={selectQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      expect(screen.getByText('Restart')).toBeInTheDocument();
      expect(screen.getByText('Stop')).toBeInTheDocument();
    });

    it('renders option descriptions when provided', () => {
      render(() => <QuestionCard question={selectQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      expect(screen.getByText('Restart the service')).toBeInTheDocument();
    });

    it('submit is disabled before selecting an option', () => {
      render(() => <QuestionCard question={selectQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).toBeDisabled();
    });

    it('submit is enabled after selecting an option', () => {
      render(() => <QuestionCard question={selectQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      fireEvent.click(screen.getByText('Restart'));

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).not.toBeDisabled();
    });

    it('calls onAnswer with the selected option value', () => {
      const onAnswer = vi.fn();
      render(() => <QuestionCard question={selectQuestion} onAnswer={onAnswer} onSkip={vi.fn()} />);

      fireEvent.click(screen.getByText('Stop'));
      fireEvent.click(screen.getByText('Submit Answer'));

      expect(onAnswer).toHaveBeenCalledOnce();
      expect(onAnswer).toHaveBeenCalledWith([{ id: 'q-select', value: 'stop' }]);
    });

    it('selecting a different option changes the answer', () => {
      const onAnswer = vi.fn();
      render(() => <QuestionCard question={selectQuestion} onAnswer={onAnswer} onSkip={vi.fn()} />);

      fireEvent.click(screen.getByText('Restart'));
      fireEvent.click(screen.getByText('Stop'));
      fireEvent.click(screen.getByText('Submit Answer'));

      expect(onAnswer).toHaveBeenCalledWith([{ id: 'q-select', value: 'stop' }]);
    });
  });

  describe('multiple questions', () => {
    const multiQuestion = makePendingQuestion({
      questions: [
        makeQuestion({ id: 'q-name', question: 'Enter the name' }),
        makeQuestion({
          id: 'q-action',
          type: 'select',
          question: 'Choose action',
          options: [
            { label: 'Create', value: 'create' },
            { label: 'Update', value: 'update' },
          ],
        }),
      ],
    });

    it('renders all questions', () => {
      render(() => <QuestionCard question={multiQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      expect(screen.getByText('Enter the name')).toBeInTheDocument();
      expect(screen.getByText('Choose action')).toBeInTheDocument();
    });

    it('submit is disabled until all questions are answered', () => {
      render(() => <QuestionCard question={multiQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      // Answer only the text question
      const input = screen.getByPlaceholderText('Type your answer...');
      fireEvent.input(input, { target: { value: 'test-name' } });

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).toBeDisabled();
    });

    it('submit is enabled when all questions are answered', () => {
      render(() => <QuestionCard question={multiQuestion} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      const input = screen.getByPlaceholderText('Type your answer...');
      fireEvent.input(input, { target: { value: 'test-name' } });
      fireEvent.click(screen.getByText('Create'));

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).not.toBeDisabled();
    });

    it('submits all answers in order', () => {
      const onAnswer = vi.fn();
      render(() => <QuestionCard question={multiQuestion} onAnswer={onAnswer} onSkip={vi.fn()} />);

      const input = screen.getByPlaceholderText('Type your answer...');
      fireEvent.input(input, { target: { value: 'my-resource' } });
      fireEvent.click(screen.getByText('Update'));
      fireEvent.click(screen.getByText('Submit Answer'));

      expect(onAnswer).toHaveBeenCalledWith([
        { id: 'q-name', value: 'my-resource' },
        { id: 'q-action', value: 'update' },
      ]);
    });
  });

  describe('skip', () => {
    it('calls onSkip when Skip is clicked', () => {
      const onSkip = vi.fn();
      render(() => (
        <QuestionCard question={makePendingQuestion()} onAnswer={vi.fn()} onSkip={onSkip} />
      ));

      fireEvent.click(screen.getByText('Skip'));
      expect(onSkip).toHaveBeenCalledOnce();
    });
  });

  describe('isAnswering state', () => {
    it('shows "Answering..." instead of "Submit Answer"', () => {
      render(() => (
        <QuestionCard
          question={makePendingQuestion({ isAnswering: true })}
          onAnswer={vi.fn()}
          onSkip={vi.fn()}
        />
      ));

      expect(screen.getByText('Answering...')).toBeInTheDocument();
      expect(screen.queryByText('Submit Answer')).not.toBeInTheDocument();
    });

    it('disables both buttons when isAnswering is true', () => {
      render(() => (
        <QuestionCard
          question={makePendingQuestion({ isAnswering: true })}
          onAnswer={vi.fn()}
          onSkip={vi.fn()}
        />
      ));

      const buttons = screen.getAllByRole('button');
      for (const button of buttons) {
        expect(button).toBeDisabled();
      }
    });

    it('disables the text input when isAnswering is true', () => {
      render(() => (
        <QuestionCard
          question={makePendingQuestion({ isAnswering: true })}
          onAnswer={vi.fn()}
          onSkip={vi.fn()}
        />
      ));

      expect(screen.getByPlaceholderText('Type your answer...')).toBeDisabled();
    });

    it('disables select option buttons when isAnswering is true', () => {
      const selectQ = makePendingQuestion({
        isAnswering: true,
        questions: [
          makeQuestion({
            type: 'select',
            question: 'Pick one',
            options: [
              { label: 'A', value: 'a' },
              { label: 'B', value: 'b' },
            ],
          }),
        ],
      });

      render(() => <QuestionCard question={selectQ} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      // All buttons (option A, option B, submit, skip) should be disabled
      const buttons = screen.getAllByRole('button');
      for (const button of buttons) {
        expect(button).toBeDisabled();
      }
    });

    it('does not call onAnswer when submit is clicked while answering', () => {
      const onAnswer = vi.fn();
      render(() => (
        <QuestionCard
          question={makePendingQuestion({ isAnswering: true })}
          onAnswer={onAnswer}
          onSkip={vi.fn()}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: /answering/i }));
      expect(onAnswer).not.toHaveBeenCalled();
    });

    it('does not call onSkip when skip is clicked while answering', () => {
      const onSkip = vi.fn();
      render(() => (
        <QuestionCard
          question={makePendingQuestion({ isAnswering: true })}
          onAnswer={vi.fn()}
          onSkip={onSkip}
        />
      ));

      fireEvent.click(screen.getByRole('button', { name: /skip/i }));
      expect(onSkip).not.toHaveBeenCalled();
    });
  });

  describe('edge cases', () => {
    it('blocks submission when only some text questions are answered', () => {
      const multiQ = makePendingQuestion({
        questions: [
          makeQuestion({ id: 'q1', question: 'First' }),
          makeQuestion({ id: 'q2', question: 'Second' }),
        ],
      });

      render(() => <QuestionCard question={multiQ} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      // Answer only the first of two text questions
      const inputs = screen.getAllByPlaceholderText('Type your answer...');
      expect(inputs).toHaveLength(2);
      fireEvent.input(inputs[0], { target: { value: 'answer-1' } });

      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).toBeDisabled();
    });

    it('renders select question with no options without crashing', () => {
      const noOptionsQ = makePendingQuestion({
        questions: [
          makeQuestion({
            id: 'q-empty',
            type: 'select',
            question: 'Pick something',
            options: undefined,
          }),
        ],
      });

      render(() => <QuestionCard question={noOptionsQ} onAnswer={vi.fn()} onSkip={vi.fn()} />);

      expect(screen.getByText('Pick something')).toBeInTheDocument();
      // Submit should be disabled since there is no way to answer
      const submitBtn = screen.getByText('Submit Answer').closest('button')!;
      expect(submitBtn).toBeDisabled();
    });
  });
});
