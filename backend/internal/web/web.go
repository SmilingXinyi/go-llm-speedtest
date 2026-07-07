//go:build !embed

// Package web 暴露前端构建产物。默认（无 embed build tag）不内嵌任何资源，
// 仅提供 API / 磁盘静态模式；以 `-tags embed` 构建时由 web_embed.go 内嵌 dist/。
package web

import "io/fs"

// Dist 返回内嵌的前端文件系统与是否可用。
// 非 embed 构建恒为 (nil, false)，调用方应回退到 --static 磁盘目录或纯 API 模式。
func Dist() (fs.FS, bool) {
	return nil, false
}
