// Command llm-studio 是 LLM Studio 的统一入口：以子命令同时支持 HTTP 服务与单次基准测试，
// 并可选把前端构建产物内嵌进同一二进制（`-tags embed` 构建）。
//
// 用法：
//
//	# 默认子命令为 server（裸跑即起 HTTP 服务）
//	llm-studio                      # 等价 llm-studio server
//	llm-studio server --addr :8787  # 起 HTTP 服务（内嵌前端或 --static 磁盘）
//	llm-studio bench --channel qianfan --model ernie-4.0-8k --prompt "用一句话介绍你自己"
//
// 内嵌前端需以 `-tags embed` 构建（Taskfile 的 task build 会先构建前端并拷贝 dist）；
// 否则 server 仅提供 API，或通过 --static 指向前端磁盘构建产物。
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/cli"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/env"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/server"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/web"
)

func main() {
	// 第一个非 flag 参数视为子命令；省略时默认 server（裸跑即起服务）。
	cmd := "server"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	switch cmd {
	case "server":
		runServer(args)
	case "bench", "cli":
		if err := cli.Run(args); err != nil {
			fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

// runServer 解析 server flag 并启动 HTTP 服务。
// 前端来源优先级：--static 磁盘目录 > 内嵌前端（-tags embed）> 纯 API。
func runServer(args []string) {
	// 默认自动加载 cwd 下的 .env（存在则注入环境变量，不覆盖已有值；不存在则跳过）。
	_ = env.LoadDotEnv(".env")

	fset := flag.NewFlagSet("server", flag.ExitOnError)
	addr := fset.String("addr", ":8787", "监听地址")
	cfgPath := fset.String("config", "conf/llm.yaml", "llm.yaml 配置文件路径")
	staticDir := fset.String("static", "", "前端构建产物目录；为空时用内嵌前端（若有）或仅提供 API")
	historyDir := fset.String("history", "history", "基准测试结果 CSV 的存放目录")
	fset.Parse(args)

	abs := func(p string) string {
		if p == "" {
			return ""
		}
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}

	// 未显式指定 --static 时，尝试使用内嵌前端（仅 -tags embed 构建命中）。
	var embedFS fs.FS
	if *staticDir == "" {
		if fsys, ok := web.Dist(); ok {
			embedFS = fsys
		}
	}

	s := server.New(server.Options{
		ConfigPath: *cfgPath,
		StaticDir:  abs(*staticDir),
		HistoryDir: abs(*historyDir),
		EmbedFS:    embedFS,
	})
	if err := s.Start(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`llm-studio — LLM Studio 统一入口

用法:
  llm-studio [server|bench] [flags]

子命令:
  server              启动 HTTP 服务（默认；裸跑即等价此子命令）
  bench               执行一次基准测试（等价旧 cmd/cli）

server flags:
  --addr string       监听地址（默认 :8787）
  --config string     llm.yaml 路径（默认 conf/llm.yaml）
  --static string     前端磁盘构建产物目录；省略时用内嵌前端（-tags embed 构建）或仅 API
  --history string    基准测试结果 CSV 存放目录（默认 history）

bench flags:
  --config string     llm.yaml 路径（默认 conf/llm.yaml）
  --channel string    模型渠道（必填，或环境变量 LLM_CHANNEL）
  --model string      模型名（省略取渠道 models[0]）
  --prompt string     prompt（支持 file://path 加载文件）
  --thinking          开启 thinking 模式
  --concurrency int   并发请求数量（默认 1）
  --out string        输出格式：csv 写文件，默认控制台打印
`)
}
