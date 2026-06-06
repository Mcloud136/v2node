package format

import "testing"

func TestUserTag(t *testing.T) {
	tag := UserTag("trojan", "abc-123")
	expected := "trojan|abc-123"
	if tag != expected {
		t.Fatalf("UserTag: got %q, want %q", tag, expected)
	}
}

func TestUserTagEmpty(t *testing.T) {
	tag := UserTag("", "")
	if tag != "|" {
		t.Fatalf("UserTag empty: got %q, want %q", tag, "|")
	}
}

func TestUserTagConsistency(t *testing.T) {
	// Same inputs should produce same output
	a := UserTag("tag1", "uuid1")
	b := UserTag("tag1", "uuid1")
	if a != b {
		t.Fatal("UserTag should be deterministic")
	}
	// Different inputs should produce different output
	c := UserTag("tag1", "uuid2")
	if a == c {
		t.Fatal("Different UUIDs should produce different tags")
	}
}
