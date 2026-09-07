package ai

import "strings"

// Turning a provider's refusal into something an athlete can act on.
//
// The verbatim message is kept, because it is the provider's own account of
// what went wrong and hiding it would make a real problem unreportable. What
// it lacks is the next move: "You exceeded your current quota" is accurate and
// tells a first-time user nothing, since the quota it means belongs to a
// developer account they may not know they opened.
//
// Matching is on sets of words rather than one phrase, which is not fussiness:
// the first version of this looked for "invalid api key" and missed
// Anthropic's actual "API key is invalid." — the same words in the other
// order, and the commonest refusal there is. Each rule fires when any one of
// its alternatives has all of its words present.
//
// Only failures seen on the way in are listed. A message no rule recognises is
// passed through untouched rather than guessed at: sending an athlete to fix
// something that is not broken is worse than saying nothing.
var refusals = []struct {
	alternatives [][]string
	fix          string
}{
	{
		alternatives: [][]string{
			{"quota"},
			{"credit", "balance"},
			{"billing"},
			{"payment"},
			{"purchase"},
		},
		fix: "Your key works but the account behind it has no balance. " +
			"Add a little credit on the provider's billing page — this is the part a " +
			"ChatGPT or Claude subscription does not cover — then press Connect again.",
	},
	{
		alternatives: [][]string{
			{"api", "key", "invalid"},
			{"api", "key", "incorrect"},
			{"invalid", "x-api-key"},
			{"authentication_error"},
			{"unauthorized"},
		},
		fix: "The provider did not recognise that key. Copy it again from the keys page: " +
			"a key is shown once, and one that was truncated on the way over looks like this.",
	},
	{
		alternatives: [][]string{
			{"does not exist"},
			{"not_found"},
			{"model_not_found"},
			{"do not have access"},
		},
		fix: "The key is fine but this account cannot reach that model. " +
			"Pick another from the list — the cheapest one is reachable on every account — " +
			"or unlock the model with your provider.",
	},
	{
		alternatives: [][]string{
			{"rate_limit"},
			{"rate limit"},
			{"too many requests"},
		},
		fix: "The provider is rate-limiting this account right now. Wait a minute and press Connect again; " +
			"a brand new account is limited hardest for its first few requests.",
	},
}

// guide appends the next move to a provider's own refusal, and leaves anything
// it does not recognise exactly as it came.
func guide(message string) string {
	lower := strings.ToLower(message)
	for _, r := range refusals {
		for _, alternative := range r.alternatives {
			if containsAll(lower, alternative) {
				return strings.TrimSpace(message) + "\n\n" + r.fix
			}
		}
	}
	return message
}

func containsAll(haystack string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(haystack, w) {
			return false
		}
	}
	return true
}
