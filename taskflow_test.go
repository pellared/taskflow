package goyek_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goyek/goyek"
)

func Test_Register_errors(t *testing.T) {
	testCases := []struct {
		desc string
		task goyek.Task
	}{
		{
			desc: "missing task name",
			task: goyek.Task{},
		},
		{
			desc: "invalid dependency",
			task: goyek.Task{Name: "my-task", Deps: goyek.Deps{goyek.RegisteredTask{}}},
		},
		{
			desc: "invalid task name",
			task: goyek.Task{Name: "-flag"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			flow := &goyek.Taskflow{}

			act := func() { flow.Register(tc.task) }

			assertPanics(t, act, "should panic")
		})
	}
}

func Test_Register_same_name(t *testing.T) {
	flow := &goyek.Taskflow{}
	task := goyek.Task{Name: "task"}
	flow.Register(task)

	act := func() { flow.Register(task) }

	assertPanics(t, act, "should not be possible to register tasks with same name twice")
}

func Test_successful(t *testing.T) {
	ctx := context.Background()
	flow := &goyek.Taskflow{}
	var executed1 int
	task1 := flow.Register(goyek.Task{
		Name: "task-1",
		Action: func(*goyek.TF) {
			executed1++
		},
	})
	var executed2 int
	flow.Register(goyek.Task{
		Name: "task-2",
		Action: func(*goyek.TF) {
			executed2++
		},
		Deps: goyek.Deps{task1},
	})
	var executed3 int
	flow.Register(goyek.Task{
		Name: "task-3",
		Action: func(*goyek.TF) {
			executed3++
		},
		Deps: goyek.Deps{task1},
	})
	got := func() []int {
		return []int{executed1, executed2, executed3}
	}

	exitCode := flow.Run(ctx, "task-1")
	requireEqual(t, exitCode, 0, "first execution should pass")
	requireEqual(t, got(), []int{1, 0, 0}, "should execute task 1")

	exitCode = flow.Run(ctx, "task-2")
	requireEqual(t, exitCode, 0, "second execution should pass")
	requireEqual(t, got(), []int{2, 1, 0}, "should execute task 1 and 2")

	exitCode = flow.Run(ctx, "task-1", "task-2", "task-3")
	requireEqual(t, exitCode, 0, "third execution should pass")
	requireEqual(t, got(), []int{3, 2, 1}, "should execute task 1 and 2 and 3")
}

func Test_dependency_failure(t *testing.T) {
	flow := &goyek.Taskflow{}
	var executed1 int
	task1 := flow.Register(goyek.Task{
		Name: "task-1",
		Action: func(tf *goyek.TF) {
			executed1++
			tf.Error("it still runs")
			executed1 += 10
			tf.FailNow()
			executed1 += 100
		},
	})
	var executed2 int
	flow.Register(goyek.Task{
		Name: "task-2",
		Action: func(*goyek.TF) {
			executed2++
		},
		Deps: goyek.Deps{task1},
	})
	var executed3 int
	flow.Register(goyek.Task{
		Name: "task-3",
		Action: func(*goyek.TF) {
			executed3++
		},
		Deps: goyek.Deps{task1},
	})
	got := func() []int {
		return []int{executed1, executed2, executed3}
	}

	exitCode := flow.Run(context.Background(), "task-2", "task-3")

	assertEqual(t, exitCode, 1, "should return error from first task")
	assertEqual(t, got(), []int{11, 0, 0}, "should execute task 1")
}

func Test_fail(t *testing.T) {
	flow := &goyek.Taskflow{}
	failed := false
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			defer func() {
				failed = tf.Failed()
			}()
			tf.Fatal("failing")
		},
	})

	exitCode := flow.Run(context.Background(), "task")

	assertEqual(t, exitCode, 1, "should return error")
	assertTrue(t, failed, "tf.Failed() should return true")
}

func Test_skip(t *testing.T) {
	flow := &goyek.Taskflow{}
	skipped := false
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			defer func() {
				skipped = tf.Skipped()
			}()
			tf.Skip("skipping")
		},
	})

	exitCode := flow.Run(context.Background(), "task")

	assertEqual(t, exitCode, 0, "should pass")
	assertTrue(t, skipped, "tf.Skipped() should return true")
}

