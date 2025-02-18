import React from 'react';
import { Gauge } from "lucide-react";
import { cn } from "../../lib/utils";
import ContainerRowBase from './ContainerRowBase';
import { useContainerStore } from '../../stores/containerStore';

// Utility functions
const formatNetworkRate = (rateInMB) => {
  if (rateInMB >= 1) {
    return `${Math.round(rateInMB)} MB/s`;
  } else {
    const rateInKB = rateInMB * 1024; // Convert MB to KB
    return `${Math.round(rateInKB)} KB/s`;
  }
};

const getProgressBarColor = (value, type = 'default') => {
  if (type === 'network-up') {
    if (value >= 90) return 'bg-purple-500';
    if (value >= 75) return 'bg-purple-400';
    return 'bg-purple-300';
  } else if (type === 'network-down') {
    if (value >= 90) return 'bg-blue-500';
    if (value >= 75) return 'bg-blue-400';
    return 'bg-blue-300';
  } else {
    if (value >= 90) return 'bg-red-500';
    if (value >= 75) return 'bg-yellow-500';
    return 'bg-green-500';
  }
};

// Utility function to highlight search terms in text
const highlightSearchTerms = (text, searchInput, searchTerms) => {
  const terms = searchInput ? [searchInput, ...searchTerms] : searchTerms;
  if (terms.length === 0) return text;

  const parts = [];
  let lastIndex = 0;
  const lowerText = text.toLowerCase();

  terms.forEach((term, termIndex) => {
    const lowerTerm = term.toLowerCase();
    let index = lowerText.indexOf(lowerTerm, lastIndex);
    
    while (index !== -1) {
      // Add non-matching text before the match
      if (index > lastIndex) {
        parts.push({
          text: text.slice(lastIndex, index),
          isMatch: false,
          key: `${lastIndex}-${index}-${termIndex}`
        });
      }
      
      // Add the matching text
      parts.push({
        text: text.slice(index, index + term.length),
        isMatch: true,
        key: `match-${index}-${termIndex}`
      });
      
      lastIndex = index + term.length;
      index = lowerText.indexOf(lowerTerm, lastIndex);
    }
  });

  // Add any remaining text
  if (lastIndex < text.length) {
    parts.push({
      text: text.slice(lastIndex),
      isMatch: false,
      key: `end-${lastIndex}`
    });
  }

  return parts.map(part => (
    <span
      key={part.key}
      className={part.isMatch ? 'font-bold transition duration-150' : 'text-gray-400'}
    >
      {part.text}
    </span>
  ));
};

