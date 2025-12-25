package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "new":
		if err := runNew(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "dev":
		if err := runDev(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "make:model":
		if err := runMakeModel(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "make:controller":
		if err := runMakeController(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  jimo new <project-name> [--module <module-path>] [--repo <git-url>] [--branch <branch>]")
	fmt.Fprintln(os.Stderr, "  jimo serve [--port <port>] [--cmd <path>] ")
	fmt.Fprintln(os.Stderr, "  jimo dev [--port <port>] [--cmd <path>]")
	fmt.Fprintln(os.Stderr, "  jimo make:model <Name>")
	fmt.Fprintln(os.Stderr, "  jimo make:controller <Name> [--api] [--resource]")
}

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	module := fs.String("module", "", "Go module path for the new project (default: project name)")
	repo := fs.String("repo", "https://github.com/jimo-go/jimo.git", "Skeleton repository URL")
	branch := fs.String("branch", "main", "Skeleton repository branch")

	projectName, flagArgs, err := splitProjectArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if projectName == "" {
		return errors.New("missing <project-name>")
	}

	projectDir := projectName

	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("target directory already exists: %s", projectDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := runCmd("git", "clone", "--depth", "1", "--branch", *branch, *repo, projectDir); err != nil {
		return err
	}

	if err := os.RemoveAll(filepath.Join(projectDir, ".git")); err != nil {
		return err
	}

	mod := strings.TrimSpace(*module)
	if mod == "" {
		mod = projectName
	}
	if err := rewriteGoMod(projectDir, mod); err != nil {
		return err
	}
	if err := rewriteImports(projectDir, "github.com/jimo-go/jimo", mod); err != nil {
		return err
	}

	fmt.Printf("Created %s\n", projectDir)
	fmt.Printf("Next:\n")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Printf("  jimo serve\n")
	return nil
}

func splitProjectArgs(args []string) (projectName string, flagArgs []string, err error) {
	// Allow flags anywhere:
	// - jimo new myapp --module x
	// - jimo new --module x myapp
	pos := make([]string, 0, 1)
	flags := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		pos = append(pos, a)
	}

	if len(pos) > 1 {
		return "", nil, fmt.Errorf("too many arguments: %v", pos)
	}
	if len(pos) == 1 {
		projectName = pos[0]
	}
	return projectName, flags, nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	port := fs.String("port", "", "Port to listen on (sets PORT env var)")
	cmdPath := fs.String("cmd", "./cmd/server", "Path to the server package to run")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := exec.Command("go", "run", *cmdPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if p := strings.TrimSpace(*port); p != "" {
		cmd.Env = append(cmd.Env, "PORT="+p)
	}
	return cmd.Run()
}

func runDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	port := fs.String("port", "", "Port to listen on (sets PORT env var)")
	cmdPath := fs.String("cmd", "./cmd/server", "Path to the server package")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := exec.LookPath("air"); err != nil {
		return errors.New("air is not installed; install it with: go install github.com/air-verse/air@latest")
	}

	if err := ensureAirToml(*cmdPath); err != nil {
		return err
	}

	cmd := exec.Command("air", "-c", ".air.toml")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if p := strings.TrimSpace(*port); p != "" {
		cmd.Env = append(cmd.Env, "PORT="+p)
	}
	return cmd.Run()
}

func rewriteGoMod(projectDir, modulePath string) error {
	path := filepath.Join(projectDir, "go.mod")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(b), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "module ") {
			out = append(out, "module "+modulePath)
			continue
		}
		if strings.HasPrefix(trim, "replace github.com/jimo-go/framework =>") {
			continue
		}
		out = append(out, line)
	}

	content := strings.Join(out, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func rewriteImports(projectDir, fromModule, toModule string) error {
	from := strings.TrimSuffix(strings.TrimSpace(fromModule), "/") + "/"
	to := strings.TrimSuffix(strings.TrimSpace(toModule), "/") + "/"
	if from == to {
		return nil
	}

	return filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		in := string(b)
		out := strings.ReplaceAll(in, from, to)
		if out == in {
			return nil
		}
		return os.WriteFile(path, []byte(out), 0o644)
	})
}

func ensureAirToml(cmdPath string) error {
	path := ".air.toml"
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	cmdPath = strings.TrimSpace(cmdPath)
	if cmdPath == "" {
		cmdPath = "./cmd/server"
	}

	content := "root = \".\"\n" +
		"tmp_dir = \"tmp\"\n\n" +
		"[build]\n" +
		"  cmd = \"go build -o ./tmp/jimo-dev " + cmdPath + "\"\n" +
		"  bin = \"tmp/jimo-dev\"\n" +
		"  include_ext = [\"go\", \"html\", \"tmpl\", \"tpl\"]\n" +
		"  exclude_dir = [\"tmp\", \"vendor\", \"node_modules\"]\n" +
		"  stop_on_error = true\n\n" +
		"[misc]\n" +
		"  clean_on_exit = true\n"

	return os.WriteFile(path, []byte(content), 0o644)
}

