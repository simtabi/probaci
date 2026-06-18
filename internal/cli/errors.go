package cli

import "errors"

// codedError carries a process exit code.
type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string { return e.err.Error() }
func (e codedError) Unwrap() error { return e.err }

// Exit codes (also documented in docs/).
const (
	exitOK            = 0
	exitStageFailed   = 1
	exitUsage         = 2
	exitDockerMissing = 125
)

func failure(err error) error   { return codedError{code: exitStageFailed, err: err} }
func usageErr(err error) error  { return codedError{code: exitUsage, err: err} }
func dockerErr(err error) error { return codedError{code: exitDockerMissing, err: err} }

// ExitCode extracts the process exit code from an error returned by Execute.
func ExitCode(err error) int {
	if err == nil {
		return exitOK
	}
	var ce codedError
	if errors.As(err, &ce) {
		return ce.code
	}
	return exitStageFailed
}
