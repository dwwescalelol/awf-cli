package manifest

// ParseError reports where in the document parsing failed. Path is a slash
// separated pointer to the offending node, empty when the failure is on a
// header field.
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

// ValidationError reports what is wrong at one place in the document. Path is
// a slash separated pointer to the offending node, in the style of ParseError.
type ValidationError struct {
	Path string
	Msg  string
}

func (e *ValidationError) Error() string { return "invalid " + e.Path + ": " + e.Msg }
