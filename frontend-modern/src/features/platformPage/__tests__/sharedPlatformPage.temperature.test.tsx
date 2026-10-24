import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { temperatureStore } from '@/utils/temperature';
import { PlatformTableTemperatureValue } from '../sharedPlatformPage';

afterEach(() => {
  cleanup();
  temperatureStore.setUnit('celsius');
});

describe('PlatformTableTemperatureValue', () => {
  it('uses the live preference with one decimal place', async () => {
    temperatureStore.setUnit('fahrenheit');
    render(() => <PlatformTableTemperatureValue value={37.5} />);
    expect(screen.getByText('99.5°F')).toBeInTheDocument();
    temperatureStore.setUnit('celsius');
    expect(await screen.findByText('37.5°C')).toBeInTheDocument();
  });

  it.each([null, undefined, NaN, Infinity, -Infinity, 0, -1])(
    'keeps the existing absent/sentinel reading %s unknown in either unit',
    (value) => {
      temperatureStore.setUnit('celsius');
      render(() => <PlatformTableTemperatureValue value={value} emptyText="Not reported" />);
      expect(screen.getByText('Not reported')).toBeInTheDocument();
      temperatureStore.setUnit('fahrenheit');
      expect(screen.getByText('Not reported')).toBeInTheDocument();
      expect(screen.queryByText(/°[CF]/)).not.toBeInTheDocument();
    },
  );
});
