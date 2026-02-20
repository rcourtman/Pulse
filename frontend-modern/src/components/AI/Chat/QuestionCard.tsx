import { Component, Show, For, createSignal } from 'solid-js';
import type { PendingQuestion } from './types';

interface QuestionCardProps {
  question: PendingQuestion;
  onAnswer: (answers: Array<{ id: string; value: string }>) => void;
  onSkip: () => void;
}

export const QuestionCard: Component<QuestionCardProps> = (props) => {
  // Store answers for each question
  const [answers, setAnswers] = createSignal<Record<string, string>>({});

  const handleInputChange = (questionId: string, value: string) => {
    setAnswers((prev) => ({ ...prev, [questionId]: value }));
  };

  const handleSelectOption = (questionId: string, value: string) => {
    setAnswers((prev) => ({ ...prev, [questionId]: value }));
  };

  const handleSubmit = () => {
    const answerArray = props.question.questions.map((q) => ({
      id: q.id,
      value: answers()[q.id] || '',
    }));
    props.onAnswer(answerArray);
  };

  const isValid = () => {
    // Check all questions have answers
    return props.question.questions.every((q) => {
      const answer = answers()[q.id];
      return answer && answer.trim() !== '';
    });
  };

  return (
    <div class="rounded-md border border-blue-300 dark:border-blue-700 overflow-hidden shadow-sm">
      {/* Header */}
      <div class="px-3 py-2 text-xs font-medium flex items-center gap-2 bg-blue-50 dark:bg-blue-900 text-blue-800 dark:text-blue-200 border-b border-blue-200 dark:border-blue-800">
        <div class="p-1 rounded bg-blue-100 dark:bg-blue-800">
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <span class="font-semibold">Question from Pulse Assistant</span>
      </div>

      {/* Questions */}
      <div class="px-3 py-3 bg-blue-50 dark:bg-blue-900 space-y-4">
        <For each={props.question.questions}>
          {(q) => (
            <div class="space-y-2">
              <Show when={q.header}>
                <p class="text-[11px] text-slate-600 dark:text-slate-300">
                  {q.header}
                </p>
              </Show>
              <p class="text-sm font-medium text-slate-800 dark:text-slate-200">
                {q.question}
              </p>

              <Show when={q.type === 'text'}>
                <input
                  type="text"
                  value={answers()[q.id] || ''}
                  onInput={(e) => handleInputChange(q.id, e.currentTarget.value)}
                  class="w-full px-3 py-2 text-sm border border-blue-200 dark:border-blue-700 rounded-md bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="Type your answer..."
                  disabled={props.question.isAnswering}
                />
              </Show>

              <Show when={q.type === 'select' && q.options}>
                <div class="space-y-1">
                  <For each={q.options}>
                    {(option) => (
                      <button
                        type="button"
                        onClick={() => handleSelectOption(q.id, option.value)}
                        disabled={props.question.isAnswering}
                        class={`w-full px-3 py-2 text-sm text-left rounded-md border transition-colors ${
                          answers()[q.id] === option.value
                            ? 'bg-blue-100 dark:bg-blue-800 border-blue-400 dark:border-blue-600 text-blue-800 dark:text-blue-200'
                            : 'bg-white dark:bg-slate-800 border-blue-200 dark:border-blue-700 text-slate-700 dark:text-slate-300 hover:bg-blue-50 dark:hover:bg-blue-900'
                        } ${props.question.isAnswering ? 'opacity-50 cursor-not-allowed' : ''}`}
                      >
                        <div class="flex flex-col gap-0.5">
                          <span>{option.label}</span>
                          <Show when={option.description}>
                            <span class="text-[11px] text-slate-500 dark:text-slate-400">
                              {option.description}
                            </span>
                          </Show>
                        </div>
                      </button>
                    )}
                  </For>
                </div>
              </Show>
            </div>
          )}
        </For>

        {/* Actions */}
        <div class="flex gap-2 pt-2">
          <button
            type="button"
            onClick={handleSubmit}
            disabled={props.question.isAnswering || !isValid()}
            class={`flex-1 px-3 py-2 text-xs font-semibold rounded-md transition-all ${
              props.question.isAnswering
                ? 'bg-blue-400 text-white cursor-wait'
                : !isValid()
                ? 'bg-slate-300 dark:bg-slate-600 text-slate-500 dark:text-slate-400 cursor-not-allowed'
                : 'bg-blue-500 hover:bg-blue-600 text-white shadow-sm hover:shadow-sm'
            }`}
          >
            <Show
              when={!props.question.isAnswering}
              fallback={
                <span class="flex items-center justify-center gap-1.5">
                  <svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                  Answering...
                </span>
              }
            >
              <span class="flex items-center justify-center gap-1.5">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                Submit Answer
              </span>
            </Show>
          </button>
          <button
            type="button"
            onClick={props.onSkip}
            disabled={props.question.isAnswering}
            class="flex-1 px-3 py-2 text-xs font-semibold bg-slate-100 hover:bg-slate-200 dark:bg-slate-700 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 rounded-md transition-colors disabled:opacity-50"
          >
            <span class="flex items-center justify-center gap-1.5">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
              Skip
            </span>
          </button>
        </div>
      </div>
    </div>
  );
};
