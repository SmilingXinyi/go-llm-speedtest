// Command server 提供 LLM Speed Test 的 HTTP 服务：渠道配置（llm.yaml）CRUD API，
// 并可选地托管前端构建产物（单进程同源提供页面与 API）。
//
// 用法：
//
//	go run ./cmd/server --config ./llm.yaml --static ../frontend/dist
//
// 开发时前端走 vite dev server（5173），通过 vite proxy 把 /api 转发到本服务（:8787）。
//
// 实际 HTTP 逻辑在 internal/server，本入口仅解析 flag 并委托。
package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/labstack/echo/v5"

	"github.com/SmilingXinyi/go-llm-speedtest/backend/internal/env"
	"github.com/SmilingXinyi/go-llm-speedtest/backend/internal/server"
)

func main() {
	// 默认自动加载 cwd 下的 .env（存在则注入环境变量，不覆盖已有值；不存在则跳过）。
	_ = env.LoadDotEnv(".env")

	addr := flag.String("addr", ":8787", "监听地址")
	cfgPath := flag.String("config", "conf/llm.yaml", "llm.yaml 配置文件路径")
	staticDir := flag.String("static", "../frontend/dist", "前端构建产物目录；为空或不存在则仅提供 API")
	historyDir := flag.String("history", "history", "基准测试结果 CSV 的存放目录")
	flag.Parse()

	abs := func(p string) string {
		if p == "" {
			return ""
		}
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}

	s := server.New(server.Options{
		ConfigPath: *cfgPath,
		StaticDir:  abs(*staticDir),
		HistoryDir: abs(*historyDir),
	})

	e := echo.New()
	s.Register(e)
	log.Printf("LLM Speed Test server listening on %s (config=%s, static=%s, history=%s)", *addr, *cfgPath, abs(*staticDir), abs(*historyDir))
	if err := e.Start(*addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
