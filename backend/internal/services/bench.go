// Package services 封装 LLM Bench 的可复用业务逻辑。
//
// Service 将一次「按渠道 + 模型发起并发流式请求、采集连接/阶段耗时、输出结果」
// 的完整流程封装为 Run 方法，供 CLI 及未来其它入口（HTTP server、批量调度等）复用。
// 配置来源为 internal/config 加载的 llm.yaml，请求与 SSE 解析复用 internal/llm。
package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/config"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/llm"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/providers/nange"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/providers/qianfan"
)

// RunOptions 是一次 Bench 运行的参数。
type RunOptions struct {
	Channel     string // 渠道名（必填，需在 llm.yaml 中存在）
	Model       string // 模型名；为空时取渠道 models[0]；非空时需在渠道 models 列表内
	Prompt      string // prompt 文本（main.go 已完成 file:// 解析）
	Thinking    bool   // 是否开启 thinking；false 时显式下发 disabled
	Concurrency int    // 并发请求数，<1 视为 1
	Out         string // "csv" 写文件，空串走控制台打印
}

// Service 持有已加载的渠道配置，提供 Bench 运行能力。
type Service struct {
	cfg *config.Config
}

// New 创建一个使用给定配置的 Service。
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// connTrace 记录一次请求的连接阶段时间点（httptrace 回调填充）。
type connTrace struct {
	dnsStart, dnsDone time.Time
	tcpStart, tcpDone time.Time
	tlsStart, tlsDone time.Time
	gotConn           time.Time
	wroteRequest      time.Time
	reused            bool
}

// runResult 是单个并发请求的汇总结果。
type runResult struct {
	index        int
	result       *llm.ChatResult
	err          error
	boot         time.Time
	ct           connTrace
	probeLatency time.Duration
	tokenRate    float64
}

// run 执行一次 Bench 的核心流程：解析渠道与模型、并发发起流式请求、收集结果。
// 被 Run（CLI，写 CSV/控制台）与 RunRows（HTTP，返回结构化行）复用。
func (s *Service) run(ctx context.Context, opts RunOptions) (channel, model string, results []runResult, err error) {
	ch, err := s.cfg.Channel(opts.Channel)
	if err != nil {
		return "", "", nil, err
	}

	model = opts.Model
	if model == "" {
		model = ch.Models[0] // 列表为默认候选，省略时取首个
	} else if !ch.HasModel(model) {
		return "", "", nil, fmt.Errorf("模型 %q 不在渠道 %s 的 models 列表中", model, ch.Name)
	}

	promptText := opts.Prompt
	n := opts.Concurrency
	if n < 1 {
		n = 1
	}

	results = make([]runResult, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			boot := time.Now()

			var ct connTrace
			trace := &httptrace.ClientTrace{
				DNSStart:          func(httptrace.DNSStartInfo) { ct.dnsStart = time.Now() },
				DNSDone:           func(httptrace.DNSDoneInfo) { ct.dnsDone = time.Now() },
				ConnectStart:      func(string, string) { ct.tcpStart = time.Now() },
				ConnectDone:       func(string, string, error) { ct.tcpDone = time.Now() },
				TLSHandshakeStart: func() { ct.tlsStart = time.Now() },
				TLSHandshakeDone:  func(tls.ConnectionState, error) { ct.tlsDone = time.Now() },
				GotConn:           func(i httptrace.GotConnInfo) { ct.gotConn = time.Now(); ct.reused = i.Reused },
				WroteRequest:      func(httptrace.WroteRequestInfo) { ct.wroteRequest = time.Now() },
			}

			client := &llm.Client{
				BaseURL:  strings.TrimRight(ch.BaseURL, "/"),
				APIKey:   ch.Token,
				Model:    model,
				HTTP:     &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()},
				Provider: providerFor(ch.Name),
			}
			// 与原 CLI 行为一致：仅当关闭 thinking 时显式下发 disabled；
			// 开启 thinking 时不设置字段，使用模型默认（部分模型默认即开启）。
			if !opts.Thinking {
				no := false
				client.EnableThinking = &no
			}

			res, err := client.StreamChat(
				httptrace.WithClientTrace(ctx, trace),
				[]llm.Message{{Role: "user", Content: promptText}},
				nil, nil,
			)

			rr := runResult{index: idx, result: res, err: err, boot: boot, ct: ct}
			if err == nil && res != nil {
				pl := ct.gotConn.Sub(res.StartedAt)
				if ct.reused {
					pl = 0
				}
				rr.probeLatency = pl
				if res.Usage != nil && !res.FirstTokenAt.IsZero() {
					genSec := res.EndedAt.Sub(res.FirstTokenAt).Seconds()
					if genSec > 0 {
						rr.tokenRate = float64(res.Usage.CompletionTokens) / genSec
					}
				}
			}
			results[idx] = rr
		}(i)
	}

	wg.Wait()
	return ch.Name, model, results, nil
}

