import {createContext, useCallback, useContext, useState, type ReactNode} from 'react';

// 全局消息通知：右上角堆叠卡片，自动消失。取代页面内联提示横幅。

type ToastType = 'success' | 'error' | 'info';

interface ToastItem {
    id: number;
    type: ToastType;
    msg: string;
}

interface ToastApi {
    success: (msg: string) => void;
    error: (msg: string) => void;
    info: (msg: string) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

let counter = 0;

const STYLE: Record<ToastType, string> = {
    success: 'border-accent2 text-accent2',
    error: 'border-error text-error',
    info: 'border-accent text-accent'
};

export function ToastProvider({children}: {children: ReactNode}) {
    const [items, setItems] = useState<ToastItem[]>([]);

    const remove = useCallback((id: number) => {
        setItems(a => a.filter(t => t.id !== id));
    }, []);

    const push = useCallback(
        (type: ToastType, msg: string) => {
            const id = ++counter;
            setItems(a => [...a, {id, type, msg}]);
            setTimeout(() => remove(id), 3500);
        },
        [remove]
    );

    const api: ToastApi = {
        success: m => push('success', m),
        error: m => push('error', m),
        info: m => push('info', m)
    };

    return (
        <ToastContext.Provider value={api}>
            {children}
            <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 w-80 max-w-[calc(100vw-2rem)]">
                {items.map(t => (
                    <div
                        key={t.id}
                        className={`card px-4 py-3 text-sm flex items-center justify-between gap-2 ${STYLE[t.type]}`}
                    >
                        <span>{t.msg}</span>
                        <button className="opacity-50 hover:opacity-100 shrink-0" onClick={() => remove(t.id)}>
                            ✕
                        </button>
                    </div>
                ))}
            </div>
        </ToastContext.Provider>
    );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useToast(): ToastApi {
    const ctx = useContext(ToastContext);
    if (!ctx) {
        throw new Error('useToast must be used within ToastProvider');
    }
    return ctx;
}
