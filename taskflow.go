package goyek

import (
	"context"
	"fmt"
	"os"
	"regexp"
)

const (
	// CodePass indicates that taskflow passed.
	CodePass = 0
	// CodeFail indicates that taskflow failed.
	CodeFail = 1
	// CodeInvalidArgs indicates that taskflow got invalid input.
	CodeInvalidArgs = 2
)

// Taskflow is the root type of the package.
// Use Register methods to register all tasks
// and Run or Main method to execute provided tasks.
type Taskflow struct {
	// Output specifies the writers where text is to be printed.
	// If a writer is nil, Goyek uses defaults: os.Stdout for Standard, os.Stderr for Messaging.
	// If you want to explicitly skip output, set the corresponding member to ioutil.Discard.
	Output Output

	DefaultTask RegisteredTask // task which is run when non is explicitly provided

	verbose *RegisteredBoolParam   // when enabled, then the whole output will be always streamed
	workDir *RegisteredStringParam // sets the working directory
	params  map[string]registeredParam
	tasks   map[string]Task
}

// RegisteredTask represents a task that has been registered to a Taskflow.
// It can be used as a dependency for another Task.
type RegisteredTask struct {
	name string
}

// VerboseParam returns the out-of-the-box verbose parameter which controls the output behavior.
func (f *Taskflow) VerboseParam() RegisteredBoolParam {
	if f.verbose == nil {
		param := f.RegisterBoolParam(BoolParam{
			Name:  "v",
			Usage: "Verbose: log all tasks as they are run.",
		})
		f.verbose = &param
	}

	return *f.verbose
}

// WorkDirParam returns the out-of-the-box working directory parameter which controls the working directory.
func (f *Taskflow) WorkDirParam() RegisteredStringParam {
	if f.workDir == nil {
		param := f.RegisterStringParam(StringParam{
			Name:    "wd",
			Usage:   "Working directory: set the working directory.",
			Default: ".",
		})
		f.workDir = &param
	}

	return *f.workDir
}

// RegisterValueParam registers a generic parameter that is defined by the calling code.
// Use this variant in case the primitive-specific implementations cannot cover the parameter.
//
// The value is provided via a factory function since Taskflow could be executed multiple times,
// requiring a new Value instance each time.
func (f *Taskflow) RegisterValueParam(p ValueParam) RegisteredValueParam {
	regParam := registeredParam{
		name:     p.Name,
		usage:    p.Usage,
		newValue: p.NewValue,
	}
	f.registerParam(regParam)
	return RegisteredValueParam{regParam}
}

// RegisterBoolParam registers a boolean parameter.
func (f *Taskflow) RegisterBoolParam(p BoolParam) RegisteredBoolParam {
	valGetter := func() ParamValue {
		value := boolValue(p.Default)
		return &value
	}
	f.registerParam(registeredParam{
		name:     p.Name,
		usage:    p.Usage,
		newValue: valGetter,
	})
	return RegisteredBoolParam{registeredParam{name: p.Name}}
}

// RegisterIntParam registers an integer parameter.
func (f *Taskflow) RegisterIntParam(p IntParam) RegisteredIntParam {
	valGetter := func() ParamValue {
		value := intValue(p.Default)
		return &value
	}
	regParam := registeredParam{
		name:     p.Name,
		usage:    p.Usage,
		newValue: valGetter,
	}
	f.registerParam(regParam)
	return RegisteredIntParam{regParam}
}

// RegisterStringParam registers a string parameter.
func (f *Taskflow) RegisterStringParam(p StringParam) RegisteredStringParam {
	valGetter := func() ParamValue {
		value := stringValue(p.Default)
		return &value
	}
	regParam := registeredParam{
		name:     p.Name,
		usage:    p.Usage,
		newValue: valGetter,
	}
	f.registerParam(regParam)
	return RegisteredStringParam{regParam}
}

// ParamNamePattern describes the regular expression a parameter name must match.
const ParamNamePattern = "^[a-zA-Z0-9][a-zA-Z0-9_-]*$"

var paramNameRegex = regexp.MustCompile(ParamNamePattern)

func (f *Taskflow) registerParam(p registeredParam) {
	if !paramNameRegex.MatchString(p.name) {
		panic("parameter name must match ParamNamePattern")
	}
	if p.newValue == nil {
		panic("parameter is missing default value factory")
	}
	if _, exists := f.params[p.name]; exists {
		panic(fmt.Sprintf("%s parameter was already registered", p.name))
	}
	if f.params == nil {
		f.params = make(map[string]registeredParam)
	}
	f.params[p.name] = p
}

// TaskNamePattern describes the regular expression a task name must match.
const TaskNamePattern = "^[a-zA-Z0-9_][a-zA-Z0-9_-]*$"

var taskNameRegex = regexp.MustCompile(TaskNamePattern)

// Register registers the task. It panics in case of any error.
func (f *Taskflow) Register(task Task) RegisteredTask {
	// validate
	if !taskNameRegex.MatchString(task.Name) {
		panic("task name must match TaskNamePattern")
	}
	if f.isRegistered(task.Name) {
		panic(fmt.Sprintf("%s task was already registered", task.Name))
	}
	for _, dep := range task.Deps {
		if !f.isRegistered(dep.name) {
			panic(fmt.Sprintf("invalid dependency %s", dep.name))
		}
	}

	f.tasks[task.Name] = task
	return RegisteredTask{name: task.Name}
}

// Run runs provided tasks and all their dependencies.
// Each task is executed at most once.
func (f *Taskflow) Run(ctx context.Context, args ...string) int {
	if ctx == nil {
		ctx = context.Background()
	}

	flow := &flowRunner{
		output:      f.Output,
		params:      f.params,
		tasks:       f.tasks,
		verbose:     f.VerboseParam(),
		workDir:     f.WorkDirParam(),
		defaultTask: f.DefaultTask,
	}

	if flow.output.Standard == nil {
		flow.output.Standard = os.Stdout
	}
	if flow.output.Messaging == nil {
		flow.output.Messaging = os.Stderr
	}

	return flow.Run(ctx, args)
}

func (f *Taskflow) isRegistered(name string) bool {
	if f.tasks == nil {
		f.tasks = map[string]Task{}
	}
	_, ok := f.tasks[name]
	return ok
}
