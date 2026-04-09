package users

// ImportFailure holds information about a single bookmark that could not be
// imported.
type ImportFailure struct {
	URL    string
	Reason string
}
