import type { Alert } from '@/types/api';

// Get alert highlighting styles based on active alerts for a resource
export const getAlertStyles = (
  resourceId: string, 
  activeAlerts: Record<string, Alert>,
  resourceType?: 'guest' | 'node' | 'storage'
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
      rowClass: 'bg-red-50 dark:bg-red-900/20 border-l-4 border-red-500',
      indicatorClass: 'bg-red-500',
      badgeClass: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
      hasAlert: true,
      alertCount,
      severity: 'critical' as const
    };
  }
  
  if (highestSeverity === 'warning') {
    return {
      rowClass: 'bg-orange-50 dark:bg-orange-900/20 border-l-4 border-orange-500',
      indicatorClass: 'bg-orange-500',
      badgeClass: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
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