// Run 执行一次 Bench：解析渠道与模型、并发发起流式请求、按 Out 输出结果。
func (s *Service) Run(ctx context.Context, opts RunOptions) error {
	channel, model, results, err := s.run(ctx, opts)
	if err != nil {
		return err
	}
	if opts.Out == "csv" {
		filename, data := buildCSV(results, channel, model, opts.Thinking, opts.Prompt)
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			return fmt.Errorf("写入 CSV 文件失败: %w", err)
		}
		fmt.Printf("结果已写入 %s\n", filename)
		return nil
	}
	printConsole(results, channel, model, opts.Thinking, opts.Prompt, len(results))
	return nil
}

// RunCSV 执行一次 Bench，将结果 CSV 写入 dir（目录不存在则创建），返回文件名与 CSV 内容。
// 供 HTTP /api/bench 使用：持久化到 history 目录，并返回内容供前端按 CSV 渲染（与 Viewer 对齐）。
func (s *Service) RunCSV(ctx context.Context, opts RunOptions, dir string) (string, []byte, error) {
	channel, model, results, err := s.run(ctx, opts)
	if err != nil {
		return "", nil, err
	}
	filename, data := buildCSV(results, channel, model, opts.Thinking, opts.Prompt)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, fmt.Errorf("创建历史目录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o600); err != nil {
		return "", nil, fmt.Errorf("写入 CSV 文件失败: %w", err)
	}
	return filename, data, nil
}

// RunRows 执行一次 Bench 并返回结构化结果行（map 的 key 与 CSV 表头英文段一致，
// 也与前端 Viewer 的 Row 字段对齐），供 HTTP /api/bench 直接 JSON 返回、前端复用 Dashboard 展示。
func (s *Service) RunRows(ctx context.Context, opts RunOptions) ([]map[string]string, error) {
	channel, model, results, err := s.run(ctx, opts)
	if err != nil {
		return nil, err
	}
	return buildRows(results, channel, model, opts.Thinking, opts.Prompt), nil
}

