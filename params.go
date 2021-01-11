package taskflow

import (
	"errors"
	"strconv"
	"time"
)

// ErrParamNotSet indicates that a parameter is not set.
var ErrParamNotSet = errors.New("parameter is not set")

// Params represents Taskflow parameters used within Taskflow.
// The default values set in the struct are overridden in Run method.
type Params map[string]string

// Int converts the parameter to int using the Go syntax for integer literals.
// ErrParamNotSet error is returned if the parameter was not set.
// *strconv.NumError error is returned if the parameter conversion failed.
func (p Params) Int(key string) (int, error) {
	v := p[key]
	if v == "" {
		return 0, ErrParamNotSet
	}
	i, err := strconv.ParseInt(v, 0, strconv.IntSize)
	return int(i), err
}

// Bool converts the parameter to bool.
// It accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// Any other value returns an error.
// ErrParamNotSet error is returned if the parameter was not set.
// *strconv.NumError error is returned if the parameter conversion failed.
func (p Params) Bool(key string) (bool, error) {
	v := p[key]
	if v == "" {
		return false, ErrParamNotSet
	}
	return strconv.ParseBool(v)
}

// Float64 converts the parameter to float64 accepting decimal and hexadecimal floating-point number syntax.
// ErrParamNotSet error is returned if the parameter was not set.
// *strconv.NumError error is returned if the parameter conversion failed.
func (p Params) Float64(key string) (float64, error) {
	v := p[key]
	if v == "" {
		return 0, ErrParamNotSet
	}
	return strconv.ParseFloat(v, 64)
}

// Duration converts the parameter to time.Duration using time.ParseDuration.
// A duration string is a possibly signed sequence of
// decimal numbers, each with optional fraction and a unit suffix,
// such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
// ErrParamNotSet error is returned if the parameter was not set.
// An error is also returned if the parameter conversion failed.
func (p Params) Duration(key string) (time.Duration, error) {
	v := p[key]
	if v == "" {
		return 0, ErrParamNotSet
	}
	return time.ParseDuration(v)
}