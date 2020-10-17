package taskflow_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pellared/taskflow"
)

func Example() {
	tasks := &taskflow.Taskflow{}
	task1 := tasks.MustRegister(taskflow.Task{
		Name: "task-1",
		Command: func(tf *taskflow.TF) {
			tf.Logf("one")
		},
	})
	task2 := tasks.MustRegister(taskflow.Task{
		Name: "task-2",
		Command: func(tf *taskflow.TF) {
			tf.Logf("hello")
			tf.FailNow()
			tf.Logf("world")
		},
		Dependencies: taskflow.Deps{task1},
	})
	tasks.MustRegister(taskflow.Task{
		Name: "task-3",
		Command: func(tf *taskflow.TF) {
			tf.Logf("three")
		},
		Dependencies: taskflow.Deps{task2},
	})

	tasks.Execute(context.Background(), "task-3") //nolint // example
	// Output:
	// ===== TASK  task-2
	// hello
	// ----- FAIL: task-2 (0.00s)
}

func Example_verbose() {
	tasks := &taskflow.Taskflow{
		Verbose: true, // move to flags TODO
	}
	task1 := tasks.MustRegister(taskflow.Task{
		Name: "task-1",
		Command: func(tf *taskflow.TF) {
			tf.Logf("one")
		},
	})
	task2 := tasks.MustRegister(taskflow.Task{
		Name: "task-2",
		Command: func(tf *taskflow.TF) {
			tf.Skipf("two")
			tf.Logf("it is skipped")
		},
		Dependencies: taskflow.Deps{task1},
	})
	tasks.MustRegister(taskflow.Task{
		Name: "task-3",
		Command: func(tf *taskflow.TF) {
			tf.Fatalf("hello from " + tf.Name())
			tf.Logf("world")
		},
		Dependencies: taskflow.Deps{task2},
	})

	tasks.Execute(context.Background(), "task-3") //nolint // example
	// Output:
	// ===== TASK  task-1
	// one
	// ----- PASS: task-1 (0.00s)
	// ===== TASK  task-2
	// two
	// ----- SKIP: task-2 (0.00s)
	// ===== TASK  task-3
	// hello from task-3
	// ----- FAIL: task-3 (0.00s)
}

func Test_successful(t *testing.T) {
	ctx := context.Background()
	tasks := &taskflow.Taskflow{
		Output: ioutil.Discard,
	}
	var executed1 int
	task1 := tasks.MustRegister(taskflow.Task{
		Name: "task-1",
		Command: func(*taskflow.TF) {
			executed1++
		},
	})
	var executed2 int
	tasks.MustRegister(taskflow.Task{
		Name: "task-2",
		Command: func(*taskflow.TF) {
			executed2++
		},
		Dependencies: taskflow.Deps{task1},
	})
	var executed3 int
	tasks.MustRegister(taskflow.Task{
		Name: "task-3",
		Command: func(*taskflow.TF) {
			executed3++
		},
		Dependencies: taskflow.Deps{task1},
	})
	got := func() []int {
		return []int{executed1, executed2, executed3}
	}

	tasks.MustExecute(ctx, "task-1")
	require.Equal(t, []int{1, 0, 0}, got(), "should execute task 1")

	tasks.MustExecute(ctx, "task-2")
	require.Equal(t, []int{2, 1, 0}, got(), "should execute task 1 and 2")

	tasks.MustExecute(ctx, "task-1", "task-2", "task-3")
	require.Equal(t, []int{3, 2, 1}, got(), "should execute task 1 and 2 and 3")
}

func Test_dependency_failure(t *testing.T) {
	ctx := context.Background()
	tasks := &taskflow.Taskflow{
		Output: ioutil.Discard,
	}
	var executed1 int
	task1 := tasks.MustRegister(taskflow.Task{
		Name: "task-1",
		Command: func(tf *taskflow.TF) {
			executed1++
			tf.Errorf("it still runs")
			executed1 += 10
			tf.FailNow()
			executed1 += 100
		},
	})
	var executed2 int
	tasks.MustRegister(taskflow.Task{
		Name: "task-2",
		Command: func(*taskflow.TF) {
			executed2++
		},
		Dependencies: taskflow.Deps{task1},
	})
	var executed3 int
	tasks.MustRegister(taskflow.Task{
		Name: "task-3",
		Command: func(*taskflow.TF) {
			executed3++
		},
		Dependencies: taskflow.Deps{task1},
	})
	got := func() []int {
		return []int{executed1, executed2, executed3}
	}

	err := tasks.Execute(ctx, "task-1", "task-2", "task-3")

	assert.Error(t, err, "should return error from first task")
	assert.Equal(t, []int{11, 0, 0}, got(), "should execute task 1")
}