func Test_task_panics(t *testing.T) {
	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			panic("panicked!")
		},
	})

	exitCode := flow.Run(context.Background(), "task")

	assertEqual(t, exitCode, 1, "should return error from first task")
}

func Test_cancelation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name: "task",
	})

	exitCode := flow.Run(ctx, "task")

	assertEqual(t, exitCode, 1, "should return error canceled")
}

func Test_cancelation_during_last_task(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			cancel()
		},
	})

	exitCode := flow.Run(ctx, "task")

	assertEqual(t, exitCode, 1, "should return error canceled")
}

func Test_empty_action(t *testing.T) {
	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name: "task",
	})

	exitCode := flow.Run(context.Background(), "task")

	assertEqual(t, exitCode, 0, "should pass")
}

func Test_invalid_args(t *testing.T) {
	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name: "task",
	})

	testCases := []struct {
		desc string
		args []string
	}{
		{
			desc: "missing task name",
		},
		{
			desc: "bad flag",
			args: []string{"-badflag", "task"},
		},
		{
			desc: "bad task name",
			args: []string{"badtask"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			exitCode := flow.Run(context.Background(), tc.args...)

			assertEqual(t, exitCode, 2, "should return error bad args")
		})
	}
}

func Test_help(t *testing.T) {
	flow := &goyek.Taskflow{}
	fastParam := flow.RegisterBoolParam(goyek.BoolParam{
		Name:  "fast",
		Usage: "simulates fast-lane processing",
	})
	a := flow.Register(goyek.Task{
		Name:   "a",
		Params: goyek.Params{fastParam},
		Usage:  "some task",
	})
	flow.DefaultTask = a

	exitCode := flow.Run(context.Background(), "-h")

	assertEqual(t, exitCode, goyek.CodePass, "should return OK")
}

func Test_printing(t *testing.T) {
	sb := &strings.Builder{}
	flow := &goyek.Taskflow{
		Output: sb,
	}
	skipped := flow.Register(goyek.Task{
		Name: "skipped",
		Action: func(tf *goyek.TF) {
			tf.Skipf("Skipf %d", 0)
		},
	})
	flow.Register(goyek.Task{
		Name: "failing",
		Deps: goyek.Deps{skipped},
		Action: func(tf *goyek.TF) {
			tf.Log("Log", 1)
			tf.Logf("Logf %d", 2)
			tf.Error("Error", 3)
			tf.Errorf("Errorf %d", 4)
			tf.Fatalf("Fatalf %d", 5)
		},
	})

	flow.Run(context.Background(), "-v", "failing")

	assertContains(t, sb.String(), "Skipf 0", "should contain proper output from \"skipped\" task")
	assertContains(t, sb.String(), `Log 1
Logf 2
Error 3
Errorf 4
Fatalf 5`, "should contain proper output from \"failing\" task")
}

func Test_concurrent_printing(t *testing.T) {
	testCases := []struct {
		verbose bool
	}{
		{verbose: false},
		{verbose: true},
	}
	for _, tc := range testCases {
		testName := fmt.Sprintf("Verbose:%v", tc.verbose)
		t.Run(testName, func(t *testing.T) {
			sb := &strings.Builder{}
			flow := goyek.Taskflow{
				Output: sb,
			}
			flow.Register(goyek.Task{
				Name: "task",
				Action: func(tf *goyek.TF) {
					ch := make(chan struct{})
					go func() {
						defer func() { ch <- struct{}{} }()
						tf.Log("from child goroutine")
					}()
					tf.Error("from main goroutine")
					<-ch
				},
			})

			var args []string
			if tc.verbose {
				args = append(args, "-v")
			}
			args = append(args, "task")
			exitCode := flow.Run(context.Background(), args...)

			assertEqual(t, exitCode, goyek.CodeFail, "should fail")
			assertContains(t, sb.String(), "from child goroutine", "should contain log from child goroutine")
			assertContains(t, sb.String(), "from main goroutine", "should contain log from main goroutine")
		})
	}
}

