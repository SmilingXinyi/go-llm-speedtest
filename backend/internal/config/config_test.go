package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm.yaml")
	initial := "# channel list\nchannels:\n  - name: old\n    base_url: https://old.example/v1\n    token: old-token\n    models:\n      - old-model\n  - name: keep\n    base_url: https://keep.example/v1\n    token: keep-token\n    models:\n      - keep-model\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	updated := Channel{Name: "new", BaseURL: "https://new.example/v1", Token: "new-token", Models: []string{"model-a", "model-b"}}
	if err := UpdateChannel(path, "old", updated); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	channels, err := ListChannels(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 || channels[0].Name != "new" || channels[0].BaseURL != updated.BaseURL || strings.Join(channels[0].Models, ",") != "model-a,model-b" {
		t.Fatalf("updated channels = %#v", channels)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# channel list") {
		t.Fatalf("update dropped document comment: %s", data)
	}
}

func TestUpdateChannelRejectsDuplicateName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm.yaml")
	if err := os.WriteFile(path, []byte("channels:\n  - name: one\n    base_url: https://one\n    token: token\n    models: [one]\n  - name: two\n    base_url: https://two\n    token: token\n    models: [two]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := UpdateChannel(path, "one", Channel{Name: "two", BaseURL: "https://new", Token: "token", Models: []string{"new"}})
	if err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("UpdateChannel() error = %v, want duplicate error", err)
	}
}
