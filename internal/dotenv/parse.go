package dotenv

// envPair is one parsed KEY=VALUE line from a .env file, order preserved --
// interpolation (a future task) can reference a key defined earlier in the
// SAME file, so parseFile returns a slice, not a map.
type envPair struct {
	Key   string
	Value string
}

// parseFile is a stub in this task (T1): it returns no pairs and no error,
// just enough for Load to compile and run end-to-end with no observable
// effect. A future task replaces this with the real line-classification +
// quote/interpolation/escape/comment parser (see
// .specs/features/dotenv-loading/design.md's parseFile component).
func parseFile(raw []byte) ([]envPair, error) {
	return nil, nil
}