// buildRows 将 runResult 列表格式化为前端可消费的行（key 为英文字段名，value 均为字符串）。
func buildRows(results []runResult, channel, model string, thinking bool, promptText string) []map[string]string {
	fmtT := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04:05.000")
	}
	fmtMs := func(d time.Duration) string {
		if d <= 0 {
			return ""
		}
		return strconv.FormatInt(d.Milliseconds(), 10)
	}
	rows := make([]map[string]string, 0, len(results))
	for _, rr := range results {
		row := map[string]string{
			"index":                strconv.Itoa(rr.index + 1),
			"model":                "",
			"channel":              channel,
			"thinking":             strconv.FormatBool(thinking),
			"prompt":               promptText,
			"content":              "",
			"reasoning_content":    "",
			"finish_reason":        "",
			"prompt_tokens":        "",
			"completion_tokens":    "",
			"total_tokens":         "",
			"cached_tokens":        "",
			"boot_time":            fmtT(rr.boot),
			"dns_start":            fmtT(rr.ct.dnsStart),
			"dns_done":             fmtT(rr.ct.dnsDone),
			"dns_ms":               fmtMs(rr.ct.dnsDone.Sub(rr.ct.dnsStart)),
			"tcp_start":            fmtT(rr.ct.tcpStart),
			"tcp_done":             fmtT(rr.ct.tcpDone),
			"tcp_ms":               fmtMs(rr.ct.tcpDone.Sub(rr.ct.tcpStart)),
			"tls_start":            fmtT(rr.ct.tlsStart),
			"tls_done":             fmtT(rr.ct.tlsDone),
			"tls_ms":               fmtMs(rr.ct.tlsDone.Sub(rr.ct.tlsStart)),
			"conn_reused":          strconv.FormatBool(rr.ct.reused),
			"got_conn":             fmtT(rr.ct.gotConn),
			"wrote_request":        fmtT(rr.ct.wroteRequest),
			"started_at":           "",
			"first_token_at":       "",
			"ended_at":             "",
			"startup_to_request_ms": "",
			"ttfb_ms":              "",
			"first_token_to_end_ms": "",
			"total_ms":             "",
			"probe_latency_ms":     "",
			"token_rate":           "",
			"error":                "",
		}
		if rr.err != nil {
			row["error"] = rr.err.Error()
		} else {
			r := rr.result
			row["model"] = r.Model
			row["content"] = r.Content
			row["reasoning_content"] = r.ReasoningContent
			row["finish_reason"] = r.FinishReason
			if r.Usage != nil {
				row["prompt_tokens"] = strconv.Itoa(r.Usage.PromptTokens)
				row["completion_tokens"] = strconv.Itoa(r.Usage.CompletionTokens)
				row["total_tokens"] = strconv.Itoa(r.Usage.TotalTokens)
				if r.Usage.PromptTokensDetails != nil {
					row["cached_tokens"] = strconv.Itoa(r.Usage.PromptTokensDetails.CachedTokens)
				}
			}
			row["started_at"] = fmtT(r.StartedAt)
			row["first_token_at"] = fmtT(r.FirstTokenAt)
			row["ended_at"] = fmtT(r.EndedAt)
			row["startup_to_request_ms"] = strconv.FormatInt(r.StartedAt.Sub(rr.boot).Milliseconds(), 10)
			row["ttfb_ms"] = strconv.FormatInt(r.FirstTokenAt.Sub(r.StartedAt).Milliseconds(), 10)
			row["first_token_to_end_ms"] = strconv.FormatInt(r.EndedAt.Sub(r.FirstTokenAt).Milliseconds(), 10)
			row["total_ms"] = strconv.FormatInt(r.EndedAt.Sub(rr.boot).Milliseconds(), 10)
			row["probe_latency_ms"] = strconv.FormatInt(rr.probeLatency.Milliseconds(), 10)
			row["token_rate"] = fmt.Sprintf("%.1f", rr.tokenRate)
		}
		rows = append(rows, row)
	}
	return rows
}

// safeName 把字符串规整为文件名安全片段：仅替换对文件名或解析不安全的字符
// （下划线、路径分隔符、空格、控制符等）为 '-'，保留字母、数字、中文等 Unicode 字符。
// 这样文件名中的下划线只用于分隔字段，便于反向解析；同时不丢失中文渠道名等信息。
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '_', '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '-'
		}
		if r < 32 {
			return '-'
		}
		return r
	}, s)
}

