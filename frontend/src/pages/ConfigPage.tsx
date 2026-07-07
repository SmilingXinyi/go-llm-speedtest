import {useEffect, useState, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {addChannel, listChannels, removeChannel, type Channel} from '../api';
import {useToast} from '../components/Toast';

function maskToken(token: string): string {
    if (!token) return '-';
    if (token.length <= 5) return '***';
    return `${token.slice(0, 3)}***${token.slice(-2)}`;
}

const EMPTY: Channel = {name: '', base_url: '', token: '', models: []};

export default function ConfigPage() {
    const {t} = useTranslation();
    const toast = useToast();
    const [channels, setChannels] = useState<Channel[]>([]);
    const [loading, setLoading] = useState(true);

    const [form, setForm] = useState<Channel>({...EMPTY});
    const [modelsText, setModelsText] = useState('');
    const [saving, setSaving] = useState(false);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            setChannels(await listChannels());
        } catch (e) {
            toast.error((e as Error).message);
        } finally {
            setLoading(false);
        }
    }, [toast]);

    useEffect(() => {
        let cancelled = false;
        listChannels()
            .then(chs => {
                if (!cancelled) setChannels(chs);
            })
            .catch(e => {
                if (!cancelled) toast.error((e as Error).message);
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const handleAdd = async (e: React.FormEvent) => {
        e.preventDefault();
        const models = modelsText
            .split(/[\n,]/)
            .map(s => s.trim())
            .filter(Boolean);
        const ch: Channel = {...form, models};
        if (!ch.name || !ch.base_url || !ch.token || models.length === 0) {
            toast.error(t('config.errors.required'));
            return;
        }
        setSaving(true);
        try {
            await addChannel(ch);
            setForm({...EMPTY});
            setModelsText('');
            toast.success(t('config.success.added', {name: ch.name}));
            await refresh();
        } catch (e) {
            toast.error((e as Error).message);
        } finally {
            setSaving(false);
        }
    };

    const handleRemove = async (name: string) => {
        if (!confirm(t('config.confirm.delete', {name}))) return;
        try {
            await removeChannel(name);
            toast.success(t('config.success.removed', {name}));
            await refresh();
        } catch (e) {
            toast.error((e as Error).message);
        }
    };

    return (
        <div className="flex flex-col gap-6">
            <h1 className="text-2xl font-bold text-texth">{t('config.title')}</h1>

            <section className="flex flex-col gap-3">
                <div className="section-title">{t('config.configured')}</div>
                {loading ? (
                    <div className="text-text/70">{t('common.loading')}</div>
                ) : channels.length === 0 ? (
                    <div className="text-text/70">{t('config.noChannels')}</div>
                ) : (
                    <div className="grid gap-3">
                        {channels.map(ch => (
                            <div key={ch.name} className="card p-5 flex flex-col gap-3">
                                <div className="flex items-center justify-between">
                                    <span className="text-base font-semibold text-accent">{ch.name}</span>
                                    <button
                                        className="btn btn-ghost btn-sm text-error hover:bg-error/10"
                                        onClick={() => handleRemove(ch.name)}
                                    >
                                        {t('common.delete')}
                                    </button>
                                </div>
                                <div className="text-sm flex flex-wrap gap-x-2">
                                    <span className="text-text/60">{t('common.baseUrl')}</span>
                                    <code className="bg-surface2 px-1.5 rounded break-all text-texth">
                                        {ch.base_url}
                                    </code>
                                </div>
                                <div className="text-sm flex gap-x-2">
                                    <span className="text-text/60">{t('common.token')}</span>
                                    <code className="bg-surface2 px-1.5 rounded text-texth">{maskToken(ch.token)}</code>
                                </div>
                                <div className="text-sm flex flex-wrap items-center gap-2">
                                    <span className="text-text/60">{t('common.models')}</span>
                                    {ch.models.map(m => (
                                        <span key={m} className="chip">
                                            {m}
                                        </span>
                                    ))}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </section>

            <section className="flex flex-col gap-3">
                <div className="section-title">{t('config.addChannel')}</div>
                <form className="card p-5 grid grid-cols-1 sm:grid-cols-2 gap-4" onSubmit={handleAdd}>
                    <label className="block">
                        <span className="label">{t('config.nameLabel')}</span>
                        <input
                            className="input"
                            value={form.name}
                            onChange={e => setForm({...form, name: e.target.value})}
                            placeholder={t('config.namePlaceholder')}
                        />
                    </label>
                    <label className="block">
                        <span className="label">{t('common.baseUrl')}</span>
                        <input
                            className="input"
                            value={form.base_url}
                            onChange={e => setForm({...form, base_url: e.target.value})}
                            placeholder={t('config.baseUrlPlaceholder')}
                        />
                    </label>
                    <label className="block">
                        <span className="label">{t('common.token')}</span>
                        <input
                            className="input"
                            value={form.token}
                            onChange={e => setForm({...form, token: e.target.value})}
                            placeholder={t('config.tokenPlaceholder')}
                        />
                    </label>
                    <label className="block sm:col-span-2">
                        <span className="label">{t('config.modelsLabel')}</span>
                        <textarea
                            className="input min-h-20 resize-y"
                            rows={3}
                            value={modelsText}
                            onChange={e => setModelsText(e.target.value)}
                            placeholder={t('config.modelsPlaceholder')}
                        />
                    </label>
                    <div className="sm:col-span-2">
                        <button type="submit" className="btn btn-primary" disabled={saving}>
                            {saving && (
                                <span className="inline-block w-3 h-3 border-2 border-white/40 border-t-white rounded-full animate-spin" />
                            )}
                            {saving ? t('config.submitting') : t('config.addChannel')}
                        </button>
                    </div>
                </form>
            </section>
        </div>
    );
}