const ContainerRow = React.memo(({ container, getAlertScore, compact, searchInput }) => {
  const isRunning = container.status === 'running';
  const isAlerted = getAlertScore(container) > 0;
  const { searchTerms } = useContainerStore();

  const nameColor = isRunning ? 'text-gray-200' : 'text-gray-500';
  
  // Metric color calculation
  const getMetricColor = (value) => {
    return isRunning ? 'text-white' : 'text-gray-500';
  };

  const cpuColor = getMetricColor(container.cpu);
  const memColor = getMetricColor(container.memory);
  const diskColor = getMetricColor(container.disk);
  const netInColor = getMetricColor(container.networkIn);
  const netOutColor = getMetricColor(container.networkOut);

  // Use both searchInput and searchTerms for highlighting
  const highlightedName = highlightSearchTerms(container.name, searchInput, searchTerms);

  return (
    <ContainerRowBase compact={compact}>
      {/* Container Name and Mobile Metrics */}
      <div className="grid grid-cols-[1fr_auto] sm:grid-cols-1 items-center gap-2 col-span-2 sm:col-span-1 w-full">
        <div className="flex items-center gap-2 min-w-0">
          <div className={`w-2 h-2 rounded-full flex-shrink-0 ${isRunning ? 'bg-green-500' : 'bg-gray-500'}`} />
          <span 
            title={container.ip ? `IP: ${container.ip}` : ''} 
            className={`${nameColor} truncate`}
          >
            {highlightedName}
          </span>
        </div>
        
        {/* Mobile metrics summary */}
        <div className="flex sm:hidden items-center gap-3">
          <div className="flex flex-col items-end gap-0.5">
            <div className="flex gap-3">
              <span className={`text-xs ${cpuColor}`}>CPU: {container.cpu.toFixed(1)}%</span>
              <span className={`text-xs ${memColor}`}>Mem: {container.memory}%</span>
            </div>
            <div className="flex gap-3">
              <span className={`text-xs ${diskColor}`}>Disk: {container.disk}%</span>
              <span className={`text-xs ${netInColor}`}>Net: {formatNetworkRate(container.networkIn + container.networkOut)}</span>
            </div>
          </div>
          {isAlerted && (
            <div className="p-1.5 bg-red-500/20 rounded-lg">
              <Gauge className="w-4 h-4 text-red-500" />
            </div>
          )}
        </div>
      </div>
    
      {/* Desktop metrics */}
      <div className="hidden sm:block">
        <ContainerRowBase.MetricCell>
          <div className="flex items-center gap-1">
            <span className={`w-12 ${cpuColor}`}>{container.cpu.toFixed(1)}%</span>
          </div>
          <div className="flex-1 bg-gray-700 rounded-full h-2 relative">
            <div
              className={`${getProgressBarColor(container.cpu)} h-full rounded-full transition-all duration-300`}
              style={{ width: `${Math.min(container.cpu, 100)}%` }}
            />
          </div>
        </ContainerRowBase.MetricCell>
      </div>
    
      <div className="hidden sm:block">
        <ContainerRowBase.MetricCell>
          <div className="flex items-center gap-1">
            <span className={`w-12 ${memColor}`}>{container.memory}%</span>
          </div>
          <div className="flex-1 bg-gray-700 rounded-full h-2 relative">
            <div
              className={`${getProgressBarColor(container.memory)} h-full rounded-full transition-all duration-300`}
              style={{ width: `${Math.min(container.memory, 100)}%` }}
            />
          </div>
        </ContainerRowBase.MetricCell>
      </div>
    
      <div className="hidden sm:block">
        <ContainerRowBase.MetricCell>
          <div className="flex items-center gap-1">
            <span className={`w-12 ${diskColor}`}>{container.disk}%</span>
          </div>
          <div className="flex-1 bg-gray-700 rounded-full h-2 relative">
            <div
              className={`${getProgressBarColor(container.disk)} h-full rounded-full transition-all duration-300`}
              style={{ width: `${Math.min(container.disk, 100)}%` }}
            />
          </div>
        </ContainerRowBase.MetricCell>
      </div>
    
      <div className="hidden sm:block">
        <ContainerRowBase.MetricCell>
          <div className="flex flex-col w-full gap-1">
            <div className="flex items-center gap-1">
              <span className={`w-20 text-xs ${netOutColor}`}>
                ↑ {formatNetworkRate(container.networkOut)}
              </span>
              <div className="flex-1 bg-gray-700 rounded-full h-1.5">
                <div
                  className={`${getProgressBarColor(container.networkOut, 'network-up')} h-full rounded-full transition-all duration-300`}
                  style={{ width: `${Math.min(container.networkOut, 100)}%` }}
                />
              </div>
            </div>
            <div className="flex items-center gap-1">
              <span className={`w-20 text-xs ${netInColor}`}>
                ↓ {formatNetworkRate(container.networkIn)}
              </span>
              <div className="flex-1 bg-gray-700 rounded-full h-1.5">
                <div
                  className={`${getProgressBarColor(container.networkIn, 'network-down')} h-full rounded-full transition-all duration-300`}
                  style={{ width: `${Math.min(container.networkIn, 100)}%` }}
                />
              </div>
            </div>
          </div>
        </ContainerRowBase.MetricCell>
      </div>
    
      {/* Desktop alert indicator */}
      <div className="hidden sm:flex items-center justify-end">
        {isAlerted && (
          <div className="p-1.5 bg-red-500/20 rounded-lg">
            <Gauge className="w-4 h-4 text-red-500" />
          </div>
        )}
      </div>
    </ContainerRowBase>
  );
});

export default ContainerRow;