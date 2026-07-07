//go:build embed

// 仅在 `-tags embed` 构建时编译：把 dist/ 整体内嵌进二进制。
// dist/ 由 Taskfile 的 embed 任务从 frontend/dist 拷贝生成，构建前必须存在且非空。

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist 返回内嵌的前端文件系统（已剥离 dist/ 前缀）与 true。
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
