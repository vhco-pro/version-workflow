# version-workflow

Reusable workflow that versions a repository with GitVersion and pushes the
exact tag, after the caller's checks have passed. The tag is what starts a
release; this workflow publishes nothing itself.

It calls [`michielvha/gitversion-tag-action`](https://github.com/michielvha/gitversion-tag-action),
which with its configuration owns the version and the tag push. Around it, the
workflow adds only what every caller would otherwise copy:

- **When to tag.** The default branch tags. Another branch tags a prerelease
  only when an open PR from it has the `release:preview` label at this commit.
  Every other run, and every `pull_request` run, calculates the version without
  tagging.
- **Who pushes.** An optional GitHub App token, minted only for a run that tags.
- **What is tagged.** The commit the caller's checks ran on, even if the branch
  has moved since.

## Adopt it

Copy [examples/tag-after-checks](examples/tag-after-checks) into the
repository's `.github/workflows/`, replace `FULL_COMMIT_SHA` with a released
commit of this repository, and replace the `checks` job with the repository's
own. `preview.yml` reruns CI when the label is added, because adding a label
pushes nothing.

The example uses a GitHub App, so its tag push starts the repository's
tag-triggered release workflow. Without an App, the caller job grants
`contents: write`, the caller's `GITHUB_TOKEN` pushes the tag, and a dependent
job in the same run releases the `tag` output, because a `GITHUB_TOKEN` push
starts no workflow. [vhco-pro/go-release-workflow](https://github.com/vhco-pro/go-release-workflow)
has complete examples of both arrangements.

## Contract

Inputs and outputs are defined in [version.yml](.github/workflows/version.yml):
`dry-run`, `preview-label` (empty disables previews), `config-file-path` and
`release-app-client-id` in, with the `release-app-private-key` secret;
`tagged`, `tag` and `version` out. The workflow declares no permissions: the
caller job grants `pull-requests: read`, and `contents: read` with an App or
`contents: write` without one.

## This repository

[release.yml](.github/workflows/release.yml) runs [test.yml](.github/workflows/test.yml)
first, then releases this repository with
[vhco-pro/gh-release-workflow](https://github.com/vhco-pro/gh-release-workflow).
The tests lint the workflow and the examples with actionlint, check its contract
with `go test ./...`, and run it: on a PR, the real tag decision runs and has
to decide not to tag. Locally: `go tool actionlint .github/workflows/*.yml examples/*/*.yml`
and `go test ./...`.
