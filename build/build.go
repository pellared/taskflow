package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goyek/goyek"
)

func main() {
	flow().Main()
}

func flow() *goyek.Taskflow {
	flow := &goyek.Taskflow{}

	// parameters
	ci := flow.RegisterBoolParam(goyek.BoolParam{
		Name:  "ci",
		Usage: "Whether CI is calling the build script",
	})

	// tasks
	clean := flow.Register(taskClean())
	build := flow.Register(taskBuild())
	fmt := flow.Register(taskFmt())
	markdownlint := flow.Register(taskMarkdownLint())
	misspell := flow.Register(taskMisspell())
	golangciLint := flow.Register(taskGolangciLint())
	test := flow.Register(taskTest())
	modTidy := flow.Register(taskModTidy())
	diff := flow.Register(taskDiff(ci))

	// pipelines
	lint := flow.Register(taskLint(goyek.Deps{
		misspell,
		markdownlint,
		golangciLint,
	}))
	all := flow.Register(taskAll(goyek.Deps{
		clean,
		build,
		fmt,
		lint,
		test,
		modTidy,
		diff,
	}))
	flow.DefaultTask = all

	return flow
}

const buildDir = "build"

func taskClean() goyek.Task {
	return goyek.Task{
		Name:  "clean",
		Usage: "remove git ignored files",
		Action: func(tf *goyek.TF) {
			if err := tf.Cmd("git", "clean", "-fX").Run(); err != nil {
				tf.Fatal(err)
			}
		},
	}
}

func taskBuild() goyek.Task {
	return goyek.Task{
		Name:  "build",
		Usage: "go build",
		Action: func(tf *goyek.TF) {
			if err := tf.Cmd("go", "build", "./...").Run(); err != nil {
				tf.Fatal(err)
			}
		},
	}
}

func taskFmt() goyek.Task {
	return goyek.Task{
		Name:  "fmt",
		Usage: "gofumports",
		Action: func(tf *goyek.TF) {
			installFmt := tf.Cmd("go", "install", "mvdan.cc/gofumpt/gofumports")
			installFmt.Dir = buildDir
			if err := installFmt.Run(); err != nil {
				tf.Fatal(err)
			}

			tf.Cmd("gofumports", strings.Split("-l -w -local github.com/goyek/goyek .", " ")...).Run() //nolint // it is OK if it returns error
		},
	}
}

func taskMarkdownLint() goyek.Task {
	return goyek.Task{
		Name:  "markdownlint",
		Usage: "markdownlint-cli (requires docker)",
		Action: func(tf *goyek.TF) {
			curDir, err := os.Getwd()
			if err != nil {
				tf.Fatal(err)
			}

			docsMount := curDir + ":/markdown"
			if err := tf.Cmd("docker", "run", "--rm", "-v", docsMount, "06kellyjac/markdownlint-cli:0.27.1", "**/*.md").Run(); err != nil {
				tf.Error(err)
			}

			gitHubTemplatesMount := filepath.Join(curDir, ".github") + ":/markdown"
			if err := tf.Cmd("docker", "run", "--rm", "-v", gitHubTemplatesMount, "06kellyjac/markdownlint-cli:0.27.1", "**/*.md").Run(); err != nil {
				tf.Error(err)
			}
		},
	}
}

func taskMisspell() goyek.Task {
	return goyek.Task{
		Name:  "misspell",
		Usage: "misspell",
		Action: func(tf *goyek.TF) {
			installFmt := tf.Cmd("go", "install", "github.com/client9/misspell/cmd/misspell")
			installFmt.Dir = buildDir
			if err := installFmt.Run(); err != nil {
				tf.Fatal(err)
			}

			lint := tf.Cmd("misspell", "-error", "-locale=US", "-i=importas", ".")
			if err := lint.Run(); err != nil {
				tf.Fatal(err)
			}
		},
	}
}

func taskGolangciLint() goyek.Task {
	return goyek.Task{
		Name:  "golangci-lint",
		Usage: "golangci-lint",
		Action: func(tf *goyek.TF) {
			installLint := tf.Cmd("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint")
			installLint.Dir = buildDir
			if err := installLint.Run(); err != nil {
				tf.Fatal(err)
			}

			lint := tf.Cmd("golangci-lint", "run")
			if err := lint.Run(); err != nil {
				tf.Fatal(err)
			}
		},
	}
}

func taskTest() goyek.Task {
	return goyek.Task{
		Name:  "test",
		Usage: "go test with race detector and code covarage",
		Action: func(tf *goyek.TF) {
			if err := tf.Cmd("go", "test", "-race", "-covermode=atomic", "-coverprofile=coverage.out", "./...").Run(); err != nil {
				tf.Fatal(err)
			}
		},
	}
}

func taskModTidy() goyek.Task {
	return goyek.Task{
		Name:  "mod-tidy",
		Usage: "go mod tidy",
		Action: func(tf *goyek.TF) {
			if err := tf.Cmd("go", "mod", "tidy").Run(); err != nil {
				tf.Error(err)
			}

			toolsModTidy := tf.Cmd("go", "mod", "tidy")
			toolsModTidy.Dir = buildDir
			if err := toolsModTidy.Run(); err != nil {
				tf.Error(err)
			}
		},
	}
}

func taskDiff(ci goyek.RegisteredBoolParam) goyek.Task {
	return goyek.Task{
		Name:   "diff",
		Usage:  "git diff",
		Params: goyek.Params{ci},
		Action: func(tf *goyek.TF) {
			if !ci.Get(tf) {
				tf.Skip("ci param is not set, skipping")
			}

			if err := tf.Cmd("git", "diff", "--exit-code").Run(); err != nil {
				tf.Error(err)
			}

			cmd := tf.Cmd("git", "status", "--porcelain")
			sb := &strings.Builder{}
			cmd.Stdout = io.MultiWriter(tf.Output().Standard, sb)
			if err := cmd.Run(); err != nil {
				tf.Error(err)
			}
			if sb.Len() > 0 {
				tf.Error("git status --porcelain returned output")
			}
		},
	}
}

func taskLint(deps goyek.Deps) goyek.Task {
	return goyek.Task{
		Name:  "lint",
		Usage: "all linters",
		Deps:  deps,
	}
}

func taskAll(deps goyek.Deps) goyek.Task {
	return goyek.Task{
		Name:  "all",
		Usage: "build pipeline",
		Deps:  deps,
	}
}
