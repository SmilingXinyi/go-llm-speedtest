// Package server 提供 LLM Studio 的 HTTP 服务：渠道配置（llm.yaml）CRUD API、
// 基准测试与历史记录 API，并可选托管前端构建产物（内嵌 embed.FS 或磁盘静态目录）。
//
// 既被 cmd/server 直接使用，也被统一入口 cmd/llm-studio 的 server 子命令复用，
// 保证两个入口的 HTTP 行为完全一致。
package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/labstack/echo/v5"

	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/config"
	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/services"
)

// Options 是构造 Server 的参数。
type Options struct {
	ConfigPath string // llm.yaml 路径
	StaticDir  string // 前端磁盘构建产物目录；为空或不存在的目录则忽略
	HistoryDir string // 基准测试结果 CSV 的存放目录
	EmbedFS    fs.FS  // 内嵌前端文件系统（如 web.Dist() 命中）；非 nil 时优先于 StaticDir
}

// Server 持有运行期配置，所有 handler 共用同一 llm.yaml 路径、静态来源与历史目录。
type Server struct {
	cfgPath    string
	staticDir  string
	historyDir string
	embedFS    fs.FS // 非 nil 时优先于磁盘静态目录
}

// New 构造一个 Server。HistoryDir 不存在时会尝试创建；配置文件不存在时会自动创建空模板。
func New(opts Options) *Server {
	if opts.HistoryDir != "" {
		if err := os.MkdirAll(opts.HistoryDir, 0o755); err != nil {
			log.Printf("警告: 创建历史目录 %s 失败: %v", opts.HistoryDir, err)
		}
	}
	if opts.ConfigPath != "" {
		if err := config.EnsureFile(opts.ConfigPath); err != nil {
			log.Printf("警告: 创建配置文件 %s 失败: %v", opts.ConfigPath, err)
		}
	}
	s := &Server{
		cfgPath:    opts.ConfigPath,
		staticDir:  opts.StaticDir,
		historyDir: opts.HistoryDir,
		embedFS:    opts.EmbedFS,
	}
	// 内嵌前端要求根目录存在 index.html，否则视为无前端（降级到磁盘 / 纯 API）。
	if s.embedFS != nil {
		if f, err := s.embedFS.Open("index.html"); err != nil {
			s.embedFS = nil
		} else {
			f.Close()
		}
	}
	return s
}

// Register 在给定 echo 实例上注册 /api/* 路由与前端静态兜底。
// API 路由先注册，优先级高于 /* 静态兜底。
func (s *Server) Register(e *echo.Echo) {
	e.GET("/api/channels", s.listChannels)
	e.POST("/api/channels", s.addChannel)
	e.DELETE("/api/channels/:name", s.removeChannel)
	e.POST("/api/bench", s.runBench)
	e.GET("/api/history", s.listHistory)
	e.GET("/api/history/:name", s.getHistory)

	if s.embedFS != nil {
		e.GET("/*", s.embedStatic)
	} else if s.staticDir != "" {
		if info, err := os.Stat(s.staticDir); err != nil || !info.IsDir() {
			log.Printf("静态目录 %s 不存在或不是目录，仅提供 API 模式", s.staticDir)
		} else {
			e.GET("/*", s.spaStatic)
		}
	}
}

// Start 监听 addr 并启动 HTTP 服务。
func (s *Server) Start(addr string) error {
	src := "embed"
	if s.embedFS == nil {
		src = s.staticDir
	}
	log.Printf("LLM Studio server listening on %s (config=%s, static=%s, history=%s)", addr, s.cfgPath, src, s.historyDir)
	return s.start(addr)
}

// start 单独抽出以便测试覆盖而不真正监听；此处即 echo.Start。
func (s *Server) start(addr string) error {
	e := echo.New()
	s.Register(e)
	return e.Start(addr)
}

// fail 记录一次 API 错误到服务端日志（方法 路径 → 状态码: 错误），并以 JSON 返回给前端。
func (s *Server) fail(c *echo.Context, code int, err error) error {
	log.Printf("%s %s -> %d: %v", c.Request().Method, c.Request().URL.Path, code, err)
	return c.JSON(code, map[string]string{"error": err.Error()})
}

// listChannels GET /api/channels —— 返回全部渠道。
func (s *Server) listChannels(c *echo.Context) error {
	chs, err := config.ListChannels(s.cfgPath)
	if err != nil {
		return s.fail(c, http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, chs)
}

// addChannel POST /api/channels —— 追加一个渠道并写回 llm.yaml。
func (s *Server) addChannel(c *echo.Context) error {
	var ch config.Channel
	if err := c.Bind(&ch); err != nil {
		return s.fail(c, http.StatusBadRequest, fmt.Errorf("请求体解析失败: %w", err))
	}
	if err := config.AddChannel(s.cfgPath, ch); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "已存在") {
			code = http.StatusConflict
		}
		return s.fail(c, code, err)
	}
	return c.JSON(http.StatusCreated, ch)
}

