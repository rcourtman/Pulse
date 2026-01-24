/**
 * Utility functions for formatting alert values based on their metric type.
 * Temperature metrics show the user-selected unit (°C or °F).
 */

import { getTemperatureSymbol } from './temperature';

/**
 * Metric types that are measured in degrees (temperature) rather than percentages.
 */
const TEMPERATURE_METRIC_TYPES = new Set(['temperature', 'temp']);

/**
 * Metric types that are measured in MB/s rather than percentages.
 */
const THROUGHPUT_METRIC_TYPES = new Set(['diskRead', 'diskWrite', 'networkIn', 'networkOut']);

/**
 * Returns the appropriate unit suffix for a given metric type.
 * @param metricType The alert's metric type (e.g., 'cpu', 'memory', 'temperature')
 * @returns The unit suffix to append to values (e.g., '%', '°C', '°F', ' MB/s')
 */
export function getAlertUnit(metricType?: string): string {
    if (!metricType) return '%';
    const typeLower = metricType.toLowerCase();

    if (TEMPERATURE_METRIC_TYPES.has(typeLower)) {
        return getTemperatureSymbol();
    }

    if (THROUGHPUT_METRIC_TYPES.has(typeLower)) {
        return ' MB/s';
    }

    return '%';
}

/**
 * Formats an alert value with the appropriate unit based on metric type.
 * @param value The numeric value to format
 * @param metricType The alert's metric type (e.g., 'cpu', 'memory', 'temperature')
 * @param decimals Number of decimal places (default: 1)
 * @returns Formatted string with appropriate unit (e.g., '82.5%', '74.0°C')
 */
export function formatAlertValue(
    value: number | undefined,
    metricType?: string,
    decimals = 1
): string {
    if (value === undefined || !Number.isFinite(value)) {
        return 'N/A';
    }
    return `${value.toFixed(decimals)}${getAlertUnit(metricType)}`;
}

/**
 * Formats a threshold value with the appropriate unit based on metric type.
 * Returns 'Disabled' for values <= 0 and 'Not configured' for undefined values.
 * @param value The threshold value to format
 * @param metricType The alert's metric type (e.g., 'cpu', 'memory', 'temperature')
 * @returns Formatted threshold string
 */
export function formatAlertThreshold(
    value: number | undefined,
    metricType?: string
): string {
    if (value === undefined || Number.isNaN(value)) {
        return 'Not configured';
    }
    if (value <= 0) {
        return 'Disabled';
    }
    return `${value}${getAlertUnit(metricType)}`;
}

