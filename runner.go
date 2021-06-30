package goyek

import (
	"context"
	"runtime/debug"
	"time"
)

// runner is used to run a Action.
type runner struct {
	Ctx         context.Context
	TaskName    string
	Output      Output
	ParamValues map[string]ParamValue
}

// runResult contains the results of a Action run.
type runResult struct {
	failed   bool
	skipped  bool
	duration time.Duration
}

// Failed returns true if a action failed.
// Actions are failed by calling Error, Fail or related methods, or a panic.
func (r runResult) Failed() bool {
	return r.failed
}

// Skipped returns true if a action was skipped.
// Actions are skipped by calling Skip or related methods.
func (r runResult) Skipped() bool {
	return r.skipped
}

// Duration returns the durations of the Action.
func (r runResult) Duration() time.Duration {
	return r.duration
}

// Run runs the action.
func (r runner) Run(action func(tf *TF)) runResult {
	finished := make(chan runResult)
	go func() {
		syncedOutput := Output{
			Standard:  &syncWriter{Writer: r.Output.Standard},
			Messaging: &syncWriter{Writer: r.Output.Messaging},
		}
		tf := &TF{
			ctx:         r.Ctx,
			name:        r.TaskName,
			output:      syncedOutput,
			paramValues: r.ParamValues,
		}
		from := time.Now()
		defer func() {
			if r := recover(); r != nil {
				tf.Errorf("panic: %v", r)
				tf.Log(string(debug.Stack()))
			}
			result := runResult{
				failed:   tf.failed,
				skipped:  tf.skipped,
				duration: time.Since(from),
			}
			finished <- result
		}()
		action(tf)
	}()
	return <-finished
}
