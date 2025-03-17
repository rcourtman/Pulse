/**
 * Debug file for testing spaced expressions
 */

// Mock metrics data with values specifically for threshold testing
const mockMetricsData = {
  cpu: {
    '101': { usage: 40, cores: 2 },
    '102': { usage: 75, cores: 4 },
    '103': { usage: 25, cores: 1 },
    '104': { usage: 60, cores: 2 },
    '105': { usage: 90, cores: 8 }
  },
  memory: {
    '101': { used: 1024, total: 4096, usagePercent: 25 },
    '102': { used: 6144, total: 8192, usagePercent: 75 },
    '103': { used: 512, total: 1024, usagePercent: 50 },
    '104': { used: 3072, total: 4096, usagePercent: 80 },
    '105': { used: 14336, total: 16384, usagePercent: 90 }
  },
  disk: {
    '101': { used: 10240, total: 51200, usagePercent: 20 },
    '102': { used: 76800, total: 102400, usagePercent: 75 },
    '103': { used: 2560, total: 5120, usagePercent: 50 },
    '104': { used: 40960, total: 51200, usagePercent: 85 },
    '105': { used: 92160, total: 102400, usagePercent: 95 }
  }
};

// Mock guest data with clear names indicating their metrics
const mockGuests = [
  { id: '101', name: 'low-usage', type: 'qemu', status: 'running', node: 'node-1' },
  { id: '102', name: 'medium-usage', type: 'qemu', status: 'running', node: 'node-1' },
  { id: '103', name: 'very-low-usage', type: 'lxc', status: 'running', node: 'node-2' },
  { id: '104', name: 'high-usage', type: 'qemu', status: 'running', node: 'node-2' },
  { id: '105', name: 'very-high-usage', type: 'qemu', status: 'running', node: 'node-3' }
];

// Define the regex patterns
const resourceExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)\s*(>|<|>=|<=|=)\s*(\d+)$/i;
const directExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)(>|<|>=|<=|=)(\d+)$/i;
const spacedExpressionRegex = /^(cpu|mem(ory)?|disk|network|net)\s+([<>]=?|=)\s+(\d+)$/i;

// Function to test if a term matches a guest
function matchesTerm(guest, termLower, metricsData) {
  console.log(`Testing term "${termLower}" for guest ${guest.id} (${guest.name})`);
  
  // Check for spaced expressions like "cpu > 50"
  const spacedMatch = termLower.match(spacedExpressionRegex);
  if (spacedMatch) {
    console.log(`  Matched spacedExpressionRegex: ${JSON.stringify(spacedMatch)}`);
    
    let resource = spacedMatch[1].toLowerCase();
    const memoryCapture = spacedMatch[2]; // Capture the optional (ory) part
    if (resource === 'mem' || (resource === 'mem' && memoryCapture)) {
      resource = 'memory';
    }
    
    const operator = spacedMatch[3];
    const value = parseFloat(spacedMatch[4]);
    
    console.log(`  Resource: ${resource}, Operator: ${operator}, Value: ${value}`);
    
    // Get the metric value
    let metricValue = 0;
    
    if (resource === 'cpu') {
      metricValue = metricsData?.cpu?.[guest.id]?.usage ?? 0;
    } else if (resource === 'memory') {
      metricValue = metricsData?.memory?.[guest.id]?.usagePercent ?? 0;
    } else if (resource === 'disk') {
      metricValue = metricsData?.disk?.[guest.id]?.usagePercent ?? 0;
    }
    
    console.log(`  Metric value: ${metricValue}`);
    
    // Apply operator
    let result = false;
    switch (operator) {
      case '>': result = metricValue > value; break;
      case '<': result = metricValue < value; break;
      case '>=': result = metricValue >= value; break;
      case '<=': result = metricValue <= value; break;
      case '=': result = metricValue === value; break;
    }
    
    console.log(`  Result: ${result}`);
    return result;
  }
  
  // Check for direct expressions like "cpu>50"
  const directMatch = termLower.match(directExpressionRegex);
  if (directMatch) {
    console.log(`  Matched directExpressionRegex: ${JSON.stringify(directMatch)}`);
    
    let resource = directMatch[1].toLowerCase();
    const memoryCapture = directMatch[2]; // Capture the optional (ory) part
    if (resource === 'mem' || (resource === 'mem' && memoryCapture)) {
      resource = 'memory';
    }
    
    const operator = directMatch[3];
    const value = parseFloat(directMatch[4]);
    
    console.log(`  Resource: ${resource}, Operator: ${operator}, Value: ${value}`);
    
    // Get the metric value
    let metricValue = 0;
    
    if (resource === 'cpu') {
      metricValue = metricsData?.cpu?.[guest.id]?.usage ?? 0;
    } else if (resource === 'memory') {
      metricValue = metricsData?.memory?.[guest.id]?.usagePercent ?? 0;
    } else if (resource === 'disk') {
      metricValue = metricsData?.disk?.[guest.id]?.usagePercent ?? 0;
    }
    
    console.log(`  Metric value: ${metricValue}`);
    
    // Apply operator
    let result = false;
    switch (operator) {
      case '>': result = metricValue > value; break;
      case '<': result = metricValue < value; break;
      case '>=': result = metricValue >= value; break;
      case '<=': result = metricValue <= value; break;
      case '=': result = metricValue === value; break;
    }
    
    console.log(`  Result: ${result}`);
    return result;
  }
  
  console.log(`  No match found for term "${termLower}"`);
  return false;
}

// Test the spaced expressions
console.log("\n=== Testing CPU > 50 ===");
const cpuResults = mockGuests.filter(guest => matchesTerm(guest, "cpu > 50", mockMetricsData));
console.log("Guests matching 'cpu > 50':", cpuResults.map(g => g.id));

console.log("\n=== Testing Memory > 50 ===");
const memResults = mockGuests.filter(guest => matchesTerm(guest, "memory > 50", mockMetricsData));
console.log("Guests matching 'memory > 50':", memResults.map(g => g.id));

console.log("\n=== Testing Disk > 50 ===");
const diskResults = mockGuests.filter(guest => matchesTerm(guest, "disk > 50", mockMetricsData));
console.log("Guests matching 'disk > 50':", diskResults.map(g => g.id));

// Test direct expressions for comparison
console.log("\n=== Testing CPU>50 ===");
const cpuDirectResults = mockGuests.filter(guest => matchesTerm(guest, "cpu>50", mockMetricsData));
console.log("Guests matching 'cpu>50':", cpuDirectResults.map(g => g.id)); 