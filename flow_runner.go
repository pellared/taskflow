package goyek

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

type flowRunner struct {
	output      Output
	params      map[string]registeredParam
	paramValues map[string]ParamValue
	tasks       map[string]Task
	verbose     RegisteredBoolParam
	workDir     RegisteredStringParam
	defaultTask RegisteredTask
}

// Run runs provided tasks and all their dependencies.
// Each task is executed at most once.
func (f *flowRunner) Run(ctx context.Context, args []string) int {
	f.verifyAllParametersAreInUse()
	f.initializeParameters()
	tasks, usageRequested, err := f.parseArguments(args)
	if err != nil {
		f.output.WriteMessagef("cannot parse arguments: %v", err)
		return CodeInvalidArgs
	}

	if usageRequested {
		printUsage(f.output.Standard, f)
		return CodePass
	}

	tasks = f.tasksToRun(tasks)

	if len(tasks) == 0 {
		f.output.WriteMessagef("no task provided")
		printUsage(f.output.Messaging, f)
		return CodeInvalidArgs
	}

	popWorkingDir, err := f.pushWorkingDir()
	if err != nil {
		f.output.WriteMessagef("cannot change working directory: %v", err)
		return CodeInvalidArgs
	}
	defer popWorkingDir()

	return f.runTasks(ctx, tasks)
}

func (f *flowRunner) verifyAllParametersAreInUse() {
	if unusedParams := f.unusedParams(); len(unusedParams) > 0 {
		panic(fmt.Sprintf("unused parameters: %v\n", unusedParams))
	}
}

func (f *flowRunner) initializeParameters() {
	f.paramValues = make(map[string]ParamValue)
	for _, param := range f.params {
		value := param.newValue()
		f.paramValues[param.name] = value
	}
}

func (f *flowRunner) parseArguments(args []string) ([]string, bool, error) {
	usageRequested := false
	var argHandler func(string) error
	handleNextArgFor := func(value ParamValue) {
		nextHandler := argHandler
		argHandler = func(s string) error {
			err := value.Set(s)
			argHandler = nextHandler
			return err
		}
	}
	var tasks []string

	argHandler = func(arg string) error {
		if _, isTask := f.tasks[arg]; isTask {
			tasks = append(tasks, arg)
			return nil
		}
		if arg[0] == '-' {
			// parse parameters
			split := strings.SplitN(arg[1:], "=", 2) //nolint:gomnd // ignore
			if value, isFlag := f.paramValues[split[0]]; isFlag {
				switch {
				case len(split) > 1:
					return value.Set(split[1])
				case value.IsBool():
					return value.Set("")
				default:
					handleNextArgFor(value)
					return nil
				}
			}
		}
		// if they haven't been overridden above, provide usage for common queries
		if (arg == "-h") || (arg == "--help") || (arg == "help") {
			usageRequested = true
			return nil
		}
		return fmt.Errorf("unknown argument: %s", arg)
	}

	for _, arg := range args {
		err := argHandler(arg)
		if err != nil {
			return []string{}, false, err
		}
	}
	return tasks, usageRequested, nil
}

func (f *flowRunner) tasksToRun(tasks []string) []string {
	if len(tasks) > 0 || (f.defaultTask.name == "") {
		return tasks
	}
	return []string{f.defaultTask.name}
}

func (f *flowRunner) pushWorkingDir() (func(), error) {
	wdParamVal, hasParam := f.paramValues[f.workDir.Name()]
	if !hasParam {
		return func() {}, nil
	}

	oldWd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	wd := wdParamVal.Get().(string) //nolint // it is always a string
	if err := os.Chdir(wd); err != nil {
		return func() {}, err
	}
	return func() {
		if err := os.Chdir(oldWd); err != nil {
			panic(err)
		}
	}, nil
}

func (f *flowRunner) runTasks(ctx context.Context, tasks []string) int {
	from := time.Now()
	executedTasks := map[string]bool{}
	for _, name := range tasks {
		if err := f.run(ctx, name, executedTasks); err != nil {
			f.output.WriteMessagef("%v\t%.3fs", err, time.Since(from).Seconds())
			return CodeFail
		}
	}
	f.output.WriteMessagef("ok\t%.3fs", time.Since(from).Seconds())
	return CodePass
}

