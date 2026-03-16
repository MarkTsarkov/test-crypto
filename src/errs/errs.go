package errs

import "errors"

var ErrNotFound = errors.New("not found")
var ErrWrongStatus = errors.New("wrong status")
var ErrUnprocessableEntity = errors.New("unprocessable entity")
var ErrPureBalance = errors.New("pure balance")