func runMakeModel(args []string) error {
	if len(args) < 1 {
		return errors.New("missing model name")
	}
	name := args[0]
	if name == "" {
		return errors.New("model name cannot be empty")
	}
	dir := "app/models"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file := filepath.Join(dir, strings.ToLower(name)+".go")
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("model already exists: %s", file)
	}
	content := fmt.Sprintf(`package models

import (
	"github.com/jimo-go/framework/database"
)

type %s struct {
	ID int `+"`json:\"id\"`"+`
	// Add fields here
}

func (%s) TableName() string { return "%ss" }

func %ss() *database.Record[%s] {
	return database.Model[%s]()
}
`, name, name, strings.ToLower(name), strings.ToLower(name), name, name)
	return os.WriteFile(file, []byte(content), 0o644)
}

func runMakeController(args []string) error {
	var api, resource bool
	var name string
	// Manual parse to support --api and --resource
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--api":
			api = true
		case "--resource":
			resource = true
		default:
			if name == "" && !strings.HasPrefix(args[i], "-") {
				name = args[i]
			}
		}
	}
	if name == "" {
		return errors.New("missing controller name")
	}
	dir := "app/http/controllers"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file := filepath.Join(dir, strings.ToLower(name)+"_controller.go")
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("controller already exists: %s", file)
	}
	var tmpl string
	if api {
		tmpl = apiControllerTmpl(name)
	} else if resource {
		tmpl = resourceControllerTmpl(name)
	} else {
		tmpl = basicControllerTmpl(name)
	}
	return os.WriteFile(file, []byte(tmpl), 0o644)
}

func basicControllerTmpl(name string) string {
	return fmt.Sprintf(`package controllers

import (
	"github.com/jimo-go/framework"
	jimohttp "github.com/jimo-go/framework/http"
)

type %sController struct{}

func (c *%sController) Index(ctx *jimohttp.Context) {
	ctx.String("Hello from %sController Index")
}

func (c *%sController) Show(ctx *jimohttp.Context) {
	// TODO: implement Show
}
`, name, name, name, name)
}

func apiControllerTmpl(name string) string {
	return fmt.Sprintf(`package controllers

import (
	"github.com/jimo-go/framework"
	jimohttp "github.com/jimo-go/framework/http"
)

type %sController struct{}

func (c *%sController) Index(ctx *jimohttp.Context) {
	ctx.JSON(jimohttp.Map{"message": "%s index"})
}

func (c *%sController) Store(ctx *jimohttp.Context) {
	// TODO: validate input and create
	ctx.JSON(jimohttp.Map{"message": "%s created"})
}

func (c *%sController) Show(ctx *jimohttp.Context) {
	// TODO: fetch and show
	ctx.JSON(jimohttp.Map{"message": "%s show"})
}

func (c *%sController) Update(ctx *jimohttp.Context) {
	// TODO: validate input and update
	ctx.JSON(jimohttp.Map{"message": "%s updated"})
}

func (c *%sController) Destroy(ctx *jimohttp.Context) {
	// TODO: delete
	ctx.JSON(jimohttp.Map{"message": "%s deleted"})
}
`, name, name, name, name, name, name, name, name, name, name, name)
}

func resourceControllerTmpl(name string) string {
	lower := strings.ToLower(name)
	return `package controllers

import (
	"github.com/jimo-go/framework"
	jimohttp "github.com/jimo-go/framework/http"
)

type ` + name + `Controller struct{}

func (c *` + name + `Controller) Index(ctx *jimohttp.Context) {
	// TODO: list ` + lower + `
	ctx.String("List ` + name + `")
}

func (c *` + name + `Controller) Create(ctx *jimohttp.Context) {
	// TODO: show create form
	ctx.String("Create ` + name + ` form")
}

func (c *` + name + `Controller) Store(ctx *jimohttp.Context) {
	// TODO: handle create form submission
	ctx.String("Store ` + name + `")
}

func (c *` + name + `Controller) Show(ctx *jimohttp.Context) {
	// TODO: show single ` + lower + `
	ctx.String("Show ` + name + `")
}

func (c *` + name + `Controller) Edit(ctx *jimohttp.Context) {
	// TODO: show edit form
	ctx.String("Edit ` + name + ` form")
}

func (c *` + name + `Controller) Update(ctx *jimohttp.Context) {
	// TODO: handle edit form submission
	ctx.String("Update ` + name + `")
}

func (c *` + name + `Controller) Destroy(ctx *jimohttp.Context) {
	// TODO: delete ` + lower + `
	ctx.String("Destroy ` + name + `")
}
`
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
