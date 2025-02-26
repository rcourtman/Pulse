import { useState, useEffect } from 'react';
import { FaMemory, FaMicrochip, FaNetworkWired } from 'react-icons/fa';

const FilterControls = ({ thresholds, onThresholdChange }) => {
  const [localThresholds, setLocalThresholds] = useState(thresholds);
  
  // Update local thresholds when props change
  useEffect(() => {
    setLocalThresholds(thresholds);
  }, [thresholds]);
  
  // Handle slider change
  const handleSliderChange = (metric, value) => {
    setLocalThresholds((prev) => ({ ...prev, [metric]: value }));
    onThresholdChange(metric, value);
  };
  
  return (
    <div className="bg-white rounded-lg shadow-md p-4 mb-6">
      <h2 className="text-lg font-semibold mb-4">Filter by Threshold</h2>
      
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* CPU Threshold */}
        <div>
          <div className="flex justify-between items-center mb-2">
            <label htmlFor="cpu-threshold" className="flex items-center text-sm font-medium text-gray-700">
              <FaMicrochip className="mr-2 text-red-500" />
              CPU Usage
            </label>
            <span className="text-sm font-medium">
              {localThresholds.cpu}%
            </span>
          </div>
          <input
            id="cpu-threshold"
            type="range"
            min="0"
            max="100"
            value={localThresholds.cpu}
            onChange={(e) => handleSliderChange('cpu', parseInt(e.target.value, 10))}
            className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
          />
          <div className="flex justify-between text-xs text-gray-500 mt-1">
            <span>0%</span>
            <span>50%</span>
            <span>100%</span>
          </div>
        </div>
        
        {/* Memory Threshold */}
        <div>
          <div className="flex justify-between items-center mb-2">
            <label htmlFor="memory-threshold" className="flex items-center text-sm font-medium text-gray-700">
              <FaMemory className="mr-2 text-blue-500" />
              Memory Usage
            </label>
            <span className="text-sm font-medium">
              {localThresholds.memory}%
            </span>
          </div>
          <input
            id="memory-threshold"
            type="range"
            min="0"
            max="100"
            value={localThresholds.memory}
            onChange={(e) => handleSliderChange('memory', parseInt(e.target.value, 10))}
            className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
          />
          <div className="flex justify-between text-xs text-gray-500 mt-1">
            <span>0%</span>
            <span>50%</span>
            <span>100%</span>
          </div>
        </div>
        
        {/* Network Threshold */}
        <div>
          <div className="flex justify-between items-center mb-2">
            <label htmlFor="network-threshold" className="flex items-center text-sm font-medium text-gray-700">
              <FaNetworkWired className="mr-2 text-green-500" />
              Network Usage
            </label>
            <span className="text-sm font-medium">
              {localThresholds.network}%
            </span>
          </div>
          <input
            id="network-threshold"
            type="range"
            min="0"
            max="100"
            value={localThresholds.network}
            onChange={(e) => handleSliderChange('network', parseInt(e.target.value, 10))}
            className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
          />
          <div className="flex justify-between text-xs text-gray-500 mt-1">
            <span>0%</span>
            <span>50%</span>
            <span>100%</span>
          </div>
        </div>
      </div>
      
      <div className="mt-4 text-xs text-gray-500">
        <p>Drag sliders to filter resources by usage. Only resources with usage above the threshold will be shown.</p>
        <p>Press ESC to reset all filters and sorting.</p>
      </div>
    </div>
  );
};

export default FilterControls; 