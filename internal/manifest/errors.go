package manifest

// ParseError reports where in the document parsing failed. Path is a slash
// separated pointer to the offending node, empty when the failure is on a
// header field: go-yaml decodes those with its struct decoder, which offers no
// hook to record the key.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	if e.Path == "" {
		return "parse: " + e.Err.Error()
	}
	return "parse " + e.Path + ": " + e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }

func wrap(segment string, err error) error {
	if pe, ok := err.(*ParseError); ok {
		return &ParseError{Path: "/" + segment + pe.Path, Err: pe.Err}
	}
	return &ParseError{Path: "/" + segment, Err: err}
}
