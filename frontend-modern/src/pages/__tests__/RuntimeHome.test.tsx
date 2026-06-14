import { render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import RuntimeHome from '@/pages/RuntimeHome';

describe('RuntimeHome', () => {
  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('defers workspace routing to the authenticated app shell', () => {
    render(() => <RuntimeHome />);

    expect(screen.getByText('Opening workspace...')).toBeInTheDocument();
  });

  it('renders the authenticated workspace handoff through the active locale', () => {
    setActiveLocale('es');

    render(() => <RuntimeHome />);

    expect(screen.getByText('Abriendo el espacio de trabajo...')).toBeInTheDocument();
  });
});
