export interface SecurityScoreTonePresentation {
  headerBg: string;
  headerBorder: string;
  iconWrap: string;
  icon: string;
  subtitle: string;
  score: string;
  badge: string;
}

export interface SecurityScorePresentation {
  label: 'Strong' | 'Moderate' | 'Weak';
  tone: SecurityScoreTonePresentation;
  icon: 'shield-check' | 'shield' | 'shield-alert';
}

export function getSecurityScorePresentation(score: number): SecurityScorePresentation {
  if (score >= 80) {
    return {
      label: 'Strong',
      icon: 'shield-check',
      tone: {
        headerBg: 'bg-emerald-50 dark:bg-emerald-950',
        headerBorder: 'border-b border-emerald-200 dark:border-emerald-800',
        iconWrap: 'bg-emerald-100 dark:bg-emerald-900',
        icon: 'text-emerald-700 dark:text-emerald-300',
        subtitle: 'text-emerald-700 dark:text-emerald-300',
        score: 'text-emerald-800 dark:text-emerald-200',
        badge: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
      },
    };
  }

  if (score >= 50) {
    return {
      label: 'Moderate',
      icon: 'shield',
      tone: {
        headerBg: 'bg-amber-50 dark:bg-amber-950',
        headerBorder: 'border-b border-amber-200 dark:border-amber-800',
        iconWrap: 'bg-amber-100 dark:bg-amber-900',
        icon: 'text-amber-700 dark:text-amber-300',
        subtitle: 'text-amber-700 dark:text-amber-300',
        score: 'text-amber-800 dark:text-amber-200',
        badge: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      },
    };
  }

  return {
    label: 'Weak',
    icon: 'shield-alert',
    tone: {
      headerBg: 'bg-rose-50 dark:bg-rose-950',
      headerBorder: 'border-b border-rose-200 dark:border-rose-800',
      iconWrap: 'bg-rose-100 dark:bg-rose-900',
      icon: 'text-rose-700 dark:text-rose-300',
      subtitle: 'text-rose-700 dark:text-rose-300',
      score: 'text-rose-800 dark:text-rose-200',
      badge: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
    },
  };
}

export function getSecurityScoreTextClass(score: number): string {
  return getSecurityScorePresentation(score).tone.icon;
}

export function getSecurityScoreSymbol(score: number): '✓' | '!' | '!!' {
  switch (getSecurityScorePresentation(score).icon) {
    case 'shield-check':
      return '✓';
    case 'shield':
      return '!';
    default:
      return '!!';
  }
}
