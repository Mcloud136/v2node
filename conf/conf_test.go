package conf

import (
	"os"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	c := New()
	if c.LogConfig.Level != "info" {
		t.Fatalf("Default level: got %q, want %q", c.LogConfig.Level, "info")
	}
	if c.LogConfig.Access != "none" {
		t.Fatalf("Default access: got %q, want %q", c.LogConfig.Access, "none")
	}
}

func TestLoadFromPath(t *testing.T) {
	content := `{
		"Log": {"Level": "debug", "Output": "/tmp/test.log"},
		"Nodes": [
			{"ApiHost": "https://example.com/", "NodeID": 42, "ApiKey": "secret123", "Timeout": 10}
		]
	}`
	tmpFile, err := os.CreateTemp("", "v2node-test-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	c := New()
	if err := c.LoadFromPath(tmpFile.Name()); err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if c.LogConfig.Level != "debug" {
		t.Fatalf("Level: got %q, want %q", c.LogConfig.Level, "debug")
	}
	if len(c.NodeConfigs) != 1 {
		t.Fatalf("NodeConfigs: got %d, want 1", len(c.NodeConfigs))
	}
	nc := c.NodeConfigs[0]
	if nc.APIHost != "https://example.com/" {
		t.Fatalf("ApiHost: got %q", nc.APIHost)
	}
	if nc.NodeID != 42 {
		t.Fatalf("NodeID: got %d, want 42", nc.NodeID)
	}
	if nc.Key != "secret123" {
		t.Fatalf("ApiKey: got %q", nc.Key)
	}
	if nc.Timeout != 10 {
		t.Fatalf("Timeout: got %d, want 10", nc.Timeout)
	}
	if nc.RetryCount == nil || *nc.RetryCount != DefaultNodeRetryCount {
		t.Fatalf("RetryCount: got %v, want %d", nc.RetryCount, DefaultNodeRetryCount)
	}
}

func TestLoadFromPathMissingFile(t *testing.T) {
	c := New()
	err := c.LoadFromPath("/nonexistent/path.json")
	if err == nil {
		t.Fatal("Should have returned error for missing file")
	}
}

func TestLoadFromPathInvalidJSON(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "v2node-test-*.json")
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("not json")
	tmpFile.Close()

	c := New()
	err := c.LoadFromPath(tmpFile.Name())
	if err == nil {
		t.Fatal("Should have returned error for invalid JSON")
	}
}

func TestLoadMultipleNodes(t *testing.T) {
	content := `{
		"Nodes": [
			{"ApiHost": "https://a.com/", "NodeID": 1, "ApiKey": "k1"},
			{"ApiHost": "https://b.com/", "NodeID": 2, "ApiKey": "k2", "RetryCount": 3}
		]
	}`
	tmpFile, _ := os.CreateTemp("", "v2node-test-*.json")
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	c := New()
	if err := c.LoadFromPath(tmpFile.Name()); err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if len(c.NodeConfigs) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(c.NodeConfigs))
	}
	// Node 1 should have default RetryCount
	if c.NodeConfigs[0].RetryCount == nil || *c.NodeConfigs[0].RetryCount != 1 {
		t.Fatalf("Node 0 RetryCount: got %v, want 1", c.NodeConfigs[0].RetryCount)
	}
	// Node 2 should have custom RetryCount
	if c.NodeConfigs[1].RetryCount == nil || *c.NodeConfigs[1].RetryCount != 3 {
		t.Fatalf("Node 1 RetryCount: got %v, want 3", c.NodeConfigs[1].RetryCount)
	}
}
