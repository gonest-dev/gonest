package dotenv

import (
	"errors"
	"fmt"
	"strings"
)

// envPair is one parsed KEY=VALUE line from a .env file, order preserved --
// interpolation (a future task) can reference a key defined earlier in the
// SAME file, so parseFile returns a slice, not a map.
type envPair struct {
	Key   string
	Value string
}

// errUnterminatedQuote is parseValue's sentinel for an opening quote/backtick
// with no matching unescaped close before end of line. parseFile turns this
// into the line-numbered error message callers see.
var errUnterminatedQuote = errors.New("unterminated quote")

// parseFile splits raw into lines (tolerating "\r\n" -- the trailing "\r" of
// each line is trimmed before any other processing) and classifies each one,
// in order: blank (after left-trim) is skipped, a line starting with "#" is
// a whole-line comment and is skipped, otherwise the line must contain "="
// -- the part before the first "=" (trimmed) is the key, the rest is the raw
// value expression handed to parseValue. A line with none of the above
// (no "=", not blank, not a comment) is a fail-loud error identifying the
// 1-indexed line number.
//
// NOTE (this task, T2): parseValue only extracts the raw delimited content
// per quote style -- no interpolation, no escape processing, no inline
// comment stripping, no real backtick multiline. Those are future tasks.
func parseFile(raw []byte) ([]envPair, error) {
	lines := strings.Split(string(raw), "\n")
	pairs := make([]envPair, 0, len(lines))

	for idx, line := range lines {
		lineNo := idx + 1
		line = strings.TrimSuffix(line, "\r")

		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		eq := strings.IndexByte(trimmed, '=')
		if eq == -1 {
			return nil, fmt.Errorf("gonest: malformed line %d in dotenv: %q", lineNo, line)
		}

		key := strings.TrimSpace(trimmed[:eq])
		valueExpr := trimmed[eq+1:]

		value, err := parseValue(valueExpr)
		if err != nil {
			if errors.Is(err, errUnterminatedQuote) {
				return nil, fmt.Errorf("gonest: unterminated quote in dotenv, line %d", lineNo)
			}
			return nil, err
		}

		pairs = append(pairs, envPair{Key: key, Value: value})
	}

	return pairs, nil
}

// parseValue dispatches by the value's opening character -- backtick,
// single quote, double quote, or bare (anything else) -- and extracts the
// RAW delimited content only: no interpolation, no escape-sequence
// processing, no inline-comment stripping, no real multiline support for
// backtick yet (all future tasks). Leading whitespace right after "=" is
// trimmed before dispatch so "KEY = \"val\"" still detects the quote.
//
// A quoted/backtick value with no matching unescaped closing delimiter
// before end of line returns errUnterminatedQuote.
func parseValue(raw string) (value string, err error) {
	trimmed := strings.TrimLeft(raw, " \t")
	if trimmed == "" {
		return "", nil
	}

	switch trimmed[0] {
	case '`':
		return extractDelimited(trimmed, '`')
	case '\'':
		return extractDelimited(trimmed, '\'')
	case '"':
		return extractDelimited(trimmed, '"')
	default:
		return trimmed, nil
	}
}

// extractDelimited reads s (s[0] == delim) until the matching unescaped
// closing delim, returning the raw content between the delimiters
// (delimiters themselves stripped). A backslash escapes the next character
// only for the purpose of NOT treating it as the closing delimiter -- the
// escape itself is left untouched in the extracted content (escape
// resolution is a future task).
func extractDelimited(s string, delim byte) (string, error) {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) {
				i++
			}
		case delim:
			return s[1:i], nil
		}
	}
	return "", errUnterminatedQuote
}
