package ai

import (
	"strings"
	"testing"
)

// Anthropic keys start sk-ant-, which is also a valid OpenAI prefix. Pasting
// one under the wrong provider has to be caught here, before a request — and
// an athlete's patience — is spent proving it.
func TestKeyShapeCatchesTheWrongProvider(t *testing.T) {
	openai, _ := providerByID(ProviderOpenAI)
	anthropic, _ := providerByID(ProviderAnthropic)

	err := checkKeyShape(openai, "sk-ant-api03-abcdefg")
	if err == nil || !strings.Contains(err.Error(), "Claude") {
		t.Fatalf("an Anthropic key under OpenAI should name Claude, got: %v", err)
	}
	if err := checkKeyShape(openai, "sk-proj-abcdefg"); err != nil {
		t.Fatalf("a real OpenAI key was refused: %v", err)
	}
	if err := checkKeyShape(anthropic, "sk-ant-api03-abcdefg"); err != nil {
		t.Fatalf("a real Anthropic key was refused: %v", err)
	}
	if err := checkKeyShape(anthropic, "hunter2"); err == nil {
		t.Fatal("a password pasted into the key box was accepted")
	}
}

func TestKeyHintShowsOnlyTheTail(t *testing.T) {
	if got := keyHint("sk-ant-api03-verylongkey1234"); got != "1234" {
		t.Fatalf("hint = %q, want the last four characters", got)
	}
	if got := keyHint("abc"); got != "abc" {
		t.Fatalf("a short key should be returned as-is, got %q", got)
	}
}

// An unconnected account, an unknown provider name and an empty model all have
// to land somewhere sensible rather than on nothing.
func TestDefaultsFallBackToClaude(t *testing.T) {
	if defaultModel(ProviderOpenAI) != defaultOpenAIModel {
		t.Fatal("OpenAI should default to its own model")
	}
	if defaultModel("nonsense") != defaultAnthropicModel {
		t.Fatal("an unknown provider should default to Claude")
	}
	c := NewClient(nil, Credentials{Provider: ProviderOpenAI, Key: "sk-test"}, Settings{})
	if c.Model() != defaultOpenAIModel {
		t.Fatalf("a client with no model chose %q", c.Model())
	}
	if c.Provider() != ProviderOpenAI {
		t.Fatalf("provider = %q", c.Provider())
	}
}
