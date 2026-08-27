package indexing

import "errors"

var (
	errLLMUnavailable         = errors.New("llm unavailable")
	errNoFeaturesForCheatsheet = errors.New("no features detected; index the PDF or add features before rebuilding cheatsheet")
	errNoSectionText          = errors.New("no indexed section text available")
)