func Test_name(t *testing.T) {
	flow := &goyek.Taskflow{}
	taskName := "my-named-task"
	var got string
	flow.Register(goyek.Task{
		Name: taskName,
		Action: func(tf *goyek.TF) {
			got = tf.Name()
		},
	})

	exitCode := flow.Run(context.Background(), taskName)

	assertEqual(t, exitCode, 0, "should pass")
	assertEqual(t, got, taskName, "should return proper Name value")
}

type arrayValue []string

func (value *arrayValue) Set(s string) error {
	return json.Unmarshal([]byte(s), value)
}

func (value *arrayValue) Get() interface{} { return []string(*value) }

func (value *arrayValue) String() string {
	b, _ := json.Marshal(value)
	return string(b)
}

func (value *arrayValue) IsBool() bool { return false }

func Test_params(t *testing.T) {
	flow := &goyek.Taskflow{}
	boolParam := flow.RegisterBoolParam(goyek.BoolParam{
		Name:    "b",
		Default: true,
	})
	intParam := flow.RegisterIntParam(goyek.IntParam{
		Name:    "i",
		Default: 1,
	})
	stringParam := flow.RegisterStringParam(goyek.StringParam{
		Name:    "s",
		Default: "abc",
	})
	arrayParam := flow.RegisterValueParam(goyek.ValueParam{
		Name:     "array",
		NewValue: func() goyek.ParamValue { return &arrayValue{} },
	})
	var gotBool bool
	var gotInt int
	var gotString string
	var gotArray []string
	flow.Register(goyek.Task{
		Name: "task",
		Params: goyek.Params{
			boolParam,
			intParam,
			stringParam,
			arrayParam,
		},
		Action: func(tf *goyek.TF) {
			gotBool = boolParam.Get(tf)
			gotInt = intParam.Get(tf)
			gotString = stringParam.Get(tf)
			gotArray = arrayParam.Get(tf).([]string) //nolint:forcetypeassert // test code, it can panic
		},
	})

	exitCode := flow.Run(context.Background(), "-b=false", "-i", "9001", "-s", "xyz", "-array", "[\"a\", \"b\"]", "task")

	assertEqual(t, exitCode, 0, "should pass")
	assertEqual(t, gotBool, false, "bool param")
	assertEqual(t, gotInt, 9001, "int param")
	assertEqual(t, gotString, "xyz", "string param")
	assertEqual(t, gotArray, []string{"a", "b"}, "array param")
}

func Test_invalid_params(t *testing.T) {
	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name:   "task",
		Action: func(tf *goyek.TF) {},
	})

	exitCode := flow.Run(context.Background(), "-z=3", "task")

	assertEqual(t, exitCode, goyek.CodeInvalidArgs, "should fail because of unknown parameter")
}

func Test_unused_params(t *testing.T) {
	flow := &goyek.Taskflow{}
	flow.DefaultTask = flow.Register(goyek.Task{Name: "task", Action: func(tf *goyek.TF) {}})
	flow.RegisterBoolParam(goyek.BoolParam{Name: "unused"})

	assertPanics(t, func() { flow.Run(context.Background()) }, "should fail because of unused parameter")
}

func Test_param_registration_error_empty_name(t *testing.T) {
	flow := &goyek.Taskflow{}
	assertPanics(t, func() { flow.RegisterBoolParam(goyek.BoolParam{Name: ""}) }, "empty name")
}

func Test_param_registration_error_underscore_name_start(t *testing.T) {
	flow := &goyek.Taskflow{}
	assertPanics(t, func() { flow.RegisterBoolParam(goyek.BoolParam{Name: "_reserved"}) }, "should not start with underscore")
}

func Test_param_registration_error_no_default(t *testing.T) {
	flow := &goyek.Taskflow{}
	assertPanics(t, func() { flow.RegisterValueParam(goyek.ValueParam{Name: "custom"}) }, "custom parameter must have default value factory")
}

func Test_param_registration_error_double_name(t *testing.T) {
	flow := &goyek.Taskflow{}
	name := "double"
	flow.RegisterStringParam(goyek.StringParam{Name: name})
	assertPanics(t, func() { flow.RegisterBoolParam(goyek.BoolParam{Name: name}) }, "double name")
}

