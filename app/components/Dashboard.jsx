const parseNetworkValue = (value) => {
  console.log(`Parsing network value: ${value} ${typeof value}`);
  
  if (typeof value === 'number') {
    return value;
  }
  
  if (typeof value === 'string') {
    // Try to extract numeric portion from string (e.g. "10 KB/s" -> 10)
    const match = value.match(/^(\d+(\.\d+)?)/);
    if (match) {
      return parseFloat(match[1]);
    }
  }
  
  if (value && typeof value === 'object' && value.rate !== undefined) {
    console.log(`Found rate object: ${JSON.stringify(value)}`);
    return value.rate;
  }
  
  return 0;
};

const formatResource = (resource) => {
  // Debug full resource data for JDownloader
  if (resource.name === 'jdownloader') {
    console.log(`JDownloader formatted resource data: ${JSON.stringify(resource, null, 2)}`);
  }
  
  // Network debug for all resources
  console.log(`Network debug for ${resource.name} at ${new Date().toISOString()}: ${JSON.stringify({
    networkIn: resource.networkIn,
    networkOut: resource.networkOut,
    netin: resource.netin,
    netout: resource.netout,
    _rawNetin: resource._rawNetin,
    _rawNetout: resource._rawNetout,
    cpu: resource.cpu,
    memory: resource.memory
  }, null, 2)}`);

  // Process network data
  let networkIn = 0;
  let networkOut = 0;
  
  if (resource.networkIn !== undefined) {
    networkIn = parseNetworkValue(resource.networkIn);
  } else if (resource.netin !== undefined) {
    networkIn = parseNetworkValue(resource.netin);
  }
  
  if (resource.networkOut !== undefined) {
    networkOut = parseNetworkValue(resource.networkOut);
  } else if (resource.netout !== undefined) {
    networkOut = parseNetworkValue(resource.netout);
  }
  
  console.log(`Network values after processing for ${resource.name}: ${JSON.stringify({
    networkIn,
    networkOut
  }, null, 2)}`);
  
  // Format the resource data
  return {
    id: resource.id,
    name: resource.name,
    type: resource.type,
    status: resource.status,
    cpu: resource.cpu !== undefined ? resource.cpu : 0,
    memory: resource.memory !== undefined ? resource.memory : 0,
    disk: resource.disk !== undefined ? resource.disk : 0,
    networkIn,
    networkOut,
    uptime: resource.uptime,
    node: resource.node
  };
} 