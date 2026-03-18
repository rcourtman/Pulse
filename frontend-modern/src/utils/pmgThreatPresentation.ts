export type PMGThreatTone = 'spam' | 'virus' | 'quarantine';

export interface PMGThreatPresentation {
  barClass: string;
  textClass: string;
}

export function getPMGThreatPresentation(tone: PMGThreatTone): PMGThreatPresentation {
  switch (tone) {
    case 'spam':
      return {
        barClass: 'bg-orange-500',
        textClass: 'text-orange-600 dark:text-orange-400',
      };
    case 'virus':
      return {
        barClass: 'bg-red-500',
        textClass: 'text-red-600 dark:text-red-400',
      };
    case 'quarantine':
      return {
        barClass: 'bg-yellow-500',
        textClass: 'text-yellow-600 dark:text-yellow-400',
      };
  }
}