func (f *flowRunner) run(ctx context.Context, name string, executed map[string]bool) error {
	task := f.tasks[name]
	if executed[name] {
		return nil
	}
	for _, dep := range task.Deps {
		if err := f.run(ctx, dep.name, executed); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	passed := f.runTask(ctx, task)
	if err := ctx.Err(); err != nil {
		return err
	}
	if !passed {
		return errors.New("task failed")
	}
	executed[name] = true
	return nil
}

func (f *flowRunner) runTask(ctx context.Context, task Task) bool {
	if task.Action == nil {
		return true
	}

	paramValues := make(map[string]ParamValue)
	for _, param := range task.Params {
		paramValues[param.Name()] = f.paramValues[param.Name()]
	}

	// if verbose flag is registered then check its value
	verboseParamVal, ok := f.paramValues[f.verbose.Name()]
	verbose := ok && verboseParamVal.Get().(bool)

	failed := false
	measuredAction := func(tf *TF) {
		var buffered *bufferedOutput
		output := tf.Output()
		if !verbose {
			buffered = &bufferedOutput{}
			output = buffered.Output()
		}

		// report task start
		output.WriteMessagef("===== TASK  %s", tf.Name())

		// run task
		r := runner{
			Ctx:         tf.Context(),
			TaskName:    tf.Name(),
			ParamValues: tf.paramValues,
			Output:      output,
		}
		result := r.Run(task.Action)

		// report task end
		status := "PASS"
		switch {
		case result.Failed():
			status = "FAIL"
			failed = true
		case result.Skipped():
			status = "SKIP"
		}
		output.WriteMessagef("----- %s: %s (%.2fs)", status, tf.Name(), result.Duration().Seconds())

		if (buffered != nil) && result.failed {
			buffered.WriteTo(tf.Output())
		}
	}

	measuredRunner := runner{
		Ctx:         ctx,
		TaskName:    task.Name,
		ParamValues: paramValues,
		Output:      f.output,
	}
	measuredRunner.Run(measuredAction)

	return !failed
}

func (f *flowRunner) unusedParams() []string {
	remainingParams := make(map[string]struct{})
	for key := range f.params {
		remainingParams[key] = struct{}{}
	}
	delete(remainingParams, f.verbose.Name())
	delete(remainingParams, f.workDir.Name())
	for _, task := range f.tasks {
		for _, param := range task.Params {
			delete(remainingParams, param.Name())
		}
	}
	unusedParams := make([]string, 0, len(remainingParams))
	for key := range remainingParams {
		unusedParams = append(unusedParams, key)
	}
	return unusedParams
}

func printUsage(out io.Writer, f *flowRunner) {
	flagName := func(paramName string) string {
		return "-" + paramName
	}

	writeLinef(out, "Usage: [flag(s) | task(s)]...")
	writeLinef(out, "Flags:")
	w := tabwriter.NewWriter(out, 1, 1, 4, ' ', 0) //nolint:gomnd // ignore
	keys := make([]string, 0, len(f.params))
	for key := range f.params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		param := f.params[key]
		_, _ = fmt.Fprintf(w, "  %s\tDefault: %s\t%s\n", flagName(param.name), param.newValue().String(), param.usage)
	}
	_ = w.Flush()

	writeLinef(out, "Tasks:")
	keys = make([]string, 0, len(f.tasks))
	for k, task := range f.tasks {
		if task.Usage == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := f.tasks[k]
		params := make([]string, len(t.Params))
		for i, param := range t.Params {
			params[i] = flagName(param.Name())
		}
		sort.Strings(params)
		paramsText := ""
		if len(params) > 0 {
			paramsText = "; " + strings.Join(params, " ")
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s%s\n", t.Name, t.Usage, paramsText)
	}
	_ = w.Flush()

	if f.defaultTask.name != "" {
		writeLinef(out, "Default task: %s", f.defaultTask.name)
	}
}