func Test_unregistered_params(t *testing.T) {
	foreignParam := (&goyek.Taskflow{}).RegisterBoolParam(goyek.BoolParam{Name: "foreign"})
	flow := &goyek.Taskflow{}
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			foreignParam.Get(tf)
		},
	})

	exitCode := flow.Run(context.Background(), "task")

	assertEqual(t, goyek.CodeFail, exitCode, "should fail because of unregistered parameter")
}

func Test_defaultTask(t *testing.T) {
	flow := &goyek.Taskflow{}
	taskRan := false
	task := flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			taskRan = true
		},
	})
	flow.DefaultTask = task

	exitCode := flow.Run(context.Background())

	assertEqual(t, exitCode, 0, "should pass")
	assertTrue(t, taskRan, "task should have run")
}

func Test_wd_param(t *testing.T) {
	flow := &goyek.Taskflow{}
	beforeDir, err := os.Getwd()
	requireEqual(t, err, nil, "should get work dir before the taskflow")
	dir, cleanup := tempDir(t)
	defer cleanup()
	var got string
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			var osErr error
			got, osErr = os.Getwd()
			requireEqual(t, osErr, nil, "should get work dir from task")
		},
	})

	exitCode := flow.Run(context.Background(), "task", "-wd", dir)
	afterDir, err := os.Getwd()
	requireEqual(t, err, nil, "should get work dir after the taskflow")

	assertEqual(t, exitCode, 0, "should pass")
	assertEqual(t, got, dir, "should have changed the working directory in taskflow")
	assertEqual(t, afterDir, beforeDir, "should change back the working directory after taskflow")
}

func Test_wd_param_invalid(t *testing.T) {
	flow := &goyek.Taskflow{}
	beforeDir, err := os.Getwd()
	requireEqual(t, err, nil, "should get work dir before the taskflow")
	taskRan := false
	flow.Register(goyek.Task{
		Name: "task",
		Action: func(tf *goyek.TF) {
			taskRan = true
		},
	})

	exitCode := flow.Run(context.Background(), "task", "-wd=strange-dir")
	afterDir, err := os.Getwd()
	requireEqual(t, err, nil, "should get work dir after the taskflow")

	assertEqual(t, exitCode, goyek.CodeInvalidArgs, "should not proceed")
	assertEqual(t, taskRan, false, "should not run the task")
	assertEqual(t, afterDir, beforeDir, "should change back the working directory after taskflow")
}

func Test_introspection_API(t *testing.T) {
	flow := &goyek.Taskflow{}
	p := flow.RegisterStringParam(goyek.StringParam{Name: "string", Usage: "text param", Default: "dft"})
	t1 := flow.Register(goyek.Task{Name: "one", Params: goyek.Params{p}})
	flow.Register(goyek.Task{Name: "two", Usage: "action", Deps: goyek.Deps{t1}})

	tasks := flow.Tasks()
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name() < tasks[j].Name() })

	assertEqual(t, len(tasks), 2, "should return all tasks")
	assertEqual(t, tasks[0].Name(), "one", "should first return one")
	assertEqual(t, tasks[0].Params()[0].Name(), "string", "should return param Name")
	assertEqual(t, tasks[0].Params()[0].Usage(), "text param", "should return param Usage")
	assertEqual(t, tasks[0].Params()[0].Default(), "dft", "should return param Default")
	assertEqual(t, tasks[1].Name(), "two", "should next return two")
	assertEqual(t, tasks[1].Usage(), "action", "should return usage")
	assertEqual(t, tasks[1].Deps()[0].Name(), "one", "should return dependency")

	params := flow.Params()

	assertEqual(t, len(params), 3, "should return all parameters, including the out-of-the-box ones")
}

func tempDir(t *testing.T) (string, func()) {
	t.Helper()
	dirName := t.Name() + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	dir := filepath.Join(os.TempDir(), dirName)
	err := os.Mkdir(dir, 0700)
	requireEqual(t, err, nil, "failed to create a temp directory")
	cleanup := func() {
		err := os.RemoveAll(dir)
		assertEqual(t, err, nil, "should remove temp dir after test")
	}
	return dir, cleanup
}