// buildCSV 将结果构建为 CSV 字节流，返回文件名与内容。
// 文件名格式：bench_<模型>_<渠道>_<请求数量>_<日期>_<时间>.csv，
// 模型/渠道经 safeName 处理（不含下划线），保证下划线仅作字段分隔、可反向解析。
// 35 列双语表头；error 列与表头对齐（前端 Viewer 取表头英文段为 key）。
func buildCSV(results []runResult, channel, model string, thinking bool, promptText string) (string, []byte) {
	filename := fmt.Sprintf("bench_%s_%s_%d_%s.csv",
		safeName(model), safeName(channel), len(results), time.Now().Format("20060102_150405"))
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"序号/index", "模型/model", "渠道/channel", "思考模式/thinking", "提示词/prompt", "回复内容/content", "推理内容/reasoning_content",
		"结束原因/finish_reason",
		"提示词token数/prompt_tokens", "补全token数/completion_tokens", "总token数/total_tokens", "缓存token数/cached_tokens",
		"启动时间/boot_time",
		"DNS开始/dns_start", "DNS完成/dns_done", "DNS耗时/dns_ms",
		"TCP开始/tcp_start", "TCP完成/tcp_done", "TCP耗时/tcp_ms",
		"TLS开始/tls_start", "TLS完成/tls_done", "TLS耗时/tls_ms",
		"连接复用/conn_reused",
		"获取连接/got_conn", "请求已写/wrote_request",
		"请求开始时间/started_at", "首token时间/first_token_at", "结束时间/ended_at",
		"启动到请求耗时/startup_to_request_ms", "首字节耗时/ttfb_ms", "首token到结束耗时/first_token_to_end_ms", "总耗时/total_ms",
		"探测延迟/probe_latency_ms", "token速率/token_rate",
		"错误/error",
	})
	fmtT := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04:05.000")
	}
	fmtMs := func(d time.Duration) string {
		if d <= 0 {
			return ""
		}
		return strconv.FormatInt(d.Milliseconds(), 10)
	}
	for _, rr := range results {
		row := make([]string, 35)
		row[0] = strconv.Itoa(rr.index + 1)
		row[3] = strconv.FormatBool(thinking)
		row[4] = promptText
		if rr.err != nil {
			row[34] = rr.err.Error()
		} else {
			r := rr.result
			ct := rr.ct
			row[1] = r.Model
			row[2] = channel
			row[5] = r.Content
			row[6] = r.ReasoningContent
			row[7] = r.FinishReason
			if r.Usage != nil {
				row[8] = strconv.Itoa(r.Usage.PromptTokens)
				row[9] = strconv.Itoa(r.Usage.CompletionTokens)
				row[10] = strconv.Itoa(r.Usage.TotalTokens)
				if r.Usage.PromptTokensDetails != nil {
					row[11] = strconv.Itoa(r.Usage.PromptTokensDetails.CachedTokens)
				}
			}
			row[12] = fmtT(rr.boot)
			row[13] = fmtT(ct.dnsStart)
			row[14] = fmtT(ct.dnsDone)
			row[15] = fmtMs(ct.dnsDone.Sub(ct.dnsStart))
			row[16] = fmtT(ct.tcpStart)
			row[17] = fmtT(ct.tcpDone)
			row[18] = fmtMs(ct.tcpDone.Sub(ct.tcpStart))
			row[19] = fmtT(ct.tlsStart)
			row[20] = fmtT(ct.tlsDone)
			row[21] = fmtMs(ct.tlsDone.Sub(ct.tlsStart))
			row[22] = strconv.FormatBool(ct.reused)
			row[23] = fmtT(ct.gotConn)
			row[24] = fmtT(ct.wroteRequest)
			row[25] = fmtT(r.StartedAt)
			row[26] = fmtT(r.FirstTokenAt)
			row[27] = fmtT(r.EndedAt)
			row[28] = strconv.FormatInt(r.StartedAt.Sub(rr.boot).Milliseconds(), 10)
			row[29] = strconv.FormatInt(r.FirstTokenAt.Sub(r.StartedAt).Milliseconds(), 10)
			row[30] = strconv.FormatInt(r.EndedAt.Sub(r.FirstTokenAt).Milliseconds(), 10)
			row[31] = strconv.FormatInt(r.EndedAt.Sub(rr.boot).Milliseconds(), 10)
			row[32] = strconv.FormatInt(rr.probeLatency.Milliseconds(), 10)
			row[33] = fmt.Sprintf("%.1f", rr.tokenRate)
		}
		_ = w.Write(row)
	}
	w.Flush()
	return filename, buf.Bytes()
}

