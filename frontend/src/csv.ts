// 一条基准测试结果行，字段与 CSV 英文表头、后端 /api/bench 一致。
// 值均为字符串（CSV 解析结果），由组件按需 parseFloat。
export interface Row {
    [key: string]: string;
}

// RFC 4180-compliant：逐字符解析，支持引号字段内含 \n。
// 表头取最后一个 `/` 后的英文段作为 key（如「首字节耗时/ttfb_ms」→ ttfb_ms）。
export function parseCSV(text: string): Row[] {
    const records: string[][] = [];
    let cur = '',
        inQuote = false,
        fields: string[] = [];
    const flush = () => {
        fields.push(cur);
        cur = '';
    };
    for (let i = 0; i < text.length; i++) {
        const ch = text[i];
        if (ch === '"') {
            if (inQuote && text[i + 1] === '"') {
                cur += '"';
                i++;
            } else {
                inQuote = !inQuote;
            }
        } else if (ch === ',' && !inQuote) {
            flush();
        } else if (ch === '\n' && !inQuote) {
            flush();
            if (fields.some(f => f !== '')) records.push(fields);
            fields = [];
        } else if (ch !== '\r') {
            cur += ch;
        }
    }
    flush();
    if (fields.some(f => f !== '')) records.push(fields);
    if (records.length < 2) return [];
    const headers = records[0].map(h => {
        const p = h.trim().split('/');
        return p[p.length - 1].trim();
    });
    return records.slice(1).map(values => {
        const row: Row = {} as Row;
        headers.forEach((h, i) => {
            row[h] = values[i] ?? '';
        });
        return row;
    });
}
