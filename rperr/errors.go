package rperr

import (
	"fmt"

	"github.com/gophics/ravenporter/ir"
)

const (
	// SeverityInfo marks an informational validation finding.
	SeverityInfo = "info"

	// SeverityWarning marks a non-fatal validation finding.
	SeverityWarning = "warning"

	// SeverityError marks a fatal validation finding.
	SeverityError = "error"
)

// DecodeError reports a format-specific decode failure.
type DecodeError struct {
	Format  ir.FormatID
	Offset  int64
	Message string
	Cause   error
}

func (e *DecodeError) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.Cause != nil && e.Format != "":
		return fmt.Sprintf("%s decode error: %s: %v", e.Format, e.Message, e.Cause)
	case e.Cause != nil:
		return fmt.Sprintf("decode error: %s: %v", e.Message, e.Cause)
	case e.Format != "":
		return fmt.Sprintf("%s decode error: %s", e.Format, e.Message)
	default:
		return fmt.Sprintf("decode error: %s", e.Message)
	}
}

func (e *DecodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ValidationError reports a structural or semantic validation finding.
type ValidationError struct {
	Severity string
	Code     string
	Message  string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}