// printConsole 将结果按原 CLI 格式打印到控制台。
func printConsole(results []runResult, channel, model string, thinking bool, promptText string, n int) {
	for _, rr := range results {
		if n > 1 {
			fmt.Printf("\n══════════════ 请求 #%d ══════════════\n", rr.index+1)
		}
		if rr.err != nil {
			fmt.Fprintf(os.Stderr, "请求 #%d 失败: %v\n", rr.index+1, rr.err)
			continue
		}
		result := rr.result
		boot := rr.boot

		fmt.Printf("\n═══════════════════════════════════════════════════\n")
		fmt.Printf("  LLM Bench  启动 %s\n", boot.Format("2006-01-02 15:04:05.000 MST"))
		fmt.Printf("═══════════════════════════════════════════════════\n\n")

		fmt.Printf("【结果】\n")
		fmt.Printf("  模型名称: %s\n", result.Model)
		fmt.Printf("  模型渠道: %s\n", channel)
		fmt.Printf("  Thinking: %v\n", thinking)
		if result.Usage != nil {
			cached := 0
			if result.Usage.PromptTokensDetails != nil {
				cached = result.Usage.PromptTokensDetails.CachedTokens
			}
			fmt.Printf("  Token 总数: %d  (prompt=%d  completion=%d  cached=%d)\n",
				result.Usage.TotalTokens, result.Usage.PromptTokens, result.Usage.CompletionTokens, cached)
			fmt.Printf("  Token 速率: %.1f tok/s\n", rr.tokenRate)
		}
		fmt.Printf("  停止原因: %s\n\n", result.FinishReason)

		fmt.Printf("【Prompt】\n%s\n\n", promptText)
		fmt.Printf("【回复】\n%s\n\n", result.Content)
		if thinking && result.ReasoningContent != "" {
			fmt.Printf("【Thinking】\n%s\n\n", result.ReasoningContent)
		}

		ct := rr.ct
		fmt.Printf("【时间线】\n")
		fmt.Printf("  %-16s %s\n", "启动:", abs(boot))
		if ct.reused {
			fmt.Printf("  %-16s %s  +%s\n", "连接(复用):", abs(ct.gotConn), ms(ct.gotConn.Sub(boot)))
		} else {
			if !ct.dnsStart.IsZero() {
				fmt.Printf("  %-16s %s  +%s  (耗时 %s)\n", "DNS 开始:", abs(ct.dnsStart), ms(ct.dnsStart.Sub(boot)), ms(ct.dnsDone.Sub(ct.dnsStart)))
			}
			if !ct.tcpStart.IsZero() {
				fmt.Printf("  %-16s %s  +%s  (耗时 %s)\n", "TCP 开始:", abs(ct.tcpStart), ms(ct.tcpStart.Sub(boot)), ms(ct.tcpDone.Sub(ct.tcpStart)))
			}
			if !ct.tlsStart.IsZero() {
				fmt.Printf("  %-16s %s  +%s  (耗时 %s)\n", "TLS 开始:", abs(ct.tlsStart), ms(ct.tlsStart.Sub(boot)), ms(ct.tlsDone.Sub(ct.tlsStart)))
			}
			fmt.Printf("  %-16s %s  +%s\n", "获取连接:", abs(ct.gotConn), ms(ct.gotConn.Sub(boot)))
		}
		fmt.Printf("  %-16s %s  +%s\n", "请求发起:", abs(result.StartedAt), ms(result.StartedAt.Sub(boot)))
		fmt.Printf("  %-16s %s  +%s\n", "首 Token:", abs(result.FirstTokenAt), ms(result.FirstTokenAt.Sub(boot)))
		fmt.Printf("  %-16s %s  +%s\n\n", "完成:", abs(result.EndedAt), ms(result.EndedAt.Sub(boot)))
		fmt.Printf("【阶段耗时】\n")
		fmt.Printf("  启动  → 发起:      %s\n", ms(result.StartedAt.Sub(boot)))
		fmt.Printf("  发起  → 首Token:   %s\n", ms(result.FirstTokenAt.Sub(result.StartedAt)))
		fmt.Printf("  首Token→ 完成:     %s\n", ms(result.EndedAt.Sub(result.FirstTokenAt)))
		fmt.Printf("  总耗时(boot→完成): %s\n\n", ms(result.EndedAt.Sub(boot)))
		fmt.Printf("═══════════════════════════════════════════════════\n")
		fmt.Printf("  LLM Bench  结束 %s  耗时 %s\n", result.EndedAt.Format("2006-01-02 15:04:05.000 MST"), ms(result.EndedAt.Sub(boot)))
		fmt.Printf("═══════════════════════════════════════════════════\n")
	}
}

func ms(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func abs(t time.Time) string { return t.Format("15:04:05.000") }

// providerFor 按渠道名返回对应 Provider；未知渠道返回 nil（走内置 OpenAI 解析）。
func providerFor(channel string) llm.Provider {
	switch strings.ToLower(channel) {
	case "nange":
		return nange.Provider{}
	case "qianfan":
		return qianfan.Provider{}
	default:
		return nil
	}
}
