package goyek

import (
	"context"
	"runtime"
	"strings"
)

// TF is a type passed to Task's Action function to manage task state.
//
// A Task ends when its Action function returns or calls any of the methods
// FailNow, Fatal, Fatalf, SkipNow, Skip, or Skipf.
//
// All methods must be called only from the goroutine running the
// Action function.
type TF struct {
	ctx         context.Context
	name        string
	output      Output
	paramValues map[string]ParamValue
	failed      bool
	skipped     bool
}

// Context returns the taskflows' run context.
func (tf *TF) Context() context.Context {
	return tf.ctx
}

// Name returns the name of the running task.
func (tf *TF) Name() string {
	return tf.name
}

// Output provides the writers used to print output.
func (tf *TF) Output() Output {
	return tf.output
}

// Log formats its arguments using default formatting, analogous to Println,
// and prints the text to Output. A final newline is added.
// The text will be printed only if the task fails or taskflow is run in Verbose mode.
func (tf *TF) Log(args ...interface{}) {
	vs := make([]string, len(args))
	for i := 0; i < len(args); i++ {
		vs[i] = "%v"
	}
	tf.output.WriteMessagef(strings.Join(vs, " "), args...)
}

// Logf formats its arguments according to the format, analogous to Printf,
// and prints the text to Output. A final newline is added.
// The text will be printed only if the task fails or taskflow is run in Verbose mode.
func (tf *TF) Logf(format string, args ...interface{}) {
	tf.output.WriteMessagef(format, args...)
}

// Error is equivalent to Log followed by Fail.
func (tf *TF) Error(args ...interface{}) {
	tf.Log(args...)
	tf.Fail()
}

// Errorf is equivalent to Logf followed by Fail.
func (tf *TF) Errorf(format string, args ...interface{}) {
	tf.Logf(format, args...)
	tf.Fail()
}

// Failed reports whether the function has failed.
func (tf *TF) Failed() bool {
	return tf.failed
}

// Fail marks the function as having failed but continues execution.
func (tf *TF) Fail() {
	tf.failed = true
}

// Fatal is equivalent to Log followed by FailNow.
func (tf *TF) Fatal(args ...interface{}) {
	tf.Log(args...)
	tf.FailNow()
}

// Fatalf is equivalent to Logf followed by FailNow.
func (tf *TF) Fatalf(format string, args ...interface{}) {
	tf.Logf(format, args...)
	tf.FailNow()
}

// FailNow marks the function as having failed
// and stops its execution by calling runtime.Goexit
// (which then runs all deferred calls in the current goroutine).
// It finishes the whole taskflow.
func (tf *TF) FailNow() {
	tf.Fail()
	runtime.Goexit()
}

// Skipped reports whether the task was skipped.
func (tf *TF) Skipped() bool {
	return tf.skipped
}

// Skip is equivalent to Log followed by SkipNow.
func (tf *TF) Skip(args ...interface{}) {
	tf.Log(args...)
	tf.SkipNow()
}

// Skipf is equivalent to Logf followed by SkipNow.
func (tf *TF) Skipf(format string, args ...interface{}) {
	tf.Logf(format, args...)
	tf.SkipNow()
}

// SkipNow marks the task as having been skipped
// and stops its execution by calling runtime.Goexit
// (which then runs all deferred calls in the current goroutine).
// If a test fails (see Error, Errorf, Fail) and is then skipped,
// it is still considered to have failed.
// Taskflow will continue at the next task.
func (tf *TF) SkipNow() {
	tf.skipped = true
	runtime.Goexit()
}
