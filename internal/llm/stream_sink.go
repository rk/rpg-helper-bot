package llm

import "io"

// TextStreamSink receives answer text deltas encoded for the AI SDK data stream.
type TextStreamSink interface {
	WriteText(delta string) error
}

// PlainTextSink writes raw text without AI SDK framing (for tests).
type PlainTextSink struct {
	W io.Writer
}

func (p PlainTextSink) WriteText(delta string) error {
	_, err := io.WriteString(p.W, delta)
	return err
}
