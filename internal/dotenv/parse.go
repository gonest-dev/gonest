package dotenv

import (
	"errors"
	"fmt"
	"os"
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

	// resolved accumulates keys as they're parsed, in order, so a later
	// line's interpolation can reference a key defined earlier in the SAME
	// file (T3 -- resolveInterpolation checks this map before os.Getenv).
	resolved := make(map[string]string, len(lines))

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

		value, err := parseValue(valueExpr, resolved)
		if err != nil {
			if errors.Is(err, errUnterminatedQuote) {
				return nil, fmt.Errorf("gonest: unterminated quote in dotenv, line %d", lineNo)
			}
			return nil, err
		}

		pairs = append(pairs, envPair{Key: key, Value: value})
		resolved[key] = value
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
func parseValue(raw string, resolved map[string]string) (value string, err error) {
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
		content, err := extractDelimited(trimmed, '"')
		if err != nil {
			return "", err
		}
		return resolveInterpolation(applyEscapes(content), resolved), nil
	default:
		return resolveInterpolation(trimmed, resolved), nil
	}
}

// extractDelimited reads s (s[0] == delim) until the matching unescaped
// closing delim, returning the content between the delimiters (delimiters
// themselves stripped). A backslash escapes the next character for the
// purpose of NOT treating it as the closing delimiter. When the escaped
// character IS the delimiter itself (e.g. \' inside a single-quoted value,
// \" inside a double-quoted one), the backslash is consumed and only the
// literal delimiter character is written to the result -- so "it said
// \'hi\'" comes out with real "'" characters, not a stray backslash. Any
// OTHER backslash-escaped pair (e.g. \n, \\ inside a double-quoted value) is
// left untouched in the extracted content -- resolving those is a separate
// pass (applyEscapes, double-quoted values only).
func extractDelimited(s string, delim byte) (string, error) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == delim {
				b.WriteByte(delim)
			} else {
				b.WriteByte(c)
				b.WriteByte(next)
			}
			i++
			continue
		}
		if c == delim {
			return b.String(), nil
		}
		b.WriteByte(c)
	}
	return "", errUnterminatedQuote
}

// applyEscapes converts the 4 double-quoted-only backslash escape sequences
// -- \n, \r, \t, \\ -- into their real single-byte characters (0x0A, 0x0D,
// 0x09, '\'). Any other backslash pair (or a trailing lone backslash) is
// left untouched. Called ONLY from parseValue's double-quote branch, BEFORE
// resolveInterpolation runs on the result -- bare/single-quoted/backtick
// values never see this function.
func applyEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(c)
	}

	return b.String()
}

// resolveInterpolation scans s char by char (hand-rolled, not a single
// regex -- see design.md's Tech Decisions) for "$VAR", "${VAR}",
// "${VAR:-default}", "${VAR-default}", "${VAR:+alt}", "${VAR+alt}". Each
// VAR is looked up first in resolved (keys already parsed earlier in the
// SAME dotenv load, in order), falling back to os.LookupEnv(VAR) (a key
// from a previous Load path, or the real process env). "$" not followed by
// a valid variable-name start (or "{"), and "${...}" with no closing "}",
// are left untouched as literal text.
func resolveInterpolation(s string, resolved map[string]string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		c := s[i]
		if c != '$' || i+1 >= len(s) {
			b.WriteByte(c)
			i++
			continue
		}

		next := s[i+1]
		switch {
		case next == '{':
			closeIdx := strings.IndexByte(s[i+2:], '}')
			if closeIdx == -1 {
				// No matching "}" -- leave the "$" as a literal character
				// and keep scanning from the next byte.
				b.WriteByte(c)
				i++
				continue
			}
			closeIdx += i + 2
			b.WriteString(resolveBraceExpr(s[i+2:closeIdx], resolved))
			i = closeIdx + 1

		case isVarNameStart(next):
			j := i + 1
			for j < len(s) && isVarNameChar(s[j]) {
				j++
			}
			name := s[i+1 : j]
			value, _ := lookupVar(name, resolved)
			b.WriteString(value)
			i = j

		default:
			b.WriteByte(c)
			i++
		}
	}

	return b.String()
}

// resolveBraceExpr resolves the content INSIDE "${...}" (braces already
// stripped by the caller): a bare "VAR" name, or one of the 4 POSIX
// parameter-expansion operators. Operators are matched longest-prefix-first
// (":-"/":+"" before "-"/"+") so e.g. "VAR:-x" isn't misread as "VAR:" with
// op "-".
func resolveBraceExpr(inner string, resolved map[string]string) string {
	name, op, arg, hasOp := splitBraceOperator(inner)
	if !hasOp {
		value, _ := lookupVar(name, resolved)
		return value
	}

	value, isSet := lookupVar(name, resolved)

	switch op {
	case ":-":
		if !isSet || value == "" {
			return arg
		}
		return value
	case "-":
		if !isSet {
			return arg
		}
		return value
	case ":+":
		if isSet && value != "" {
			return arg
		}
		return ""
	case "+":
		if isSet {
			return arg
		}
		return ""
	default:
		return value
	}
}

// splitBraceOperator splits "${...}" inner content into (name, operator,
// argument, found). The 2-char operators (":-", ":+") are checked before
// their 1-char counterparts ("-", "+") since a 1-char scan would otherwise
// swallow the ":" into the variable name.
func splitBraceOperator(inner string) (name, op, arg string, found bool) {
	for _, candidate := range []string{":-", ":+", "-", "+"} {
		if idx := strings.Index(inner, candidate); idx != -1 {
			return inner[:idx], candidate, inner[idx+len(candidate):], true
		}
	}
	return inner, "", "", false
}

// lookupVar resolves name against resolved (keys parsed earlier in the same
// load, checked first) and falls back to os.LookupEnv. The returned bool
// distinguishes "unset" from "set but empty" -- required by the "-"/"+"
// operators (without ":"), which must NOT treat an empty-but-set variable
// the same as an unset one.
func lookupVar(name string, resolved map[string]string) (value string, isSet bool) {
	if v, ok := resolved[name]; ok {
		return v, true
	}
	return os.LookupEnv(name)
}

// isVarNameStart reports whether c can start a bare "$VAR" variable name
// (letters and underscore -- no leading digit, matching POSIX shell rules).
func isVarNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isVarNameChar reports whether c can appear after the first character of
// a bare "$VAR" variable name (adds digits to isVarNameStart's set).
func isVarNameChar(c byte) bool {
	return isVarNameStart(c) || (c >= '0' && c <= '9')
}
