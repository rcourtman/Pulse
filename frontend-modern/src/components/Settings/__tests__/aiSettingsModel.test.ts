import { describe, expect, it } from 'vitest';
import { AI_SETUP_PROVIDER_OPTIONS } from '../aiSettingsModel';
import { PROVIDER_DESCRIPTIONS } from '@/types/ai';

describe('aiSettingsModel', () => {
  it('presents DeepSeek as the current V4 provider family', () => {
    const deepseekOption = AI_SETUP_PROVIDER_OPTIONS.find((option) => option.value === 'deepseek');

    expect(deepseekOption?.description).toBe('V4');
    expect(PROVIDER_DESCRIPTIONS.deepseek).toBe('DeepSeek V4 models');
  });
});
