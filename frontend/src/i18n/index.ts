import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';
import en from './locales/en.json';
import zh from './locales/zh.json';

const STORAGE_KEY = 'locale';

export type Locale = 'zh' | 'en';

function detectLocale(): Locale {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'zh' || saved === 'en') return saved;
    return navigator.language.startsWith('zh') ? 'zh' : 'en';
}

function syncDocumentTitle() {
    document.title = i18n.t('app.title');
}

void i18n
    .use(initReactI18next)
    .init({
        resources: {
            zh: {translation: zh},
            en: {translation: en}
        },
        lng: detectLocale(),
        fallbackLng: 'zh',
        interpolation: {escapeValue: false}
    })
    .then(syncDocumentTitle);

i18n.on('languageChanged', syncDocumentTitle);

export function setLocale(locale: Locale) {
    localStorage.setItem(STORAGE_KEY, locale);
    void i18n.changeLanguage(locale);
}

export default i18n;
