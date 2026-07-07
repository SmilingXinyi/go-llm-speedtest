import {useTranslation} from 'react-i18next';
import {NavLink, Outlet} from 'react-router';
import LanguageSwitcher from '../components/LanguageSwitcher';

const iconClass = 'w-[18px] h-[18px] shrink-0';

interface NavItem {
    to: string;
    labelKey: string;
    end: boolean;
    icon: React.ReactNode;
}

const MAIN: NavItem[] = [
    {
        to: '/',
        labelKey: 'nav.review',
        end: true,
        icon: (
            <svg
                className={iconClass}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
            >
                <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
            </svg>
        )
    },
    {
        to: '/bench',
        labelKey: 'nav.bench',
        end: false,
        icon: (
            <svg
                className={iconClass}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
            >
                <path d="M9 3h6" />
                <path d="M10 3v5L5.5 17a2 2 0 0 0 1.8 3h9.4a2 2 0 0 0 1.8-3L14 8V3" />
                <path d="M7.5 14h9" />
            </svg>
        )
    }
];

const BOTTOM: NavItem[] = [
    {
        to: '/config',
        labelKey: 'nav.config',
        end: false,
        icon: (
            <svg
                className={iconClass}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
            >
                <path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3" />
                <path d="M2 14h4M10 8h4M18 16h4" />
            </svg>
        )
    }
];

function NavLinks({items}: {items: NavItem[]}) {
    const {t} = useTranslation();

    return (
        <>
            {items.map(item => {
                const label = t(item.labelKey);
                return (
                    <NavLink
                        key={item.to}
                        to={item.to}
                        end={item.end}
                        className={({isActive}) =>
                            `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors ${
                                isActive ? 'bg-surface2 text-accent' : 'text-text hover:bg-surface2 hover:text-texth'
                            }`
                        }
                        title={label}
                    >
                        {item.icon}
                        <span className="hidden md:inline">{label}</span>
                    </NavLink>
                );
            })}
        </>
    );
}

export default function AppLayout() {
    const {t} = useTranslation();

    return (
        <div className="flex min-h-screen bg-bg text-text">
            <aside className="w-16 md:w-60 shrink-0 bg-surface border-r border-line sticky top-0 h-screen flex flex-col gap-1 p-3">
                <div className="px-2 py-3 text-lg font-bold text-texth hidden md:block">{t('app.title')}</div>
                <div className="py-3 text-lg font-bold text-accent md:hidden text-center">L</div>

                <nav className="flex flex-col gap-1">
                    <NavLinks items={MAIN} />
                </nav>

                <nav className="mt-auto flex flex-col gap-1 border-t border-line pt-2">
                    <NavLinks items={BOTTOM} />
                    <LanguageSwitcher />
                </nav>
            </aside>

            <main className="flex-1 min-w-0 p-4 md:p-8">
                <div className="mx-auto w-full max-w-6xl">
                    <Outlet />
                </div>
            </main>
        </div>
    );
}
