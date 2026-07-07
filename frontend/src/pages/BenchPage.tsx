import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {listChannels, runBench, type Channel} from '../api';
import {parseCSV, type Row} from '../csv';
import {Dashboard} from '../components/BenchView';
import {useToast} from '../components/Toast';

export default function BenchPage() {
    const {t} = useTranslation();
    const toast = useToast();
    const [channels, setChannels] = useState<Channel[]>([]);
    const [channel, setChannel] = useState('');
    const [model, setModel] = useState('');
    const [prompt, setPrompt] = useState(() => t('bench.defaultPrompt'));
    const [thinking, setThinking] = useState(false);
    const [concurrency, setConcurrency] = useState('1');

    const [running, setRunning] = useState(false);
    const [rows, setRows] = useState<Row[] | null>(null);
    const [current, setCurrent] = useState('');

    useEffect(() => {
        let cancelled = false;
        listChannels()
            .then(chs => {
                if (cancelled) return;
                setChannels(chs);
                if (chs.length && !channel) {
                    setChannel(chs[0].name);
                    setModel(chs[0].models[0] ?? '');
                }
            })
            .catch(e => {
                if (!cancelled) toast.error((e as Error).message);
            });
        return () => {
            cancelled = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const selectedChannel = channels.find(c => c.name === channel);

    const onChannelChange = (name: string) => {
        setChannel(name);
        const ch = channels.find(c => c.name === name);
        setModel(ch?.models[0] ?? '');
    };

    const renderCSV = (csv: string, filename: string) => {
        const parsed = parseCSV(csv);
        if (parsed.length === 0) {
            toast.error(t('bench.errors.emptyResult'));
            return;
        }
        setRows(parsed);
        setCurrent(filename);
    };

    const handleRun = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!channel) {
            toast.error(t('bench.errors.selectChannel'));
            return;
        }
        const n = parseInt(concurrency);
        if (isNaN(n) || n < 1) {
            toast.error(t('bench.errors.concurrencyMin'));
            return;
        }
        if (n > 100) {
            toast.error(t('bench.errors.concurrencyMax'));
            return;
        }
        setRunning(true);
        setRows(null);
        setCurrent('');
        try {
            const {filename, csv} = await runBench({channel, model, prompt, thinking, concurrency: n});
            renderCSV(csv, filename);
            toast.success(t('bench.success.testComplete', {filename}));
        } catch (e) {
            toast.error((e as Error).message);
        } finally {
            setRunning(false);
        }
    };

    return (
        <div className="flex flex-col gap-6">
            <h1 className="text-2xl font-bold text-texth">{t('bench.title')}</h1>

            <section className="flex flex-col gap-3">
                <div className="section-title">{t('bench.testParams')}</div>
                <form className="card p-5 grid grid-cols-1 sm:grid-cols-2 gap-4" onSubmit={handleRun}>
                    <label className="block">
                        <span className="label">{t('bench.channelLabel')}</span>
                        <select className="input" value={channel} onChange={e => onChannelChange(e.target.value)}>
                            {channels.length === 0 && <option value="">{t('bench.noChannels')}</option>}
                            {channels.map(c => (
                                <option key={c.name} value={c.name}>
                                    {c.name}
                                </option>
                            ))}
                        </select>
                    </label>
                    <label className="block">
                        <span className="label">{t('bench.modelLabel')}</span>
                        <select className="input" value={model} onChange={e => setModel(e.target.value)}>
                            {(selectedChannel?.models ?? []).map(m => (
                                <option key={m} value={m}>
                                    {m}
                                </option>
                            ))}
                        </select>
                    </label>
                    <label className="block">
                        <span className="label">{t('bench.concurrencyLabel')}</span>
                        <input
                            type="text"
                            inputMode="numeric"
                            className="input"
                            value={concurrency}
                            onChange={e => setConcurrency(e.target.value)}
                        />
                    </label>
                    <label className="block">
                        <span className="label">{t('bench.thinkingLabel')}</span>
                        <div className="input flex items-center gap-3">
                            <input
                                type="checkbox"
                                className="w-4 h-4 accent-accent"
                                checked={thinking}
                                onChange={e => setThinking(e.target.checked)}
                            />
                            <span className="text-sm text-text">{thinking ? t('common.on') : t('common.off')}</span>
                        </div>
                    </label>
                    <label className="block sm:col-span-2">
                        <span className="label">{t('common.prompt')}</span>
                        <textarea
                            className="input min-h-20 resize-y"
                            rows={3}
                            value={prompt}
                            onChange={e => setPrompt(e.target.value)}
                        />
                    </label>
                    <div className="sm:col-span-2">
                        <button type="submit" className="btn btn-primary" disabled={running || !channel}>
                            {running && (
                                <span className="inline-block w-3 h-3 border-2 border-white/40 border-t-white rounded-full animate-spin" />
                            )}
                            {running ? t('bench.running') : t('bench.startTest')}
                        </button>
                    </div>
                </form>
            </section>

            {running && (
                <div className="flex items-center gap-2 text-text/70">
                    <span className="inline-block w-4 h-4 border-2 border-accent/40 border-t-accent rounded-full animate-spin" />
                    {t('bench.runningHint')}
                </div>
            )}

            {rows && (
                <section className="flex flex-col gap-3">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                            <div className="section-title">{t('bench.testResults')}</div>
                            <span className="chip text-accent border-accent/40">{current}</span>
                        </div>
                        <button
                            className="btn btn-ghost btn-sm"
                            onClick={() => {
                                setRows(null);
                                setCurrent('');
                            }}
                        >
                            {t('common.clear')}
                        </button>
                    </div>
                    <Dashboard rows={rows} />
                </section>
            )}
        </div>
    );
}
