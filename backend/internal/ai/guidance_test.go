package ai

import (
	"net/url"
	"strings"
	"testing"
)

// The refusal an athlete is most likely to hit, and the one that reads as
// nonsense to anyone who has not opened a developer account: the key is valid,
// the account has no balance. OpenAI says "quota", Anthropic says "credit
// balance", and neither says which account or where to top it up.
func TestTheNoBalanceRefusalSaysWhatToDoAboutIt(t *testing.T) {
	for _, verbatim := range []string{
		"You exceeded your current quota, please check your plan and billing details.",
		"Your credit balance is too low to access the Anthropic API.",
		"insufficient_quota",
	} {
		guided := guide(verbatim)
		if !strings.Contains(guided, verbatim) {
			t.Errorf("the provider's own words were lost from %q", verbatim)
		}
		if !strings.Contains(strings.ToLower(guided), "balance") {
			t.Errorf("no next move offered for %q: %q", verbatim, guided)
		}
		// The whole point: say that the subscription is not the thing that pays.
		if !strings.Contains(guided, "subscription does not cover") {
			t.Errorf("the refusal did not say a subscription does not cover it: %q", guided)
		}
	}
}

// These are the providers' real words, not invented ones. The first version
// of this matcher looked for "invalid api key" and missed Anthropic's actual
// "API key is invalid." — observed live against api.anthropic.com, and the
// commonest refusal there is.
func TestAMistypedKeyIsToldToCopyItAgain(t *testing.T) {
	for _, verbatim := range []string{
		"API key is invalid.",                       // Anthropic, observed
		"invalid x-api-key",                         // Anthropic, raw error type
		"Incorrect API key provided: sk-proj-****.", // OpenAI
		"authentication_error",                      // either, raw
	} {
		guided := guide(verbatim)
		if !strings.Contains(strings.ToLower(guided), "copy it again") {
			t.Errorf("no guidance for %q: %q", verbatim, guided)
		}
		if !strings.Contains(guided, verbatim) {
			t.Errorf("the provider's own words were lost from %q", verbatim)
		}
	}
}

func TestAModelTheAccountCannotReachPointsAtTheList(t *testing.T) {
	guided := guide("The model `gpt-5` does not exist or you do not have access to it.")
	if !strings.Contains(strings.ToLower(guided), "cheapest") {
		t.Errorf("guidance = %q", guided)
	}
}

// A message with no known next move must come back exactly as the provider
// wrote it. Guessing at an unfamiliar failure is worse than passing it on:
// it sends the athlete to fix something that is not broken.
func TestAnUnfamiliarRefusalIsPassedThroughUntouched(t *testing.T) {
	const odd = "Upstream connect error or disconnect/reset before headers."
	if got := guide(odd); got != odd {
		t.Errorf("an unrecognised refusal was embellished: %q", got)
	}
	if got := guide(""); got != "" {
		t.Errorf("guide(%q) = %q", "", got)
	}
}

// Every provider has to be followable by someone who has never seen an API
// key, and the order matters more than the words: a key made before there is
// credit behind it looks valid and refuses every request.
func TestEveryProviderCanBeFollowedFromNothing(t *testing.T) {
	for _, p := range Providers {
		if len(p.Steps) < 3 {
			t.Errorf("%s: %d steps is not a walkthrough", p.Label, len(p.Steps))
			continue
		}

		var billing, keys int = -1, -1
		for i, step := range p.Steps {
			if step.Do == "" {
				t.Errorf("%s: step %d says nothing to do", p.Label, i+1)
			}
			if step.URL != "" {
				u, err := url.Parse(step.URL)
				if err != nil || u.Scheme != "https" || u.Host == "" {
					t.Errorf("%s: step %d links to %q", p.Label, i+1, step.URL)
				}
			}
			if strings.Contains(strings.ToLower(step.Do), "credit") {
				billing = i
			}
			if strings.Contains(strings.ToLower(step.Do), "key") &&
				strings.Contains(strings.ToLower(step.Do), "create") {
				keys = i
			}
		}

		if billing < 0 {
			t.Errorf("%s: no step tells the athlete to add credit, which is the "+
				"step whose absence produces a key that refuses everything", p.Label)
		}
		if keys < 0 {
			t.Errorf("%s: no step creates a key", p.Label)
		}
		if billing >= 0 && keys >= 0 && billing > keys {
			t.Errorf("%s: credit is added after the key is made; that ordering is "+
				"what leaves people with a key and no balance", p.Label)
		}
	}
}

// The one thing a walkthrough exists to say. Both companies sell a consumer
// subscription under the same brand as the developer account, and an athlete
// who tops up the wrong one has paid and still cannot connect.
func TestTheWalkthroughWarnsThatTheSubscriptionIsTheWrongAccount(t *testing.T) {
	for _, p := range Providers {
		var said bool
		for _, step := range p.Steps {
			note := strings.ToLower(step.Note)
			if strings.Contains(note, "separate account") || strings.Contains(note, "does not cover") {
				said = true
			}
		}
		if !said {
			t.Errorf("%s: no step warns that the subscription account is not the one "+
				"the key comes from", p.Label)
		}
	}
}

// The connector calls the string a connection code, because that is what an
// athlete is doing: connecting their account. The provider's own pages call it
// an API key. Both names are right in their own place, and a walkthrough that
// renamed the provider's button would send someone hunting for a control that
// does not exist there.
func TestTheWalkthroughKeepsTheProvidersOwnWordForTheirOwnPages(t *testing.T) {
	for _, p := range Providers {
		var onTheirSite, onOurs int
		for _, step := range p.Steps {
			if step.URL != "" && strings.Contains(strings.ToLower(step.Do), "key") {
				onTheirSite++
			}
			if step.URL == "" && strings.Contains(strings.ToLower(step.Do), "connection code") {
				onOurs++
			}
		}
		if onTheirSite == 0 {
			t.Errorf("%s: no step on the provider's own site says \"key\", which is what "+
				"their page calls it", p.Label)
		}
		if onOurs == 0 {
			t.Errorf("%s: the step taken here does not name the connection code, which is "+
				"what this app's field is labelled", p.Label)
		}
	}
}

// Nothing an athlete reads should tell them to connect a "key" — they connect
// an account, and the code is how. The message every coaching endpoint answers
// with when nothing is connected is the one they see most.
func TestTheNotConnectedMessageAsksForAnAccount(t *testing.T) {
	msg := ErrNoCredentials.Error()
	if !strings.Contains(msg, "account") {
		t.Errorf("ErrNoCredentials = %q, which does not name what is being connected", msg)
	}
	if strings.Contains(msg, "key") {
		t.Errorf("ErrNoCredentials = %q; the athlete connects an account, not a key", msg)
	}
}
