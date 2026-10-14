// Package prompts 管理提示词模板：每个模板含名称，以及可选的 System Prompt 与 User Prompt
// （二者至少填一个，即支持「单一 System Prompt」或「单一 User Prompt」的模板）。
// 模板以 JSON 文件持久化，写入采用临时文件 + rename 的原子方式。
package prompts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Template 是一个提示词模板。
type Template struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
	Prompt       string `json:"prompt"`
}

// Store 是基于单个 JSON 文件的模板存储，方法并发安全。
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore 返回使用 path 作为存储文件的 Store。
func NewStore(path string) *Store { return &Store{path: path} }

// DefaultTemplates 是首次启动（存储文件不存在）时写入的常用模板。
func DefaultTemplates() []Template {
	return []Template{
		{Name: "通用助手", SystemPrompt: "你是一个乐于助人的 AI 助手，请用准确、简洁的语言回答用户的问题。"},
		{Name: "简洁回答", SystemPrompt: "请用不超过两句话回答，不要添加多余的解释。"},
		{Name: "自我介绍", Prompt: "用一句话介绍你自己"},
		{Name: "长文生成", Prompt: "请写一篇约 800 字的短文，主题是人工智能如何改变软件开发。"},
	}
}

// Ensure 在存储文件不存在时写入默认模板；已存在则不做任何改动。
func (s *Store) Ensure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path); err == nil {
		return nil
	}
	return s.save(DefaultTemplates())
}

// List 返回全部模板；文件不存在时返回空列表。
func (s *Store) List() ([]Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Add 新增模板；名称（不区分大小写）重复时返回错误。
func (s *Store) Add(t Template) error {
	if err := validate(&t); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	if indexOf(list, t.Name) >= 0 {
		return fmt.Errorf("prompts: 模板 %s 已存在", t.Name)
	}
	return s.save(append(list, t))
}

// Update 以 oldName 定位并替换模板，允许改名，但新名称不能与其它模板重复。
func (s *Store) Update(oldName string, t Template) error {
	if err := validate(&t); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	idx := indexOf(list, oldName)
	if idx < 0 {
		return fmt.Errorf("prompts: 模板 %s 不存在", oldName)
	}
	if other := indexOf(list, t.Name); other >= 0 && other != idx {
		return fmt.Errorf("prompts: 模板 %s 已存在", t.Name)
	}
	list[idx] = t
	return s.save(list)
}

// Remove 删除指定模板。
func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	idx := indexOf(list, name)
	if idx < 0 {
		return fmt.Errorf("prompts: 模板 %s 不存在", name)
	}
	return s.save(append(list[:idx], list[idx+1:]...))
}

func validate(t *Template) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errors.New("prompts: 模板缺少名称")
	}
	if strings.TrimSpace(t.SystemPrompt) == "" && strings.TrimSpace(t.Prompt) == "" {
		return errors.New("prompts: 系统提示词与提示词至少填写一个")
	}
	return nil
}

func indexOf(list []Template, name string) int {
	for i, t := range list {
		if strings.EqualFold(t.Name, name) {
			return i
		}
	}
	return -1
}

func (s *Store) load() ([]Template, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Template{}, nil
		}
		return nil, fmt.Errorf("prompts: 读取模板文件失败 %s: %w", s.path, err)
	}
	list := []Template{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return list, nil
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("prompts: 解析模板文件失败 %s: %w", s.path, err)
	}
	return list, nil
}

func (s *Store) save(list []Template) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("prompts: 序列化模板失败: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("prompts: 创建目录失败: %w", err)
	}
	f, err := os.CreateTemp(dir, ".prompts-*")
	if err != nil {
		return fmt.Errorf("prompts: 创建临时文件失败: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("prompts: 写入临时文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("prompts: 关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("prompts: 设置文件权限失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("prompts: 替换模板文件失败: %w", err)
	}
	return nil
}
