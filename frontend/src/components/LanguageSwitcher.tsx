import {useTranslation} from 'react-i18next';
import {setLocale, type Locale} from '../i18n';

export default function LanguageSwitcher() {
    const {i18n, t} = useTranslation();
    const current = i18n.language.startsWith('zh') ? 'zh' : 'en';

    const toggle = () => {
        const next: Locale = current === 'zh' ? 'en' : 'zh';
        setLocale(next);
    };

    return (
        <button
            type="button"
            className="flex items-center justify-center md:justify-start gap-3 rounded-lg px-3 py-2 text-sm text-text hover:bg-surface2 hover:text-texth transition-colors w-full"
            onClick={toggle}
            title={t('language.label')}
        >
            <svg
                className="w-[18px] h-[18px] shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
            >
                <circle cx="12" cy="12" r="10" />
                <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
            </svg>
            <span className="hidden md:inline">{current === 'zh' ? t('language.en') : t('language.zh')}</span>
            <span className="md:hidden text-xs font-medium">{current === 'zh' ? 'EN' : '中'}</span>
        </button>
    );
}
