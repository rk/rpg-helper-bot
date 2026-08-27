package llm

import "testing"

func TestLoadPromptSearchRewrite(t *testing.T) {
	p := LoadPrompt(PromptSearchRewrite)
	if p == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(p, "dice") && !contains(p, "Dice") {
		t.Fatalf("expected dice guidance in prompt: %q", p[:min(120, len(p))])
	}
}

func TestRenderPromptChat(t *testing.T) {
	out := renderPrompt(PromptChatSystem, map[string]string{
		"GLOSSARY":   "Book terminology:\n- Core: Tests => skill_check\n",
		"GAME_NOTES": "Wild die explosions enabled.\n",
		"EXCERPTS":   "\n[1] Core — Combat (pages 1-5)\nExample excerpt.\n",
	})
	if !contains(out, "Example excerpt") {
		t.Fatalf("missing excerpts: %q", out)
	}
	if !contains(out, "Wild die") {
		t.Fatalf("missing game notes: %q", out)
	}
	if !contains(out, "Tests") {
		t.Fatalf("missing glossary: %q", out)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
