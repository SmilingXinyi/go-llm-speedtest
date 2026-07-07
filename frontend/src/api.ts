// 与后端 /api/channels 交互的轻量封装。
// Channel 字段与后端 config.Channel 的 json 标签（snake_case）对齐。

export interface Channel {
    name: string;
    base_url: string;
    token: string;
    models: string[];
}

export interface ApiError extends Error {
    status: number;
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
    const res = await fetch(url, {
        method,
        headers: body ? {'Content-Type': 'application/json'} : undefined,
        body: body ? JSON.stringify(body) : undefined
    });
    if (!res.ok) {
        let msg = `${res.status} ${res.statusText}`;
        try {
            const e = await res.json();
            if (e?.error) msg = e.error;
        } catch {
            /* ignore */
        }
        const err = new Error(msg) as ApiError;
        err.status = res.status;
        throw err;
    }
    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
}

export const listChannels = () => request<Channel[]>('GET', '/api/channels');

export const addChannel = (ch: Channel) => request<Channel>('POST', '/api/channels', ch);

export const removeChannel = (name: string) => request<void>('DELETE', `/api/channels/${encodeURIComponent(name)}`);

// BenchRunOptions 是 /api/bench 的请求体。
export interface BenchRunOptions {
    channel: string;
    model?: string;
    prompt: string;
    thinking: boolean;
    concurrency: number;
}

// BenchResponse 是 /api/bench 的响应：结果写入 history 目录后的文件名 + CSV 内容。
export interface BenchResponse {
    filename: string;
    csv: string;
}

// runBench 触发一次基准测试，后端将结果 CSV 写入 history 目录并返回内容，
// 前端按 CSV 渲染（与 Viewer 一致）。
export const runBench = (opts: BenchRunOptions) => request<BenchResponse>('POST', '/api/bench', opts);

// HistoryItem 是 /api/history 列表项。
export interface HistoryItem {
    filename: string;
    time: string;
}

// listHistory 列出 history 目录下的历史 CSV（最新在前）。
export const listHistory = () => request<HistoryItem[]>('GET', '/api/history');

// getHistory 读取指定历史 CSV 的文本内容（前端按 CSV 渲染）。
export async function getHistory(name: string): Promise<string> {
    const res = await fetch(`/api/history/${encodeURIComponent(name)}`);
    if (!res.ok) {
        let msg = `${res.status} ${res.statusText}`;
        try {
            const e = await res.json();
            if (e?.error) msg = e.error;
        } catch {
            /* ignore */
        }
        const err = new Error(msg) as ApiError;
        err.status = res.status;
        throw err;
    }
    return res.text();
}
