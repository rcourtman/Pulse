#!/usr/bin/env node

/**
 * Runner for metric threshold tests
 * This script runs the dedicated metric threshold filtering tests
 */

import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';
import './metricThresholdTests.js';

console.log('Metric threshold filtering tests completed.'); 