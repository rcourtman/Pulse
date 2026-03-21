import { cleanup, renderHook } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { useThresholdSliderState } from '@/components/Dashboard/useThresholdSliderState';

vi.mock('@/utils/temperature', () => ({
  formatTemperature: (value: number) => `${value}°C`,
  getTemperatureSymbol: () => '°C',
}));

describe('useThresholdSliderState', () => {
  it('centralizes threshold slider derivations and drag cleanup', () => {
    const onChange = vi.fn();
    const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    const addWindowSpy = vi.spyOn(window, 'addEventListener');
    const removeWindowSpy = vi.spyOn(window, 'removeEventListener');
    const addDocumentSpy = vi.spyOn(document, 'addEventListener');
    const removeDocumentSpy = vi.spyOn(document, 'removeEventListener');

    const { result } = renderHook(() =>
      useThresholdSliderState({
        value: 45,
        onChange,
        type: 'temperature',
      }),
    );

    expect(result.thumbPosition()).toBe(45);
    expect(result.thumbTransform()).toBe('translateY(-50%) translateX(-50%)');
    expect(result.sliderLabel()).toBe('45°C');
    expect(result.sliderTitle()).toBe('Temperature: 45°C');
    expect(result.isDragging()).toBe(false);

    result.handleInput({
      currentTarget: { value: '75' },
    } as unknown as InputEvent & { currentTarget: HTMLInputElement; target: HTMLInputElement });
    expect(onChange).toHaveBeenCalledWith(75);

    result.handleMouseDown?.();
    expect(result.isDragging()).toBe(true);
    expect(addWindowSpy).toHaveBeenCalledWith('scroll', expect.any(Function), { capture: true });
    expect(addDocumentSpy).toHaveBeenCalledWith('mouseup', expect.any(Function));

    const scrollHandler = addWindowSpy.mock.calls.find((call) => call[0] === 'scroll')?.[1] as
      | EventListener
      | undefined;
    expect(scrollHandler).toBeTypeOf('function');
    scrollHandler?.(new Event('scroll'));
    expect(scrollToSpy).toHaveBeenCalled();

    cleanup();

    expect(removeWindowSpy).toHaveBeenCalledWith('scroll', expect.any(Function), {
      capture: true,
    });
    expect(removeDocumentSpy).toHaveBeenCalledWith('mouseup', expect.any(Function));
  });
});
