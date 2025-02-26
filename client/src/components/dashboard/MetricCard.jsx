import React from 'react';
import ResourceBar from './ResourceBar';

const MetricCard = ({ title, value, type, threshold, onThresholdChange }) => {
  // Format the value to 1 decimal place
  const formattedValue = typeof value === 'number' ? value.toFixed(1) : '0.0';
  
  // Determine the severity class based on value
  const getSeverityClass = () => {
    if (value >= 90) {
      return 'text-red-600';
    } else if (value >= 70) {
      return 'text-orange-600';
    } else if (value >= 50) {
      return 'text-yellow-600';
    }
    return 'text-gray-600';
  };
  
  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold text-gray-800">{title}</h3>
        <div className={`text-xl font-bold ${getSeverityClass()}`}>
          {formattedValue}%
        </div>
      </div>
      
      <ResourceBar 
        value={value || 0} 
        type={type} 
        label={`${formattedValue}%`} 
      />
      
      <div className="mt-4">
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Alert Threshold: {threshold}%
        </label>
        <input
          type="range"
          min="0"
          max="100"
          value={threshold}
          onChange={(e) => onThresholdChange(parseInt(e.target.value, 10))}
          className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer"
        />
      </div>
    </div>
  );
};

export default MetricCard; 