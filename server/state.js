let state = {
  nodes: [],
  vms: [],
  containers: [],
  metrics: [],
  pbs: [], // Array to hold data for each PBS instance
};

function getState() {
  return { ...state }; // Return a shallow copy to prevent direct modification
}

function updateDiscoveryData({ nodes, vms, containers, pbs }) {
  state.nodes = nodes || [];
  state.vms = vms || [];
  state.containers = containers || [];
  state.pbs = pbs || [];
  // Keep existing metrics when discovery runs
}

function updateMetricsData(metrics) {
  state.metrics = metrics || [];
}

function clearMetricsData() {
  state.metrics = [];
}

function hasData() {
    return state.nodes.length > 0 || state.vms.length > 0 || state.containers.length > 0 || state.pbs.length > 0;
}

module.exports = {
  getState,
  updateDiscoveryData,
  updateMetricsData,
  clearMetricsData,
  hasData,
}; 