import { logger } from '@/utils/logger';

export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch (error) {
        logger.warn('Clipboard API copy failed, attempting fallback copy.', error);
      }
    }

    if (typeof document === 'undefined') {
      return false;
    }

    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    textArea.style.top = '-999999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
      return document.execCommand('copy');
    } catch (error) {
      logger.error('Fallback copy failed', error);
      return false;
    } finally {
      document.body.removeChild(textArea);
    }
  } catch (err) {
    logger.error('Failed to copy to clipboard', err);
    return false;
  }
}
