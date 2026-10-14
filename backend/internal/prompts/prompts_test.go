package prompts

import (
	"path/filepath"
	"testing"
)

func TestStoreCRUD(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "sub", "prompts.json"))

	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("missing file should list empty, got %v, %v", list, err)
	}
	if err := s.Ensure(); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List()
	if len(list) != len(DefaultTemplates()) {
		t.Fatalf("want %d defaults, got %d", len(DefaultTemplates()), len(list))
	}
	if err := s.Ensure(); err != nil { // 已存在不应覆盖
		t.Fatal(err)
	}

	if err := s.Add(Template{Name: "x", SystemPrompt: "sys"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Template{Name: "X", Prompt: "u"}); err == nil {
		t.Fatal("duplicate name (case-insensitive) should fail")
	}
	if err := s.Add(Template{Name: "empty"}); err == nil {
		t.Fatal("template with neither prompt should fail")
	}
	if err := s.Add(Template{Name: " ", Prompt: "u"}); err == nil {
		t.Fatal("blank name should fail")
	}

	if err := s.Update("x", Template{Name: "y", Prompt: "u2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update("x", Template{Name: "z", Prompt: "u"}); err == nil {
		t.Fatal("updating missing template should fail")
	}
	if err := s.Update("y", Template{Name: DefaultTemplates()[0].Name, Prompt: "u"}); err == nil {
		t.Fatal("renaming onto existing template should fail")
	}

	if err := s.Remove("y"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("y"); err == nil {
		t.Fatal("removing missing template should fail")
	}
	list, _ = s.List()
	if len(list) != len(DefaultTemplates()) {
		t.Fatalf("want %d templates after cleanup, got %d", len(DefaultTemplates()), len(list))
	}
}
