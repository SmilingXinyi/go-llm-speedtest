// Package config 负责加载 llm.yaml 渠道配置。
//
// 配置文件以渠道为单位组织，每个渠道含 name / base_url / token / models，
// 取代原先散落在 .env 与渠道前缀环境变量中的配置方式。
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Channel 描述一个 LLM 渠道（OpenAI 兼容服务）。
type Channel struct {
	Name    string   `yaml:"name" json:"name"`           // 渠道名，如 nange / qianfan；用于 --channel 选择与 Provider 路由
	BaseURL string   `yaml:"base_url" json:"base_url"`   // 例如 https://api.example.com/v1
	Token   string   `yaml:"token" json:"token"`         // Bearer Token
	Models  []string `yaml:"models" json:"models"`       // 该渠道支持的候选模型列表
}

// Config 是 llm.yaml 的根结构。
type Config struct {
	Channels []Channel `yaml:"channels"`
}

// Load 读取并解析指定路径的 llm.yaml 文件。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: 读取配置文件失败 %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: 解析配置文件失败 %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate 检查渠道配置完整性：每个渠道需有 name/base_url/token 且至少一个模型。
// channels 为空时也视为错误（Load 用于跑基准，必须至少有一个渠道）。
func (c *Config) validate() error {
	if len(c.Channels) == 0 {
		return errors.New("config: 未配置任何渠道（channels 为空）")
	}
	return c.validateChannels()
}

// validateChannels 校验每个渠道字段完整性，但允许 0 个渠道。
// 供 ListChannels 等仅用于「列举」的场景：空配置是合法的「尚未配置」状态。
func (c *Config) validateChannels() error {
	seen := make(map[string]struct{}, len(c.Channels))
	for i, ch := range c.Channels {
		if strings.TrimSpace(ch.Name) == "" {
			return fmt.Errorf("config: 第 %d 个渠道缺少 name", i+1)
		}
		if _, dup := seen[ch.Name]; dup {
			return fmt.Errorf("config: 渠道名重复: %s", ch.Name)
		}
		seen[ch.Name] = struct{}{}
		if ch.BaseURL == "" {
			return fmt.Errorf("config: 渠道 %s 缺少 base_url", ch.Name)
		}
		if ch.Token == "" {
			return fmt.Errorf("config: 渠道 %s 缺少 token", ch.Name)
		}
		if len(ch.Models) == 0 {
			return fmt.Errorf("config: 渠道 %s 缺少 models", ch.Name)
		}
	}
	return nil
}

// Channel 按渠道名查找；找不到时返回错误。
func (c *Config) Channel(name string) (*Channel, error) {
	for i := range c.Channels {
		if strings.EqualFold(c.Channels[i].Name, name) {
			return &c.Channels[i], nil
		}
	}
	return nil, fmt.Errorf("config: 未找到渠道 %q（请在 llm.yaml 中配置）", name)
}

// HasModel 判断 model 是否在该渠道的候选列表中。
// 空渠道模型列表视为允许任意 model（兼容性兜底，正常配置不会触发）。
func (ch *Channel) HasModel(model string) bool {
	if len(ch.Models) == 0 {
		return true
	}
	for _, m := range ch.Models {
		if m == model {
			return true
		}
	}
	return false
}

// ListChannels 读取 path 指向的 llm.yaml，返回全部渠道。
// 容忍 channels 为空（返回空切片）：尚未配置任何渠道是合法状态，供配置页从零创建。
// 文件缺失、解析失败或单个渠道字段不完整仍返回错误。
func ListChannels(path string) ([]Channel, error) {
	cfg, err := loadLenient(path)
	if err != nil {
		return nil, err
	}
	return cfg.Channels, nil
}

// loadLenient 读取并解析 llm.yaml，校验每个渠道字段完整性但允许 0 个渠道。
// 与 Load（严格，要求至少一个渠道，供跑基准用）的区别仅在空配置的处理。
func loadLenient(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: 读取配置文件失败 %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: 解析配置文件失败 %s: %w", path, err)
	}
	if err := cfg.validateChannels(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// EnsureFile 确保 path 指向的 llm.yaml 存在；不存在则创建父目录并写入空模板
// （channels: []，含注释说明），权限 0o600。已存在则不做任何改动。
// 供 server 启动时调用，使配置页可从零创建渠道而非因文件缺失报错。
func EnsureFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: 创建配置目录失败: %w", err)
		}
	}
	tmpl := []byte("# LLM Studio 渠道配置。在配置页添加渠道后会自动写回本文件。\nchannels: []\n")
	if err := os.WriteFile(path, tmpl, 0o600); err != nil {
		return fmt.Errorf("config: 创建配置文件失败: %w", err)
	}
	return nil
}

