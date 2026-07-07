import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {listHistory, getHistory, type HistoryItem} from '../api';
import {parseCSV, type Row} from '../csv';
import {Dashboard} from '../components/BenchView';
import {useToast} from '../components/Toast';

const FILENAME_RE = /^bench_([^_]+)_([^_]+)_(\d+)_(\d{8})_(\d{6})\.csv$/;

function parseFilename(name: string): {model: string; channel: string; count: string} | null {
    const m = name.match(FILENAME_RE);
    if (!m) return null;
    return {model: m[1], channel: m[2], count: m[3]};
}

export default function ReviewPage() {
    const {t} = useTranslation();
    const toast = useToast();
    const [history, setHistory] = useState<HistoryItem[]>([]);
    const [rows, setRows] = useState<Row[] | null>(null);
    const [current, setCurrent] = useState('');
    const [dragging, setDragging] = useState(false);

    useEffect(() => {
        let cancelled = false;
        listHistory()
            .then(hs => {
                if (!cancelled) setHistory(hs);
            })
            .catch(e => {
                if (!cancelled) toast.error((e as Error).message);
            });
        return () => {
            cancelled = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const showCSV = (csv: string, label: string) => {
        const parsed = parseCSV(csv);
        if (parsed.length === 0) {
            toast.error(t('review.errors.emptyResult'));
            return;
        }
        setRows(parsed);
        setCurrent(label);
    };

    const handleFile = (file: File) => {
        if (!file.name.endsWith('.csv')) {
            toast.error(t('review.errors.csvOnly'));
            return;
        }
        const reader = new FileReader();
        reader.onload = e => showCSV(e.target?.result as string, file.name);
        reader.readAsText(file);
    };

    const onDrop = (e: React.DragEvent) => {
        e.preventDefault();
        setDragging(false);
        const f = e.dataTransfer.files[0];
        if (f) handleFile(f);
    };

    const onFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
        const f = e.target.files?.[0];
        if (f) handleFile(f);
    };

    const handleLoad = async (name: string) => {
        try {
            showCSV(await getHistory(name), name);
            toast.info(t('review.info.loaded', {name}));
        } catch (e) {
            toast.error((e as Error).message);
        }
    };

    if (rows) {
        return (
            <div className="flex flex-col gap-4">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold text-texth">{t('review.title')}</h1>
                        <span className="chip text-accent border-accent/40">{current}</span>
                    </div>
                    <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => {
                            setRows(null);
                            setCurrent('');
                        }}
                    >
                        {t('common.back')}
                    </button>
                </div>
                <Dashboard rows={rows} />
            </div>
        );
    }

    return (
        <div className="flex flex-col gap-6">
            <h1 className="text-2xl font-bold text-texth">{t('review.title')}</h1>

            <div
                className={`flex flex-wrap items-center justify-center gap-3 rounded-xl border-2 border-dashed px-6 py-5 transition-colors ${
                    dragging ? 'border-accent bg-accent/10' : 'border-line'
                }`}
                onDrop={onDrop}
                onDragOver={e => {
                    e.preventDefault();
                    setDragging(true);
                }}
                onDragLeave={() => setDragging(false)}
            >
                <span className="text-2xl">📂</span>
                <span className="text-sm text-text/80">{t('review.dragDrop')}</span>
                <span className="text-text/40 text-sm">{t('common.or')}</span>
                <label className="btn btn-primary btn-sm">
                    {t('review.selectFile')}
                    <input type="file" accept=".csv" onChange={onFileInput} className="hidden" />
                </label>
            </div>

            <section className="flex flex-col gap-3">
                <div className="section-title">{t('review.history')}</div>
                {history.length === 0 ? (
                    <div className="text-text/70 text-sm">{t('review.noHistory')}</div>
                ) : (
                    <div className="card overflow-hidden">
                        <div className="overflow-x-auto">
                            <table className="w-full text-sm">
                                <thead className="bg-surface2 text-texth">
                                    <tr>
                                        <th className="text-left font-semibold px-3 py-2">{t('common.model')}</th>
                                        <th className="text-left font-semibold px-3 py-2">{t('common.channel')}</th>
                                        <th className="text-left font-semibold px-3 py-2">{t('review.requestCount')}</th>
                                        <th className="text-left font-semibold px-3 py-2">{t('review.time')}</th>
                                        <th className="text-left font-semibold px-3 py-2">{t('review.filename')}</th>
                                        <th className="text-right font-semibold px-3 py-2"></th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {history.map(h => {
                                        const p = parseFilename(h.filename);
                                        return (
                                            <tr key={h.filename} className="border-t border-line hover:bg-surface2/50">
                                                <td className="px-3 py-2">{p?.model ?? '-'}</td>
                                                <td className="px-3 py-2">{p?.channel ?? '-'}</td>
                                                <td className="px-3 py-2">{p?.count ?? '-'}</td>
                                                <td className="px-3 py-2 text-text/70">{h.time}</td>
                                                <td className="px-3 py-2 font-mono text-xs text-text/60">
                                                    {h.filename}
                                                </td>
                                                <td className="px-3 py-2 text-right">
                                                    <button
                                                        className="btn btn-ghost btn-sm"
                                                        onClick={() => handleLoad(h.filename)}
                                                    >
                                                        {t('common.load')}
                                                    </button>
                                                </td>
                                            </tr>
                                        );
                                    })}
                                </tbody>
                            </table>
                        </div>
                    </div>
                )}
            </section>
        </div>
    );
}
