import {useEffect, useState, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {addPrompt, listPrompts, removePrompt, updatePrompt, type PromptTemplate} from '../api';
import {useToast} from '../components/Toast';

const EMPTY: PromptTemplate = {name: '', system_prompt: '', prompt: ''};

export default function PromptsPage() {
    const {t} = useTranslation();
    const toast = useToast();
    const [templates, setTemplates] = useState<PromptTemplate[]>([]);
    const [tplForm, setTplForm] = useState<PromptTemplate>({...EMPTY});
    const [tplSaving, setTplSaving] = useState(false);
    const [editingTpl, setEditingTpl] = useState<string | null>(null);

    const refreshTemplates = useCallback(async () => {
        try {
            setTemplates(await listPrompts());
        } catch (e) {
            toast.error((e as Error).message);
        }
    }, [toast]);

    useEffect(() => {
        let cancelled = false;
        listPrompts()
            .then(list => {
                if (!cancelled) setTemplates(list);
            })
            .catch(e => {
                if (!cancelled) toast.error((e as Error).message);
            });
        return () => {
            cancelled = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const resetTplForm = () => {
        setTplForm({...EMPTY});
        setEditingTpl(null);
    };

    const handleTplSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        const tpl: PromptTemplate = {...tplForm, name: tplForm.name.trim()};
        if (!tpl.name || (!tpl.system_prompt.trim() && !tpl.prompt.trim())) {
            toast.error(t('promptTemplates.errors.required'));
            return;
        }
        setTplSaving(true);
        try {
            if (editingTpl) {
                await updatePrompt(editingTpl, tpl);
                toast.success(t('promptTemplates.success.updated', {name: tpl.name}));
            } else {
                await addPrompt(tpl);
                toast.success(t('promptTemplates.success.added', {name: tpl.name}));
            }
            resetTplForm();
            await refreshTemplates();
        } catch (e) {
            toast.error((e as Error).message);
        } finally {
            setTplSaving(false);
        }
    };

    const handleTplEdit = (tpl: PromptTemplate) => {
        setEditingTpl(tpl.name);
        setTplForm({...tpl});
    };

    const handleTplRemove = async (name: string) => {
        if (!confirm(t('promptTemplates.confirm.delete', {name}))) return;
        try {
            await removePrompt(name);
            toast.success(t('promptTemplates.success.removed', {name}));
            if (editingTpl === name) resetTplForm();
            await refreshTemplates();
        } catch (e) {
            toast.error((e as Error).message);
        }
    };

    return (
        <div className="flex flex-col gap-6">
            <h1 className="text-2xl font-bold text-texth">{t('promptTemplates.title')}</h1>

            <section className="flex flex-col gap-3">
                <div className="section-title">{t('promptTemplates.configured')}</div>
                {templates.length === 0 ? (
                    <div className="text-text/70">{t('promptTemplates.noTemplates')}</div>
                ) : (
                    <div className="grid gap-3">
                        {templates.map(tpl => (
                            <div key={tpl.name} className="card p-5 flex flex-col gap-3">
                                <div className="flex items-center justify-between">
                                    <div className="flex items-center gap-2">
                                        <span className="text-base font-semibold text-accent">{tpl.name}</span>
                                        <span className="chip">
                                            {tpl.system_prompt && tpl.prompt
                                                ? t('promptTemplates.kindBoth')
                                                : tpl.system_prompt
                                                  ? t('promptTemplates.kindSystem')
                                                  : t('promptTemplates.kindUser')}
                                        </span>
                                    </div>
                                    <div className="flex gap-2">
                                        <button className="btn btn-ghost btn-sm" onClick={() => handleTplEdit(tpl)}>
                                            {t('common.edit')}
                                        </button>
                                        <button
                                            className="btn btn-ghost btn-sm text-error hover:bg-error/10"
                                            onClick={() => handleTplRemove(tpl.name)}
                                        >
                                            {t('common.delete')}
                                        </button>
                                    </div>
                                </div>
                                {tpl.system_prompt && (
                                    <div className="flex flex-col gap-1">
                                        <span className="text-sm text-text/60">{t('common.systemPrompt')}</span>
                                        <pre className="bg-surface2 rounded-lg p-3 text-xs whitespace-pre-wrap break-words max-h-40 overflow-auto">
                                            {tpl.system_prompt}
                                        </pre>
                                    </div>
                                )}
                                {tpl.prompt && (
                                    <div className="flex flex-col gap-1">
                                        <span className="text-sm text-text/60">{t('common.prompt')}</span>
                                        <pre className="bg-surface2 rounded-lg p-3 text-xs whitespace-pre-wrap break-words max-h-40 overflow-auto">
                                            {tpl.prompt}
                                        </pre>
                                    </div>
                                )}
                            </div>
                        ))}
                    </div>
                )}

                <div className="section-title">
                    {editingTpl ? t('promptTemplates.editTemplate') : t('promptTemplates.addTemplate')}
                </div>
                <form className="card p-5 grid grid-cols-1 gap-4" onSubmit={handleTplSubmit}>
                    <label className="block">
                        <span className="label">{t('promptTemplates.nameLabel')}</span>
                        <input
                            className="input"
                            value={tplForm.name}
                            onChange={e => setTplForm({...tplForm, name: e.target.value})}
                            placeholder={t('promptTemplates.namePlaceholder')}
                        />
                    </label>
                    <label className="block">
                        <span className="label">{t('common.systemPrompt')}</span>
                        <textarea
                            className="input min-h-20 resize-y"
                            rows={3}
                            value={tplForm.system_prompt}
                            onChange={e => setTplForm({...tplForm, system_prompt: e.target.value})}
                            placeholder={t('promptTemplates.systemPromptPlaceholder')}
                        />
                    </label>
                    <label className="block">
                        <span className="label">{t('common.prompt')}</span>
                        <textarea
                            className="input min-h-20 resize-y"
                            rows={3}
                            value={tplForm.prompt}
                            onChange={e => setTplForm({...tplForm, prompt: e.target.value})}
                            placeholder={t('promptTemplates.promptPlaceholder')}
                        />
                    </label>
                    <div className="flex gap-2">
                        <button type="submit" className="btn btn-primary" disabled={tplSaving}>
                            {tplSaving && (
                                <span className="inline-block w-3 h-3 border-2 border-white/40 border-t-white rounded-full animate-spin" />
                            )}
                            {tplSaving
                                ? t('config.submitting')
                                : editingTpl
                                  ? t('config.saveChanges')
                                  : t('promptTemplates.addTemplate')}
                        </button>
                        {editingTpl && (
                            <button type="button" className="btn btn-ghost" onClick={resetTplForm} disabled={tplSaving}>
                                {t('common.cancel')}
                            </button>
                        )}
                    </div>
                </form>
            </section>
        </div>
    );
}
