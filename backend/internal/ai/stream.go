package ai

// Delta is one increment of the model's output. Kind is "thinking" while the
// model is reasoning and "text" once it is writing the answer. Both providers
// report both phases, so everything downstream of here — the progress bar
// especially — is written against this and not against either API.
type Delta struct {
	Kind string
	Text string
}
