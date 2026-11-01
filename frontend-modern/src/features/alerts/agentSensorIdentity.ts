const AGENT_SENSOR_RESOURCE_ID_PATTERN = /^agent:(.+)\/sensor:((?:[0-9a-f]{2})+)$/;

// The alert resource ID of one native sensor, as the backend builds it
// (internal/alerts/host_native_sensor.go HostNativeSensorResourceID):
// "agent:<host ID>/sensor:<lower-case hex of the sensor ID's UTF-8 bytes>".
// The hex form keeps sensor IDs that differ only by case or punctuation apart.
export const agentSensorResourceId = (hostId: string, sensorId: string): string => {
  const hex = Array.from(new TextEncoder().encode(sensorId), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('');
  return `agent:${hostId.trim() || 'unknown'}/sensor:${hex}`;
};

// The host ID and sensor ID an agentSensorResourceId encodes, or null when
// the ID is not a native sensor resource.
export const parseAgentSensorResourceId = (
  resourceId: string,
): { hostId: string; sensorId: string } | null => {
  const match = resourceId.match(AGENT_SENSOR_RESOURCE_ID_PATTERN);
  if (!match) return null;
  const bytes = new Uint8Array(match[2].length / 2);
  for (let index = 0; index < bytes.length; index += 1) {
    bytes[index] = parseInt(match[2].slice(index * 2, index * 2 + 2), 16);
  }
  try {
    return {
      hostId: match[1],
      sensorId: new TextDecoder('utf-8', { fatal: true }).decode(bytes),
    };
  } catch {
    return null;
  }
};
