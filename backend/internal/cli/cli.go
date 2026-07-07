// Package cli 封装「单次基准测试」命令行逻辑：解析 flag 与环境变量、加载配置、
// 调用 internal/services 发起一次 Bench 并输出（控制台表格或 CSV 文件）。
//
// 既被 cmd/cli 直接使用，也被统一入口 cmd/llm-studio 的 bench 子命令复用，
// 保证两个入口的 CLI 行为完全一致。
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/config"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/env"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/services"
)

// Run 执行一次 CLI 基准测试。args 为去掉子命令名后的参数（如 os.Args[2:]）。
// 错误以 error 返回，由调用方决定退出码与 stderr 输出。
func Run(args []string) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	configPath := fs.String("config", "conf/llm.yaml", "llm.yaml 配置文件路径")
	channel := fs.String("channel", "", "模型渠道")
	model := fs.String("model", "", "模型名（省略时取渠道 models[0]）")
	prompt := fs.String("prompt", "用一句话介绍你自己", "prompt（支持 file://path 加载文件）")
	thinking := fs.Bool("thinking", false, "开启 thinking 模式（默认关闭）")
	concurrency := fs.Int("concurrency", 1, "并发请求数量")
	out := fs.String("out", "", "输出格式：csv 输出到文件，默认控制台打印")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// 默认自动加载 cwd 下的 .env（存在则注入环境变量，不覆盖已有值；不存在则跳过）。
	_ = env.LoadDotEnv(".env")

	ch := flagOrEnv(*channel, "LLM_CHANNEL")
	if ch == "" {
		return fmt.Errorf("需要指定 --channel 或环境变量 LLM_CHANNEL")
	}

	// base_url / token / model 走环境变量解析（配置层）。
	cfgBaseURL := channelEnv(ch, "BASE_URL")
	cfgToken := channelEnv(ch, "TOKEN")
	cfgModel := firstNonEmpty(*model, channelEnv(ch, "MODEL"))

	// prompt 支持 file://path 从文件加载。
	promptText := *prompt
	if strings.HasPrefix(promptText, "file://") {
		data, err := os.ReadFile(strings.TrimPrefix(promptText, "file://"))
		if err != nil {
			return fmt.Errorf("读取 prompt 文件失败: %w", err)
		}
		promptText = strings.TrimSpace(string(data))
	}

	// 提供了完整配置则直接构造渠道；否则回退到 llm.yaml，由 services 按渠道名解析。
	var cfg *config.Config
	if cfgBaseURL != "" && cfgToken != "" && cfgModel != "" {
		cfg = &config.Config{Channels: []config.Channel{{
			Name: ch, BaseURL: cfgBaseURL, Token: cfgToken, Models: []string{cfgModel},
		}}}
	} else {
		c, err := config.Load(*configPath)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}
		cfg = c
	}

	svc := services.New(cfg)
	return svc.Run(context.Background(), services.RunOptions{
		Channel:     ch,
		Model:       cfgModel,
		Prompt:      promptText,
		Thinking:    *thinking,
		Concurrency: *concurrency,
		Out:         *out,
	})
}

// flagOrEnv 返回 flag 值；为空时读环境变量 envKey。
func flagOrEnv(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}

// channelEnv 按渠道前缀读取配置，回退到通用 LLM_* 变量。
// 例如 channel=nange 时读 NANGE_LLM_BASE_URL，回退 LLM_BASE_URL。
func channelEnv(channel, suffix string) string {
	if channel != "" {
		if v := os.Getenv(strings.ToUpper(channel) + "_LLM_" + suffix); v != "" {
			return v
		}
	}
	return os.Getenv("LLM_" + suffix)
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
