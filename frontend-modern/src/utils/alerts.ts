import type { Alert } from '@/types/api';

// Get alert highlighting styles based on active alerts for a resource
export const getAlertStyles = (
  resourceId: string, 
  activeAlerts: Record<string, Alert>
) => {
  // Find the highest severity alert for this resource
  let highestSeverity: 'critical' | 'warning' | null = null;
  let alertCount = 0;
  
  Object.values(activeAlerts).forEach(alert => {
    if (alert.resourceId === resourceId) {
      alertCount++;
      if (alert.level === 'critical' || (alert.level === 'warning' && highestSeverity !== 'critical')) {
        highestSeverity = alert.level;
      }
    }
  });
  
  // Return appropriate styling based on alert severity
  if (highestSeverity === 'critical') {
    return {
      rowClass: 'bg-red-50 dark:bg-red-950/30 border-l-4 border-red-500 dark:border-red-400',
      indicatorClass: 'bg-red-500',
      badgeClass: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
      hasAlert: true,
      alertCount,
      severity: 'critical' as const
    };
  }
  
  if (highestSeverity === 'warning') {
    return {
      rowClass: 'bg-yellow-50 dark:bg-yellow-950/20 border-l-4 border-yellow-500 dark:border-yellow-400',
      indicatorClass: 'bg-yellow-500',
      badgeClass: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
      hasAlert: true,
      alertCount,
      severity: 'warning' as const
    };
  }
  
  return {
    rowClass: '',
    indicatorClass: '',
    badgeClass: '',
    hasAlert: false,
    alertCount: 0,
    severity: null
  };
};

// Get alert messages for a specific resource
export const getResourceAlerts = (
  resourceId: string,
  activeAlerts: Record<string, Alert>
): Alert[] => {
  return Object.values(activeAlerts).filter(alert => alert.resourceId === resourceId);
};