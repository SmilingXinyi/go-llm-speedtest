import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {type Row} from '../csv';

const TABLE_COLS = [
    'index',
    'model',
    'ttfb_ms',
    'total_ms',
    'token_rate',
    'prompt_tokens',
    'completion_tokens',
    'total_tokens',
    'cached_tokens'
] as const;

function Stat({label, value, unit}: {label: string; value: string; unit?: string}) {
    return (
        <div className="card p-4">
            <div className="text-xs text-text/60">{label}</div>
            <div className="mt-1 text-2xl font-bold text-texth">
                {value}
                {unit && <span className="ml-1 text-sm font-normal text-text/60">{unit}</span>}
            </div>
        </div>
    );
}

function BarChart({
    data,
    valueKey,
    labelKey,
    title,
    unit
}: {
    data: Row[];
    valueKey: string;
    labelKey: string;
    title: string;
    unit?: string;
}) {
    const values = data.map(r => parseFloat(r[valueKey]) || 0);
    const max = Math.max(...values, 1);
    return (
        <div className="card p-4">
            <div className="text-sm font-semibold text-texth mb-3">{title}</div>
            <div className="flex flex-col gap-1.5">
                {data.map((row, i) => (
                    <div key={i} className="flex items-center gap-2">
                        <div className="w-8 text-right text-xs text-text/60 shrink-0">#{row[labelKey]}</div>
                        <div className="flex-1 h-3 rounded bg-surface2 overflow-hidden">
                            <div
                                className="h-full rounded bg-gradient-to-r from-accent to-accent2 transition-all duration-500"
                                style={{width: `${(values[i] / max) * 100}%`}}
                            />
                        </div>
                        <div className="w-14 text-right text-xs text-text shrink-0">
                            {values[i].toFixed(0)}
                            {unit}
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}

function KV({label, value, accent}: {label: string; value: string; accent?: boolean}) {
    return (
        <div className="bg-surface2 rounded-lg p-2">
            <div className="text-xs text-text/60">{label}</div>
            <div className={`font-semibold text-sm ${accent ? 'text-accent2' : 'text-texth'}`}>{value}</div>
        </div>
    );
}

function RowModal({row, onClose}: {row: Row; onClose: () => void}) {
    const {t} = useTranslation();
    const ttfb = parseFloat(row.ttfb_ms) || 0;
    const total = parseFloat(row.total_ms) || 0;
    const generate = total - ttfb;
    const ttfbPct = total > 0 ? (ttfb / total) * 100 : 0;
    const genPct = total > 0 ? (generate / total) * 100 : 0;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-6" onClick={onClose}>
            <div className="card w-full max-w-2xl max-h-[85vh] overflow-auto" onClick={e => e.stopPropagation()}>
                <div className="p-6 flex flex-col gap-4">
                    <div className="flex items-center justify-between">
                        <h3 className="text-lg font-semibold text-texth">
                            {t('dashboard.requestDetail', {index: row.index})}
                        </h3>
                        <button className="btn btn-ghost btn-sm" onClick={onClose}>
                            ✕
                        </button>
                    </div>

                    <div className="flex flex-col gap-1">
                        <div className="section-title">{t('common.model')}</div>
                        <div className="text-accent">
                            {row.model || '-'}
                            {row.channel ? ` · ${row.channel}` : ''}
                        </div>
                    </div>

                    <div className="flex flex-col gap-2">
                        <div className="section-title">{t('dashboard.timeline')}</div>
                        <div className="flex h-4 rounded bg-surface2 overflow-hidden">
                            <div
                                className="bg-accent"
                                style={{width: `${ttfbPct}%`}}
                                title={`${t('dashboard.firstToken')} ${ttfb.toFixed(0)}ms`}
                            />
                            <div
                                className="bg-accent2"
                                style={{width: `${genPct}%`}}
                                title={`${t('dashboard.generate')} ${generate.toFixed(0)}ms`}
                            />
                        </div>
                        <div className="flex flex-wrap gap-4 text-xs text-text/80">
                            <span className="flex items-center gap-1">
                                <span className="inline-block w-2 h-2 rounded-sm bg-accent" />
                                {t('dashboard.firstToken')} {ttfb.toFixed(0)} ms
                            </span>
                            <span className="flex items-center gap-1">
                                <span className="inline-block w-2 h-2 rounded-sm bg-accent2" />
                                {t('dashboard.generate')} {generate.toFixed(0)} ms
                            </span>
                            <span>
                                {t('dashboard.total')} {total.toFixed(0)} ms
                            </span>
                        </div>
                    </div>

                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                        <KV
                            label={t('dashboard.fields.tokenRate')}
                            value={row.token_rate ? `${parseFloat(row.token_rate).toFixed(1)} tok/s` : '-'}
                        />
                        <KV label={t('dashboard.fields.promptTokens')} value={row.prompt_tokens || '-'} />
                        <KV label={t('dashboard.fields.completionTokens')} value={row.completion_tokens || '-'} />
                        <KV label={t('dashboard.fields.totalTokens')} value={row.total_tokens || '-'} />
                        {(() => {
                            const cached = parseInt(row.cached_tokens) || 0;
                            const prompt = parseInt(row.prompt_tokens) || 0;
                            return (
                                <KV
                                    label={t('dashboard.fields.cacheHit')}
                                    value={`${cached} tok${cached > 0 && prompt > 0 ? ` · ${Math.round((cached / prompt) * 100)}%` : ''}`}
                                />
                            );
                        })()}
                        <KV label={t('dashboard.fields.finishReason')} value={row.finish_reason || '-'} />
                    </div>

                    {(row.dns_ms || row.tcp_ms || row.tls_ms) && (
                        <div className="flex flex-col gap-2">
                            <div className="section-title">{t('dashboard.networkLatency')}</div>
                            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                                {row.conn_reused === 'true' ? (
                                    <KV label={t('dashboard.connectionStatus')} value={t('dashboard.connectionReused')} accent />
                                ) : (
                                    <>
                                        {row.dns_ms && <KV label="DNS" value={`${row.dns_ms} ms`} />}
                                        {row.tcp_ms && <KV label="TCP" value={`${row.tcp_ms} ms`} />}
                                        {row.tls_ms && <KV label="TLS" value={`${row.tls_ms} ms`} />}
                                    </>
                                )}
                                {row.probe_latency_ms && (
                                    <KV label={t('dashboard.probeLatency')} value={`${row.probe_latency_ms} ms`} />
                                )}
                            </div>
                        </div>
                    )}

                    {row.prompt && (
                        <div className="flex flex-col gap-1">
                            <div className="section-title">{t('common.prompt')}</div>
                            <pre className="bg-surface2 rounded-lg p-3 text-xs whitespace-pre-wrap break-words max-h-60 overflow-auto">
                                {row.prompt}
                            </pre>
                        </div>
                    )}
                    {row.content && (
                        <div className="flex flex-col gap-1">
                            <div className="section-title">{t('dashboard.replyContent')}</div>
                            <pre className="bg-surface2 rounded-lg p-3 text-xs whitespace-pre-wrap break-words max-h-60 overflow-auto">
                                {row.content}
                            </pre>
                        </div>
                    )}
                    {row.error && (
                        <div className="flex flex-col gap-1">
                            <div className="section-title text-error">{t('common.error')}</div>
                            <pre className="bg-surface2 rounded-lg p-3 text-xs whitespace-pre-wrap break-words max-h-60 overflow-auto text-error">
                                {row.error}
                            </pre>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

function DataTable({rows}: {rows: Row[]}) {
    const {t} = useTranslation();
    const [selected, setSelected] = useState<Row | null>(null);

    return (
        <>
            <div className="card overflow-hidden">
                <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                        <thead className="bg-surface2 text-texth">
                            <tr>
                                {TABLE_COLS.map(c => (
                                    <th key={c} className="text-left font-semibold px-3 py-2 whitespace-nowrap">
                                        {t(`dashboard.columns.${c}`, {defaultValue: c})}
                                    </th>
                                ))}
                            </tr>
                        </thead>
                        <tbody>
                            {rows.map((row, i) => (
                                <tr
                                    key={i}
                                    className={`border-t border-line cursor-pointer hover:bg-surface2/50 ${row.error ? 'text-error' : ''}`}
                                    onClick={() => setSelected(row)}
                                >
                                    {TABLE_COLS.map(c => (
                                        <td key={c} className="px-3 py-2 max-w-[200px] truncate">
                                            {row[c]}
                                        </td>
                                    ))}
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
            {selected && <RowModal row={selected} onClose={() => setSelected(null)} />}
        </>
    );
}

function avg(rows: Row[], key: string, excludeZeroDep?: string) {
    const filtered = excludeZeroDep ? rows.filter(r => parseFloat(r[excludeZeroDep]) > 0) : rows;
    const vals = filtered.map(r => parseFloat(r[key])).filter(v => !isNaN(v) && v > 0);
    return vals.length ? (vals.reduce((a, b) => a + b, 0) / vals.length).toFixed(1) : '-';
}

export function Dashboard({rows}: {rows: Row[]}) {
    const {t} = useTranslation();
    const models = [...new Set(rows.map(r => r.model).filter(Boolean))];

    return (
        <div className="flex flex-col gap-4">
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
                <Stat label={t('dashboard.requestTotal')} value={String(rows.length)} />
                <Stat label={t('dashboard.avgTtfb')} value={avg(rows, 'ttfb_ms')} unit="ms" />
                <Stat label={t('dashboard.avgTotal')} value={avg(rows, 'total_ms')} unit="ms" />
                <Stat label={t('dashboard.avgTokenRate')} value={avg(rows, 'token_rate', 'first_token_to_end_ms')} unit="tok/s" />
                <Stat label={t('dashboard.modelCount')} value={String(models.length)} />
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                <BarChart
                    data={rows}
                    valueKey="ttfb_ms"
                    labelKey="index"
                    title={t('dashboard.chartTtfb')}
                    unit="ms"
                />
                <BarChart data={rows} valueKey="total_ms" labelKey="index" title={t('dashboard.chartTotal')} unit="ms" />
                <BarChart data={rows} valueKey="token_rate" labelKey="index" title={t('dashboard.chartTokenRate')} unit="" />
            </div>
            <DataTable rows={rows} />
        </div>
    );
}
