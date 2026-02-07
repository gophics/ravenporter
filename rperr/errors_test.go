package rperr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gophics/ravenporter/ir"
)

func TestDecodeErrorErrorAndUnwrap(t *testing.T) {
	cause := errors.New("bad")

	assert.Equal(t, "<nil>", (*DecodeError)(nil).Error())
	assert.Equal(t, "gltf decode error: failed: bad", (&DecodeError{
		Format:  ir.FormatGLTF,
		Message: "failed",
		Cause:   cause,
	}).Error())
	assert.Equal(t, "decode error: failed: bad", (&DecodeError{
		Message: "failed",
		Cause:   cause,
	}).Error())
	assert.Equal(t, "obj decode error: failed", (&DecodeError{
		Format:  ir.FormatOBJ,
		Message: "failed",
	}).Error())
	assert.Nil(t, (*DecodeError)(nil).Unwrap())
	assert.ErrorIs(t, (&DecodeError{Cause: cause}).Unwrap(), cause)
}

func TestValidationErrorError(t *testing.T) {
	assert.Equal(t, "<nil>", (*ValidationError)(nil).Error())
	assert.Equal(t, "CODE: message", (&ValidationError{Code: "CODE", Message: "message"}).Error())
	assert.Equal(t, "message", (&ValidationError{Message: "message"}).Error())
}
