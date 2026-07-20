import { describe, expect, it } from 'vitest';
import {
  BUTTON_BASE_CLASS,
  BUTTON_VARIANT_CLASSES,
  BUTTON_SIZE_CLASSES,
  COPY_VALUE_BUTTON_BASE_CLASS,
  COPY_VALUE_BUTTON_VARIANT_CLASSES,
  COPY_VALUE_BUTTON_SIZE_CLASSES,
  ACTION_ICON_BUTTON_BASE_CLASS,
  ACTION_ICON_BUTTON_TONE_CLASSES,
  ACTION_ICON_BUTTON_SIZE_CLASSES,
  getButtonClass,
  getCopyValueButtonClass,
  getActionIconButtonClass,
} from '@/components/shared/buttonModel';
import type {
  ButtonVariant,
  ButtonSize,
  CopyValueButtonVariant,
  CopyValueButtonSize,
  ActionIconButtonTone,
  ActionIconButtonSize,
} from '@/components/shared/buttonModel';

describe('buttonModel.branchcov2', () => {
  describe('getButtonClass', () => {
    it('uses the default parameter when called with no arguments (variant=secondary, size=md)', () => {
      // Drives the `options: ButtonClassOptions = {}` default-parameter arm
      // and both `??` right arms (variant -> 'secondary', size -> 'md').
      expect(getButtonClass()).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES.secondary, BUTTON_SIZE_CLASSES.md].join(' '),
      );
    });

    it('uses the ?? right arms for variant and size when called with an empty options object', () => {
      expect(getButtonClass({})).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES.secondary, BUTTON_SIZE_CLASSES.md].join(' '),
      );
    });

    it('selects the explicit variant and size when provided (?? left arms)', () => {
      expect(getButtonClass({ variant: 'primary', size: 'lg' })).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES.primary, BUTTON_SIZE_CLASSES.lg].join(' '),
      );
    });

    it.each<ButtonVariant>([
      'primary',
      'primaryFlat',
      'warning',
      'warningSolid',
      'info',
      'success',
      'successOutline',
      'successGhost',
      'secondary',
      'danger',
      'dangerOutline',
      'dangerGhost',
      'ghost',
      'outline',
    ])('resolves the %s variant from BUTTON_VARIANT_CLASSES', (variant) => {
      expect(getButtonClass({ variant })).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES[variant], BUTTON_SIZE_CLASSES.md].join(' '),
      );
    });

    it.each<ButtonSize>([
      'xs',
      'sm',
      'mdCompact',
      'settingsAction',
      'settingsActionXs',
      'md',
      'lg',
      'icon',
      'iconMd',
    ])('resolves the %s size from BUTTON_SIZE_CLASSES', (size) => {
      expect(getButtonClass({ size })).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES.secondary, BUTTON_SIZE_CLASSES[size]].join(' '),
      );
    });

    it('appends options.class verbatim when it is a non-empty string', () => {
      expect(getButtonClass({ class: 'mt-4 w-full' })).toBe(
        [
          BUTTON_BASE_CLASS,
          BUTTON_VARIANT_CLASSES.secondary,
          BUTTON_SIZE_CLASSES.md,
          'mt-4 w-full',
        ].join(' '),
      );
    });

    it('drops an empty-string options.class via .filter(Boolean)', () => {
      // Defensive branch: empty string is falsy and must not contribute a
      // trailing space to the joined output.
      expect(getButtonClass({ class: '' })).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES.secondary, BUTTON_SIZE_CLASSES.md].join(' '),
      );
    });

    it('drops an unknown variant lookup (undefined) via .filter(Boolean)', () => {
      // Defensive branch: a malformed variant key resolves to undefined in the
      // Record lookup at runtime; filter(Boolean) must remove it so no stray
      // 'undefined' token or double space appears.
      const bogus = 'not-a-real-variant' as unknown as ButtonVariant;
      expect(getButtonClass({ variant: bogus })).toBe(
        [BUTTON_BASE_CLASS, BUTTON_SIZE_CLASSES.md].join(' '),
      );
    });

    it('drops an unknown size lookup (undefined) via .filter(Boolean)', () => {
      const bogus = 'not-a-real-size' as unknown as ButtonSize;
      expect(getButtonClass({ size: bogus })).toBe(
        [BUTTON_BASE_CLASS, BUTTON_VARIANT_CLASSES.secondary].join(' '),
      );
    });

    it('combines an explicit variant, size, and class together', () => {
      expect(getButtonClass({ variant: 'danger', size: 'icon', class: 'rounded-full' })).toBe(
        [
          BUTTON_BASE_CLASS,
          BUTTON_VARIANT_CLASSES.danger,
          BUTTON_SIZE_CLASSES.icon,
          'rounded-full',
        ].join(' '),
      );
    });
  });

  describe('getCopyValueButtonClass', () => {
    it('uses the default parameter when called with no arguments (variant=neutral, size=md)', () => {
      // `options = {}` default-parameter arm + both `??` right arms.
      expect(getCopyValueButtonClass()).toBe(
        [
          COPY_VALUE_BUTTON_BASE_CLASS,
          COPY_VALUE_BUTTON_VARIANT_CLASSES.neutral,
          COPY_VALUE_BUTTON_SIZE_CLASSES.md,
        ].join(' '),
      );
    });

    it('uses the ?? right arms when called with an empty options object', () => {
      expect(getCopyValueButtonClass({})).toBe(
        [
          COPY_VALUE_BUTTON_BASE_CLASS,
          COPY_VALUE_BUTTON_VARIANT_CLASSES.neutral,
          COPY_VALUE_BUTTON_SIZE_CLASSES.md,
        ].join(' '),
      );
    });

    it('selects the explicit variant and size when provided (?? left arms)', () => {
      expect(getCopyValueButtonClass({ variant: 'accent', size: 'sm' })).toBe(
        [
          COPY_VALUE_BUTTON_BASE_CLASS,
          COPY_VALUE_BUTTON_VARIANT_CLASSES.accent,
          COPY_VALUE_BUTTON_SIZE_CLASSES.sm,
        ].join(' '),
      );
    });

    it.each<CopyValueButtonVariant>(['neutral', 'ghost', 'accent', 'chip'])(
      'resolves the %s variant from COPY_VALUE_BUTTON_VARIANT_CLASSES',
      (variant) => {
        expect(getCopyValueButtonClass({ variant })).toBe(
          [
            COPY_VALUE_BUTTON_BASE_CLASS,
            COPY_VALUE_BUTTON_VARIANT_CLASSES[variant],
            COPY_VALUE_BUTTON_SIZE_CLASSES.md,
          ].join(' '),
        );
      },
    );

    it.each<CopyValueButtonSize>(['xs', 'sm', 'md', 'lg', 'chip'])(
      'resolves the %s size from COPY_VALUE_BUTTON_SIZE_CLASSES',
      (size) => {
        expect(getCopyValueButtonClass({ size })).toBe(
          [
            COPY_VALUE_BUTTON_BASE_CLASS,
            COPY_VALUE_BUTTON_VARIANT_CLASSES.neutral,
            COPY_VALUE_BUTTON_SIZE_CLASSES[size],
          ].join(' '),
        );
      },
    );

    it('appends options.class verbatim when it is a non-empty string', () => {
      expect(getCopyValueButtonClass({ class: 'ml-1' })).toBe(
        [
          COPY_VALUE_BUTTON_BASE_CLASS,
          COPY_VALUE_BUTTON_VARIANT_CLASSES.neutral,
          COPY_VALUE_BUTTON_SIZE_CLASSES.md,
          'ml-1',
        ].join(' '),
      );
    });

    it('drops an empty-string options.class via .filter(Boolean)', () => {
      expect(getCopyValueButtonClass({ class: '' })).toBe(
        [
          COPY_VALUE_BUTTON_BASE_CLASS,
          COPY_VALUE_BUTTON_VARIANT_CLASSES.neutral,
          COPY_VALUE_BUTTON_SIZE_CLASSES.md,
        ].join(' '),
      );
    });

    it('drops an unknown variant lookup (undefined) via .filter(Boolean)', () => {
      const bogus = 'nope' as unknown as CopyValueButtonVariant;
      expect(getCopyValueButtonClass({ variant: bogus })).toBe(
        [COPY_VALUE_BUTTON_BASE_CLASS, COPY_VALUE_BUTTON_SIZE_CLASSES.md].join(' '),
      );
    });

    it('drops an unknown size lookup (undefined) via .filter(Boolean)', () => {
      const bogus = 'xl' as unknown as CopyValueButtonSize;
      expect(getCopyValueButtonClass({ size: bogus })).toBe(
        [COPY_VALUE_BUTTON_BASE_CLASS, COPY_VALUE_BUTTON_VARIANT_CLASSES.neutral].join(' '),
      );
    });

    it('combines an explicit variant, size, and class together', () => {
      expect(getCopyValueButtonClass({ variant: 'chip', size: 'chip', class: 'tag-cls' })).toBe(
        [
          COPY_VALUE_BUTTON_BASE_CLASS,
          COPY_VALUE_BUTTON_VARIANT_CLASSES.chip,
          COPY_VALUE_BUTTON_SIZE_CLASSES.chip,
          'tag-cls',
        ].join(' '),
      );
    });
  });

  describe('getActionIconButtonClass', () => {
    it('uses the default parameter when called with no arguments (tone=neutral, size=sm)', () => {
      // `options = {}` default-parameter arm + both `??` right arms
      // (tone -> 'neutral', size -> 'sm').
      expect(getActionIconButtonClass()).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES.neutral,
          ACTION_ICON_BUTTON_SIZE_CLASSES.sm,
        ].join(' '),
      );
    });

    it('uses the ?? right arms when called with an empty options object', () => {
      expect(getActionIconButtonClass({})).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES.neutral,
          ACTION_ICON_BUTTON_SIZE_CLASSES.sm,
        ].join(' '),
      );
    });

    it('selects the explicit tone and size when provided (?? left arms)', () => {
      expect(getActionIconButtonClass({ tone: 'primary', size: 'md' })).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES.primary,
          ACTION_ICON_BUTTON_SIZE_CLASSES.md,
        ].join(' '),
      );
    });

    it.each<ActionIconButtonTone>([
      'neutral',
      'muted',
      'outline',
      'outlineSelected',
      'primary',
      'accent',
      'accentGhost',
      'success',
      'warningGhost',
      'warningOutline',
      'infoGhost',
      'danger',
    ])('resolves the %s tone from ACTION_ICON_BUTTON_TONE_CLASSES', (tone) => {
      expect(getActionIconButtonClass({ tone })).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES[tone],
          ACTION_ICON_BUTTON_SIZE_CLASSES.sm,
        ].join(' '),
      );
    });

    it.each<ActionIconButtonSize>(['2xs', 'xs', 'sm', 'md', 'lg'])(
      'resolves the %s size from ACTION_ICON_BUTTON_SIZE_CLASSES',
      (size) => {
        expect(getActionIconButtonClass({ size })).toBe(
          [
            ACTION_ICON_BUTTON_BASE_CLASS,
            ACTION_ICON_BUTTON_TONE_CLASSES.neutral,
            ACTION_ICON_BUTTON_SIZE_CLASSES[size],
          ].join(' '),
        );
      },
    );

    it('appends options.class verbatim when it is a non-empty string', () => {
      expect(getActionIconButtonClass({ class: 'self-end' })).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES.neutral,
          ACTION_ICON_BUTTON_SIZE_CLASSES.sm,
          'self-end',
        ].join(' '),
      );
    });

    it('drops an empty-string options.class via .filter(Boolean)', () => {
      expect(getActionIconButtonClass({ class: '' })).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES.neutral,
          ACTION_ICON_BUTTON_SIZE_CLASSES.sm,
        ].join(' '),
      );
    });

    it('drops an unknown tone lookup (undefined) via .filter(Boolean)', () => {
      const bogus = 'hyper' as unknown as ActionIconButtonTone;
      expect(getActionIconButtonClass({ tone: bogus })).toBe(
        [ACTION_ICON_BUTTON_BASE_CLASS, ACTION_ICON_BUTTON_SIZE_CLASSES.sm].join(' '),
      );
    });

    it('drops an unknown size lookup (undefined) via .filter(Boolean)', () => {
      const bogus = '3xl' as unknown as ActionIconButtonSize;
      expect(getActionIconButtonClass({ size: bogus })).toBe(
        [ACTION_ICON_BUTTON_BASE_CLASS, ACTION_ICON_BUTTON_TONE_CLASSES.neutral].join(' '),
      );
    });

    it('combines an explicit tone, size, and class together', () => {
      expect(getActionIconButtonClass({ tone: 'danger', size: 'xs', class: 'ring-2' })).toBe(
        [
          ACTION_ICON_BUTTON_BASE_CLASS,
          ACTION_ICON_BUTTON_TONE_CLASSES.danger,
          ACTION_ICON_BUTTON_SIZE_CLASSES.xs,
          'ring-2',
        ].join(' '),
      );
    });
  });
});
