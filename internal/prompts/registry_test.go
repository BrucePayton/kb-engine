package prompts

import "testing"

func TestRegistryEntriesAreUniqueAndPopulated(t *testing.T) {
	entries := RegisterAll()
	if len(entries) == 0 {
		t.Fatal("empty prompt registry")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Name == "" || entry.Content == "" || entry.Type != "text" {
			t.Fatalf("invalid prompt: %#v", entry)
		}
		if seen[entry.Name] {
			t.Fatalf("duplicate prompt name %q", entry.Name)
		}
		seen[entry.Name] = true
	}
	if !seen["wiki_summary"] {
		t.Fatal("wiki_summary missing")
	}
}
