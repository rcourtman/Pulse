import { useState, useEffect } from 'react';

/**
 * Custom hook to provide mock metrics data for testing
 * @returns {Object} The mock metrics data
 */
const useMockMetrics = (guestData) => {
  const [mockMetrics, setMockMetrics] = useState({
    cpu: {},
    memory: {},
    disk: {},
    network: {}
  });

  // Keep track of trends for more realistic changes
  const [trends, setTrends] = useState({});

  // Generate initial mock metrics
  useEffect(() => {
    if (!guestData || guestData.length === 0) return;
    
    const newMockMetrics = {
      cpu: {},
      memory: {},
      disk: {},
      network: {}
    };
    
    const initialTrends = {};
    
    guestData.forEach(guest => {
      // Generate random CPU usage between 5% and 80%
      newMockMetrics.cpu[guest.id] = {
        usage: Math.floor(Math.random() * 75) + 5
      };
      
      // Generate random memory usage between 10% and 90%
      const memoryPercent = Math.floor(Math.random() * 80) + 10;
      newMockMetrics.memory[guest.id] = {
        used: Math.floor(Math.random() * 8 * 1024 * 1024 * 1024) + 512 * 1024 * 1024, // 512MB to 8.5GB
        total: 16 * 1024 * 1024 * 1024, // 16GB
        usagePercent: memoryPercent
      };
      
      // Generate random disk usage between 20% and 95%
      const diskPercent = Math.floor(Math.random() * 75) + 20;
      newMockMetrics.disk[guest.id] = {
        used: Math.floor(Math.random() * 900 * 1024 * 1024 * 1024) + 100 * 1024 * 1024 * 1024, // 100GB to 1TB
        total: 1024 * 1024 * 1024 * 1024, // 1TB
        usagePercent: diskPercent
      };
      
      // Generate random network rates
      newMockMetrics.network[guest.id] = {
        inRate: Math.floor(Math.random() * 10 * 1024 * 1024) + 1024, // 1KB/s to 10MB/s
        outRate: Math.floor(Math.random() * 5 * 1024 * 1024) + 1024 // 1KB/s to 5MB/s
      };
      
      // Initialize trends
      initialTrends[guest.id] = {
        cpu: Math.random() > 0.5 ? 1 : -1,       // Random initial trend direction
        memory: Math.random() > 0.5 ? 1 : -1,    // Random initial trend direction
        disk: 1,                                 // Disk typically grows
        netIn: 0,                                // No initial trend
        netOut: 0,                               // No initial trend
        spikeCooldown: 0,                        // No initial cooldown
        loadLevel: Math.floor(Math.random() * 3) // 0=light, 1=medium, 2=heavy
      };
    });
    
    setMockMetrics(newMockMetrics);
    setTrends(initialTrends);
  }, [guestData]);
  
  // Update mock metrics periodically with more dynamic changes
  useEffect(() => {
    if (!guestData || guestData.length === 0) return;
    
    const interval = setInterval(() => {
      setMockMetrics(prev => {
        const updated = { ...prev };
        
        setTrends(prevTrends => {
          const updatedTrends = { ...prevTrends };
          
          guestData.forEach(guest => {
            if (!updatedTrends[guest.id]) {
              // Initialize trend data for new guests
              updatedTrends[guest.id] = {
                cpu: Math.random() > 0.5 ? 1 : -1,
                memory: Math.random() > 0.5 ? 1 : -1,
                disk: 1,
                netIn: 0,
                netOut: 0,
                spikeCooldown: 0,
                loadLevel: Math.floor(Math.random() * 3)
              };
            }
            
            const trend = updatedTrends[guest.id];
            
            // Occasionally change trend direction
            if (Math.random() < 0.1) {
              trend.cpu *= -1;
            }
            if (Math.random() < 0.05) {
              trend.memory *= -1;
            }
            
            // Occasionally change load level
            if (Math.random() < 0.05) {
              trend.loadLevel = Math.floor(Math.random() * 3);
            }
            
            // Decrease spike cooldown
            if (trend.spikeCooldown > 0) {
              trend.spikeCooldown--;
            }
            
            // Handle CPU updates with trend-based changes
            const cpuConfig = {
              changeRange: trend.loadLevel === 0 ? 2 : trend.loadLevel === 1 ? 5 : 10,
              minValue: trend.loadLevel === 0 ? 5 : trend.loadLevel === 1 ? 20 : 40,
              maxValue: trend.loadLevel === 0 ? 30 : trend.loadLevel === 1 ? 60 : 95,
              spikeChance: trend.loadLevel === 0 ? 0.02 : trend.loadLevel === 1 ? 0.05 : 0.1
            };
            
            // Update CPU
            if (updated.cpu[guest.id]) {
              let newCpuUsage = updated.cpu[guest.id].usage;
              
              // Check for spikes
              if (trend.spikeCooldown === 0 && Math.random() < cpuConfig.spikeChance) {
                // Create a spike
                newCpuUsage = Math.min(95, newCpuUsage + Math.random() * 40 + 20);
                trend.spikeCooldown = Math.floor(Math.random() * 5) + 3; // 3-7 intervals cooldown
              } else {
                // Normal fluctuation
                const change = (Math.random() * cpuConfig.changeRange * 2 - cpuConfig.changeRange) + 
                              (trend.cpu * cpuConfig.changeRange * 0.5);
                newCpuUsage = Math.max(cpuConfig.minValue, 
                              Math.min(cpuConfig.maxValue, newCpuUsage + change));
              }
              
              updated.cpu[guest.id].usage = newCpuUsage;
            }
            
            // Update Memory with trend-based changes
            if (updated.memory[guest.id]) {
              const memConfig = {
                changePercent: 0.03, // Maximum 3% change per update
                minPercent: 10,
                maxPercent: 90
              };
              
              // Memory often follows CPU with a delay
              const cpuInfluence = trend.cpu > 0 ? 1 : -0.5;
              const change = ((Math.random() * memConfig.changePercent * 2) - memConfig.changePercent + 
                            (trend.memory * memConfig.changePercent * 0.7) + 
                            (cpuInfluence * memConfig.changePercent * 0.3)) * 100;
              
              const newPercent = Math.max(memConfig.minPercent, 
                                 Math.min(memConfig.maxPercent, 
                                        updated.memory[guest.id].usagePercent + change));
              
              const newUsed = updated.memory[guest.id].total * (newPercent / 100);
              
              updated.memory[guest.id].used = newUsed;
              updated.memory[guest.id].usagePercent = newPercent;
            }
            
            // Update Disk with slow growth and occasional cleanups
            if (updated.disk[guest.id]) {
              // Disk usually grows slowly
              let diskChange = (Math.random() * 0.5); // 0-0.5% growth per update
              
              // Occasional disk cleanup (5% chance)
              if (Math.random() < 0.05) {
                diskChange = -1 * (Math.random() * 3 + 1); // 1-4% reduction
              }
              
              const newPercent = Math.max(20, Math.min(95, updated.disk[guest.id].usagePercent + diskChange));
              const newUsed = updated.disk[guest.id].total * (newPercent / 100);
              
              updated.disk[guest.id].used = newUsed;
              updated.disk[guest.id].usagePercent = newPercent;
            }
            
            // Update Network with burst patterns
            if (updated.network[guest.id]) {
              // Define realistic baseline values (in bytes/sec)
              const baselineInRate = 20 * 1024; // 20 KB/s baseline
              const baselineOutRate = 10 * 1024; // 10 KB/s baseline
              
              // Maximum sustainable values (in bytes/sec)
              const maxSustainedInRate = 300 * 1024; // 300 KB/s max sustained
              const maxSustainedOutRate = 150 * 1024; // 150 KB/s max sustained
              
              // Current values
              const currentInRate = updated.network[guest.id].inRate;
              const currentOutRate = updated.network[guest.id].outRate;
              
              // Force strong regression to baseline over time
              // Reset to baseline every ~30 seconds (1 in 20 chance per 1.5 sec interval)
              const needsReset = Math.random() < 0.05;
              
              // Determine if we should generate a burst
              // Reduce burst frequency when rates are already high
              const ratesAreHigh = currentInRate > 100 * 1024 || currentOutRate > 50 * 1024;
              const burstProbability = ratesAreHigh ? 0.05 : 0.1; // Reduce burst chance when already high
              const burstMode = Math.random() < burstProbability;
              
              // Calculate new rates based on current state
              let newInRate, newOutRate;
              
              if (needsReset) {
                // Periodic reset to baseline to prevent long-term growth
                newInRate = baselineInRate + (Math.random() * 10 * 1024); // baseline + small randomness
                newOutRate = baselineOutRate + (Math.random() * 5 * 1024);  // baseline + small randomness
              } else if (burstMode) {
                // Generate a traffic burst
                const burstInSize = Math.random() * 150 * 1024 + 50 * 1024; // 50-200KB/s increase
                const burstOutSize = Math.random() * 100 * 1024 + 30 * 1024; // 30-130KB/s increase
                
                // Apply burst, but cap at max values
                newInRate = Math.min(maxSustainedInRate, currentInRate + burstInSize);
                newOutRate = Math.min(maxSustainedOutRate, currentOutRate + burstOutSize);
              } else {
                // Normal regression toward baseline - stronger the further from baseline
                const inRateDistanceFromBaseline = Math.max(0, currentInRate - baselineInRate);
                const outRateDistanceFromBaseline = Math.max(0, currentOutRate - baselineOutRate);
                
                // Decay rate increases with distance (50-80% decay)
                const inDecayRate = 0.5 + (inRateDistanceFromBaseline / (maxSustainedInRate * 2));
                const outDecayRate = 0.5 + (outRateDistanceFromBaseline / (maxSustainedOutRate * 2));
                
                // Calculate decay amount (strong regression to baseline)
                const inDecayAmount = inRateDistanceFromBaseline * Math.min(0.8, inDecayRate);
                const outDecayAmount = outRateDistanceFromBaseline * Math.min(0.8, outDecayRate);
                
                // Add small random fluctuation (-10KB to +5KB)
                const randomFluctuation = (Math.random() * 15 - 10) * 1024;
                
                // Apply decay with fluctuation, ensuring we don't go below baseline
                newInRate = Math.max(
                  baselineInRate, 
                  currentInRate - inDecayAmount + randomFluctuation
                );
                
                newOutRate = Math.max(
                  baselineOutRate,
                  currentOutRate - outDecayAmount + (randomFluctuation / 2)
                );
              }
              
              // Final safety caps
              newInRate = Math.min(maxSustainedInRate, Math.max(baselineInRate / 2, newInRate));
              newOutRate = Math.min(maxSustainedOutRate, Math.max(baselineOutRate / 2, newOutRate));
              
              // Update the network metrics
              updated.network[guest.id].inRate = newInRate;
              updated.network[guest.id].outRate = newOutRate;
            }
          });
          
          return updatedTrends;
        });
        
        return updated;
      });
    }, 1500); // Update more frequently (every 1.5 seconds instead of 5 seconds)

    return () => clearInterval(interval);
  }, [guestData]);

  return mockMetrics;
};

export default useMockMetrics; 