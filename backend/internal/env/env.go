// Package env 提供 .env 文件加载：解析 KEY=VALUE 格式并注入环境变量（不覆盖已有值）。
// 供 CLI（bench）与 Server 两个入口复用，保证环境变量来源行为一致。
package env

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv 解析 KEY=VALUE 格式的 .env 文件并注入环境变量（不覆盖已有值）。
// 文件不存在时静默返回 nil。
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if n := len(val); n >= 2 {
			if (val[0] == '"' && val[n-1] == '"') || (val[0] == '\'' && val[n-1] == '\'') {
				val = val[1 : n-1]
			}
		}
		if _, ok := os.LookupEnv(key); !ok {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}
