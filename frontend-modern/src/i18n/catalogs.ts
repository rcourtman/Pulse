import { EN_MESSAGES, type I18nCatalogs } from './messages';
import { DE_MESSAGE_OVERRIDES } from './messages.de';
import { ES_MESSAGE_OVERRIDES } from './messages.es';
import { FR_MESSAGE_OVERRIDES } from './messages.fr';

export const I18N_MESSAGES: I18nCatalogs = {
  en: EN_MESSAGES,
  de: {
    ...EN_MESSAGES,
    ...DE_MESSAGE_OVERRIDES,
  },
  es: {
    ...EN_MESSAGES,
    ...ES_MESSAGE_OVERRIDES,
  },
  fr: {
    ...EN_MESSAGES,
    ...FR_MESSAGE_OVERRIDES,
  },
};
