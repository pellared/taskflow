package goyek_test

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/goyek/goyek"
)

func TestCmd_success(t *testing.T) {
	taskName := "exec"
	standard := &strings.Builder{}
	messaging := &strings.Builder{}
	flow := &goyek.Taskflow{
		Output: goyek.Output{
			Standard:  standard,
			Messaging: messaging,
		},
	}
	flow.Register(goyek.Task{
		Name: taskName,
		Action: func(tf *goyek.TF) {
			if err := tf.Cmd("go", "version").Run(); err != nil {
				tf.Fatal(err)
			}
		},
	})

	exitCode := flow.Run(context.Background(), "-v", taskName)

	assertContains(t, standard.String(), "go version go", "output should contain prefix of version report")
	assertEqual(t, exitCode, goyek.CodePass, "task should pass")
}

func TestCmd_error(t *testing.T) {
	taskName := "exec"
	flow := &goyek.Taskflow{
		Output: goyek.Output{
			Standard:  ioutil.Discard,
			Messaging: ioutil.Discard,
		},
	}
	flow.Register(goyek.Task{
		Name: taskName,
		Action: func(tf *goyek.TF) {
			if err := tf.Cmd("go", "wrong").Run(); err != nil {
				tf.Fatal(err)
			}
		},
	})

	exitCode := flow.Run(nil, taskName) //nolint:staticcheck // present that nil context is handled

	assertEqual(t, exitCode, goyek.CodeFail, "task should pass")
}
