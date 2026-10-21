package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type step struct {
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
}

type workflow struct {
	On struct {
		WorkflowCall struct {
			Inputs  map[string]yaml.Node `yaml:"inputs"`
			Secrets map[string]yaml.Node `yaml:"secrets"`
			Outputs map[string]struct {
				Value string `yaml:"value"`
			} `yaml:"outputs"`
		} `yaml:"workflow_call"`
	} `yaml:"on"`
	Permissions yaml.Node `yaml:"permissions"`
	Jobs        map[string]struct {
		Uses        string            `yaml:"uses"`
		With        map[string]string `yaml:"with"`
		Secrets     map[string]string `yaml:"secrets"`
		Permissions yaml.Node         `yaml:"permissions"`
		Outputs     map[string]string `yaml:"outputs"`
		Steps       []step            `yaml:"steps"`
	} `yaml:"jobs"`
}

func load(t *testing.T, path string) (workflow, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var w workflow
	if err := yaml.Unmarshal(raw, &w); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return w, string(raw)
}

func index(t *testing.T, steps []step, match func(step) bool, what string) int {
	t.Helper()
	found := -1
	for i, s := range steps {
		if match(s) {
			if found != -1 {
				t.Fatalf("%s appears more than once", what)
			}
			found = i
		}
	}
	if found == -1 {
		t.Fatalf("no %s step", what)
	}
	return found
}

func uses(action string) func(step) bool {
	return func(s step) bool { return strings.HasPrefix(s.Uses, action+"@") }
}

func id(name string) func(step) bool {
	return func(s step) bool { return s.ID == name }
}

var (
	ownCall   = regexp.MustCompile(`^vhco-pro/version-workflow/\.github/workflows/([\w.-]+\.yml)@`)
	outputRef = regexp.MustCompile(`needs\.(\w+)\.outputs\.([\w-]+)`)
)

// The examples are what callers copy, so they must match the workflow's current
// inputs, secrets and outputs.
func TestExamplesMatchTheWorkflow(t *testing.T) {
	files, err := filepath.Glob("../examples/*/*.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	for _, file := range files {
		c, raw := load(t, file)
		called := map[string]workflow{}
		for name, job := range c.Jobs {
			m := ownCall.FindStringSubmatch(job.Uses)
			if m == nil {
				continue
			}
			if !strings.HasSuffix(job.Uses, "@FULL_COMMIT_SHA") {
				t.Errorf("%s job %s must pin FULL_COMMIT_SHA for the caller to replace", file, name)
			}
			w, _ := load(t, filepath.Join("..", ".github", "workflows", m[1]))
			called[name] = w
			for input := range job.With {
				if _, ok := w.On.WorkflowCall.Inputs[input]; !ok {
					t.Errorf("%s job %s passes unknown input %q", file, name, input)
				}
			}
			for secret := range job.Secrets {
				if _, ok := w.On.WorkflowCall.Secrets[secret]; !ok {
					t.Errorf("%s job %s passes unknown secret %q", file, name, secret)
				}
			}
		}
		for _, ref := range outputRef.FindAllStringSubmatch(raw, -1) {
			if w, ok := called[ref[1]]; ok {
				if _, ok := w.On.WorkflowCall.Outputs[ref[2]]; !ok {
					t.Errorf("%s reads unknown output %s.%s", file, ref[1], ref[2])
				}
			}
		}
	}
}
