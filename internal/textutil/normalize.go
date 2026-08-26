package textutil

import "strings"

// NormalizePDFText replaces common PDF punctuation and ligatures with ASCII-friendly forms.
func NormalizePDFText(s string) string {
	if s == "" {
		return s
	}
	replacer := strings.NewReplacer(
		"\u201c", `"`, // “
		"\u201d", `"`, // ”
		"\u2018", `'`, // ‘
		"\u2019", `'`, // ’
		"\u201a", `'`, // ‚
		"\u201e", `"`, // „
		"\u2032", `'`, // ′
		"\u2033", `"`, // ″
		"\u2026", "...", // …
		"\u2013", "-", // –
		"\u2014", "-", // —
		"\u2212", "-", // −
		"\u00a0", " ", // nbsp
		"\ufb00", "ff", // ﬀ
		"\ufb01", "fi", // ﬁ
		"\ufb02", "fl", // ﬂ
		"\ufb03", "ffi", // ﬃ
		"\ufb04", "ffl", // ﬄ
	)
	return replacer.Replace(s)
}
