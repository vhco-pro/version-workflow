package tests

import (
	"regexp"
	"strings"
	"testing"
)

const tagDecision = "steps.gate.outputs.tag"

func TestOnlyTheDefaultBranchOrALabelledPRHeadTags(t *testing.T) {
	w, _ := load(t, "../.github/workflows/version.yml")
	steps := w.Jobs["version"].Steps
	preview := steps[index(t, steps, id("preview"), "preview")]
	gate := steps[index(t, steps, id("gate"), "gate")]

	for _, condition := range []string{
		"!inputs.dry-run", "inputs.preview-label != ''",
		"github.ref_name != github.event.repository.default_branch",
	} {
		if !strings.Contains(preview.If, condition) {
			t.Errorf("the PR lookup must be skipped unless %s", condition)
		}
	}
	for _, part := range []string{`--head "$BRANCH"`, "--state open", `--label "$LABEL"`, `.headRefOid == \"$SHA\"`} {
		if !strings.Contains(preview.Run, part) {
			t.Errorf("a preview needs an open, labelled PR whose head is this commit: missing %s", part)
		}
	}
	if preview.Env["SHA"] != "${{ github.sha }}" || preview.Env["BRANCH"] != "${{ github.ref_name }}" ||
		preview.Env["LABEL"] != "${{ inputs.preview-label }}" {
		t.Error("the PR lookup must compare this run's branch, commit and label")
	}
	want := "${{ !inputs.dry-run && (github.ref_name == github.event.repository.default_branch || steps.preview.outputs.labelled == 'true') }}"
	if gate.Env["TAG"] != want {
		t.Errorf("tag decision is %q, want %q", gate.Env["TAG"], want)
	}
}

func TestEverythingThatWritesFollowsTheTagDecision(t *testing.T) {
	w, _ := load(t, "../.github/workflows/version.yml")
	steps := w.Jobs["version"].Steps
	gate := index(t, steps, id("gate"), "gate")
	app := index(t, steps, uses("actions/create-github-app-token"), "App token")
	checkout := index(t, steps, uses("actions/checkout"), "checkout")
	version := index(t, steps, uses("michielvha/gitversion-tag-action"), "GitVersion")
	if !(gate < app && app < checkout && checkout < version) {
		t.Fatal("order must be: decide, authenticate, check out, version")
	}
	if steps[app].If != "${{ "+tagDecision+" == 'true' && inputs.release-app-client-id != '' }}" ||
		steps[app].With["repositories"] != "${{ github.event.repository.name }}" ||
		steps[app].With["permission-contents"] != "write" {
		t.Error("an App token is optional, minted only to tag, and limited to this repository's contents")
	}
	if _, pinned := steps[checkout].With["ref"]; pinned {
		t.Error("checkout must not set ref: without it the branch is held at the commit that was checked")
	}
	if steps[checkout].With["fetch-depth"] != "0" ||
		steps[checkout].With["persist-credentials"] != "${{ "+tagDecision+" == 'true' }}" {
		t.Error("GitVersion needs full history, and credentials persist only when the run tags")
	}
	if steps[version].With["dry-run"] != "${{ "+tagDecision+" != 'true' }}" ||
		steps[version].With["configFilePath"] != "${{ inputs.config-file-path }}" {
		t.Error("GitVersion must tag exactly when the decision says so")
	}
}

func TestVersionWorkflowStaysThin(t *testing.T) {
	w, raw := load(t, "../.github/workflows/version.yml")
	job := w.Jobs["version"]
	if !w.Permissions.IsZero() || !job.Permissions.IsZero() {
		t.Error("permissions are inherited from the caller job, which picks the arrangement")
	}
	for _, forbidden := range []string{"github-script@", "workflow_run", "git tag ", "git push "} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("GitVersion owns versions and tags; the workflow must not contain %q", forbidden)
		}
	}
	for _, s := range w.Jobs["version"].Steps {
		if lines := strings.Count(strings.TrimSpace(s.Run), "\n") + 1; s.Run != "" && lines > 2 {
			t.Errorf("step %q has a %d-line script; logic belongs in an action", s.ID, lines)
		}
		if s.Uses != "" && !regexp.MustCompile(`@[0-9a-f]{40}$`).MatchString(s.Uses) {
			t.Errorf("%s is not pinned to a commit", s.Uses)
		}
	}
	version := w.Jobs["version"].Steps[index(t, w.Jobs["version"].Steps, uses("michielvha/gitversion-tag-action"), "GitVersion")]
	for output, source := range map[string]string{"tagged": "tagged", "tag": "pushed-tag", "version": "semVer"} {
		if w.Jobs["version"].Outputs[output] != "${{ steps."+version.ID+".outputs."+source+" }}" ||
			w.On.WorkflowCall.Outputs[output].Value != "${{ jobs.version.outputs."+output+" }}" {
			t.Errorf("the %s output must come from GitVersion's %s", output, source)
		}
	}
}
