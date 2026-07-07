// Command cli 精确拆解一次流式 LLM 请求的各阶段耗时。
//
// 用法：
//
//	go run ./cmd/cli --channel qianfan --model ernie-4.0-8k --prompt "用一句话介绍你自己"
//
// 渠道配置从 llm.yaml 读取（默认 conf/llm.yaml，可用 --config 覆盖）。
//
// 实际逻辑在 internal/cli，本入口仅解析参数并委托。
package main

import (
	"fmt"
	"os"

	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}