// AddChannel 向 path 指向的 llm.yaml 追加一个渠道并原子写回。
// 渠道名已存在（不区分大小写）或新渠道字段不完整时返回错误，文件不会被修改。
//
// 通过 yaml.Node 操作节点树而非结构体往返，从而保留原文件的注释。
func AddChannel(path string, ch Channel) error {
	doc, seq, indent, err := loadChannelsSeq(path)
	if err != nil {
		return err
	}
	if findChannelIndex(seq, ch.Name) >= 0 {
		return fmt.Errorf("config: 渠道 %s 已存在", ch.Name)
	}
	seq.Content = append(seq.Content, channelToNode(ch))

	var cfg Config
	if err := doc.Decode(&cfg); err != nil {
		return fmt.Errorf("config: 校验失败: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	return writeNode(path, doc, indent)
}

// RemoveChannel 从 path 指向的 llm.yaml 删除指定渠道并原子写回。
// 渠道不存在（不区分大小写）时返回错误，文件不会被修改。
//
// 通过 yaml.Node 操作节点树而非结构体往返，从而保留原文件的注释。
// 删除首项时，会将其头部注释迁移到新的首项，避免丢失渠道上方的说明注释。
func RemoveChannel(path, name string) error {
	doc, seq, indent, err := loadChannelsSeq(path)
	if err != nil {
		return err
	}
	idx := findChannelIndex(seq, name)
	if idx < 0 {
		return fmt.Errorf("config: 渠道 %s 不存在", name)
	}
	removed := seq.Content[idx]
	seq.Content = append(seq.Content[:idx], seq.Content[idx+1:]...)
	// 首项被删时，把它的头部注释迁移到新的首项，保留列表上方的说明注释。
	if idx == 0 && len(seq.Content) > 0 {
		first := seq.Content[0]
		if removed.HeadComment != "" {
			if first.HeadComment != "" {
				first.HeadComment = removed.HeadComment + "\n" + first.HeadComment
			} else {
				first.HeadComment = removed.HeadComment
			}
		}
	}

	var cfg Config
	if err := doc.Decode(&cfg); err != nil {
		return fmt.Errorf("config: 校验失败: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	return writeNode(path, doc, indent)
}

// loadChannelsSeq 读取 path 并返回文档节点、channels 序列节点与原文缩进。
// 文件无 channels 键时自动创建空序列，便于 AddChannel 在空配置上首次写入。
// indent 取自原文最浅一层序列项的缩进，回写时用于保持原文件缩进风格；无法识别时默认 2。
func loadChannelsSeq(path string) (*yaml.Node, *yaml.Node, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("config: 读取配置文件失败 %s: %w", path, err)
	}
	indent := detectIndent(data)
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, 0, fmt.Errorf("config: 解析配置文件失败 %s: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		// 空文档：构造一个空的根 mapping，再补 channels 键。
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{root}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, 0, fmt.Errorf("config: 顶层结构不是 mapping")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "channels" {
			seq := root.Content[i+1]
			if seq.Kind != yaml.SequenceNode {
				return nil, nil, 0, fmt.Errorf("config: channels 不是列表")
			}
			// 空的 flow 序列（如模板里的 channels: []）回写时改用 block 风格，
			// 使首个渠道写入后为多行块格式，便于人工阅读。
			if len(seq.Content) == 0 {
				seq.Style = 0
			}
			return &doc, seq, indent, nil
		}
	}
	// 未找到 channels：追加键与空序列。
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "channels"}
	seqNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	root.Content = append(root.Content, keyNode, seqNode)
	return &doc, seqNode, indent, nil
}

// detectIndent 从原文统计最浅一层序列项（`- ` 开头）的缩进空格数。
// 用于回写时保持原文件缩进风格；识别不到时返回 2（YAML 常见默认）。
func detectIndent(data []byte) int {
	min := 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "-" || strings.HasPrefix(trimmed, "- ") {
			n := len(line) - len(trimmed)
			if n > 0 && (min == 0 || n < min) {
				min = n
			}
		}
	}
	if min == 0 {
		return 2
	}
	return min
}

// findChannelIndex 在 channels 序列中按 name（不区分大小写）查找渠道索引，未找到返回 -1。
func findChannelIndex(seq *yaml.Node, name string) int {
	for i, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			if item.Content[j].Value == "name" {
				if strings.EqualFold(item.Content[j+1].Value, name) {
					return i
				}
				break
			}
		}
	}
	return -1
}

// channelToNode 把一个 Channel 构造为 YAML mapping 节点（含 name/base_url/token/models）。
// 标量使用 plain 风格，与示例文件一致；encoder 会在值需要时自动加引号。
func channelToNode(ch Channel) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	kv := func(k, v string) {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
		)
	}
	kv("name", ch.Name)
	kv("base_url", ch.BaseURL)
	kv("token", ch.Token)
	models := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, mo := range ch.Models {
		models.Content = append(models.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mo},
		)
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "models"},
		models,
	)
	return m
}

// writeNode 将文档节点序列化为 YAML 并原子写回 path：先写同目录临时文件再 rename，
// 避免写中途崩溃留下半截文件。文件权限 0o600（含 token 等敏感信息）。
// indent 控制输出缩进，沿用原文件风格（由 detectIndent 识别）。
func writeNode(path string, doc *yaml.Node, indent int) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indent)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("config: 序列化配置失败: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("config: 序列化配置失败: %w", err)
	}
	return atomicWrite(path, buf.Bytes())
}

// atomicWrite 将 data 原子写入 path（临时文件 + Sync + Rename）。
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".llm-yaml-*")
	if err != nil {
		return fmt.Errorf("config: 创建临时文件失败: %w", err)
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("config: 写入临时文件失败: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("config: 同步临时文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("config: 关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		cleanup()
		return fmt.Errorf("config: 设置文件权限失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("config: 替换配置文件失败: %w", err)
	}
	return nil
}