// removeChannel DELETE /api/channels/:name —— 删除指定渠道并写回 llm.yaml。
func (s *Server) removeChannel(c *echo.Context) error {
	name := c.Param("name")
	if err := config.RemoveChannel(s.cfgPath, name); err != nil {
		code := http.StatusNotFound
		if !strings.Contains(err.Error(), "不存在") {
			code = http.StatusBadRequest
		}
		return s.fail(c, code, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// benchRequest 是 /api/bench 的请求体。
type benchRequest struct {
	Channel     string `json:"channel"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Thinking    bool   `json:"thinking"`
	Concurrency int    `json:"concurrency"`
}

// benchResponse 是 /api/bench 的响应体：文件名 + CSV 内容。
// 前端按 CSV 渲染（与 Viewer 一致），文件名用于历史定位。
type benchResponse struct {
	Filename string `json:"filename"`
	CSV      string `json:"csv"`
}

// runBench POST /api/bench —— 复用 internal/services 的 Bench 逻辑发起一次基准测试，
// 将结果 CSV 写入 history 目录并返回内容，前端按 CSV 渲染（与 Viewer 对齐）。
// 每次请求重新加载 llm.yaml，保证渠道配置为最新。
func (s *Server) runBench(c *echo.Context) error {
	var req benchRequest
	if err := c.Bind(&req); err != nil {
		return s.fail(c, http.StatusBadRequest, fmt.Errorf("请求体解析失败: %w", err))
	}
	if req.Channel == "" {
		return s.fail(c, http.StatusBadRequest, fmt.Errorf("channel 不能为空"))
	}

	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		return s.fail(c, http.StatusInternalServerError, err)
	}
	svc := services.New(cfg)

	filename, data, err := svc.RunCSV(c.Request().Context(), services.RunOptions{
		Channel:     req.Channel,
		Model:       req.Model,
		Prompt:      req.Prompt,
		Thinking:    req.Thinking,
		Concurrency: req.Concurrency,
	}, s.historyDir)
	if err != nil {
		// RunCSV 仅在渠道/模型校验失败或写文件失败时返回错误（单请求错误已落在各行 error 列）。
		return s.fail(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, benchResponse{Filename: filename, CSV: string(data)})
}

// historyItem 是 /api/history 列表项。
type historyItem struct {
	Filename string `json:"filename"`
	Time     string `json:"time"`
}

// listHistory GET /api/history —— 列出 history 目录下的 CSV 文件，按文件名倒序（最新在前）。
func (s *Server) listHistory(c *echo.Context) error {
	entries, err := os.ReadDir(s.historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusOK, []historyItem{})
		}
		return s.fail(c, http.StatusInternalServerError, err)
	}
	items := make([]historyItem, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".csv") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, historyItem{Filename: e.Name(), Time: info.ModTime().Format("2006-01-02 15:04:05")})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Filename > items[j].Filename })
	return c.JSON(http.StatusOK, items)
}

// getHistory GET /api/history/:name —— 返回指定历史 CSV 文件的内容（text/plain）。
func (s *Server) getHistory(c *echo.Context) error {
	name := c.Param("name")
	// 仅允许 .csv 文件名，禁止路径分隔符，防穿越。
	if name == "" || !strings.HasSuffix(name, ".csv") || strings.ContainsAny(name, `/\`) {
		return s.fail(c, http.StatusBadRequest, fmt.Errorf("无效的文件名"))
	}
	data, err := os.ReadFile(filepath.Join(s.historyDir, name))
	if err != nil {
		return s.fail(c, http.StatusNotFound, fmt.Errorf("历史记录 %s 不存在", name))
	}
	return c.String(http.StatusOK, string(data))
}

// spaStatic 提供磁盘上的前端静态资源；请求的不是真实文件时回退到 index.html，
// 使 /config 等前端路由在刷新或直连时不会 404。
// 用 http.ServeFile + 绝对路径，绕开 echo v5 c.File 的 fs.FS 对 .. / 绝对路径的限制。
func (s *Server) spaStatic(c *echo.Context) error {
	urlPath := c.Request().URL.Path
	clean := filepath.Clean("/" + urlPath) // 形如 /assets/index-xxx.js
	full := filepath.Join(s.staticDir, clean)

	// 防路径穿越：解析后必须仍在静态目录内；目录或不存在则回退 index.html（SPA 路由）。
	if !strings.HasPrefix(full, s.staticDir) {
		full = filepath.Join(s.staticDir, "index.html")
	} else if info, err := os.Stat(full); err != nil || info.IsDir() {
		full = filepath.Join(s.staticDir, "index.html")
	}
	http.ServeFile(c.Response(), c.Request(), full)
	return nil
}

// embedStatic 提供内嵌前端静态资源；请求的不是真实文件时回退到 index.html（SPA 路由）。
func (s *Server) embedStatic(c *echo.Context) error {
	urlPath := c.Request().URL.Path
	clean := strings.TrimPrefix(filepath.Clean("/"+urlPath), "/") // 形如 assets/index-xxx.js
	if clean == "" || clean == "." {
		clean = "index.html"
	}

	if f, err := s.embedFS.Open(clean); err == nil {
		info, _ := f.Stat()
		f.Close()
		if info != nil && !info.IsDir() {
			http.ServeFileFS(c.Response(), c.Request(), s.embedFS, clean)
			return nil
		}
	}
	// 回退到 index.html，保证前端路由刷新不 404。
	http.ServeFileFS(c.Response(), c.Request(), s.embedFS, "index.html")
	return nil
}
