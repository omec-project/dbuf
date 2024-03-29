<!--
Copyright 2021-present Open Networking Foundation
SPDX-License-Identifier: Apache-2.0
-->

# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement. You (or your employer) retain the copyright to your contribution,
this simply gives us permission to use and redistribute your contributions as
part of the project. Head over to <https://cla.opennetworking.org/> to see your
current agreements on file or to sign a new one.

You generally only need to submit a CLA once, so if you've already submitted one
(even if it was for a different project), you probably don't need to do it
again.

## Submitting Code

### General Information

This project follows [Google's Engineering Practices](https://google.github.io/eng-practices/review/developer/).
Use this document as a guide when submitting code and opening PRs.

Some additional points:

- Submit your changes early and often. Add the `WIP` label or prefix your PR
  title with `[WIP]` to signal that the PR is still work-in-progress, and it's
  not ready for final review. Input and corrections early in the process prevent
  huge changes later.

- We follow the standard [Go Style Guidelines](https://golang.org/cmd/gofmt/)
  for Go. Configure your editor appropriately or run `go fmt ./...` yourself.

- When merging a PR (only project maintainers can) please use
  [squash and rebase](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-request-merges#squash-and-merge-your-pull-request-commits).
  You do **not** have to do this by hand! GitHub will guide you through it, if
  possible.

- For big changes, consider opening a separate issue describing the technical 
  details there and
  [link it to the PR](https://help.github.com/en/github/managing-your-work-on-github/closing-issues-using-keywords).
  This keeps code review and design discussions clean.

### Steps to Follow

1. Fork dbuf into your personal or organization account via the fork
   button on GitHub.

2. Make your code changes.

3. Pass all tests locally (see [README.md](./README.md)). Create new tests for
   new code.

4. Create a [Pull Request](https://github.com/omec-project/dbuf/compare).
   Consider [allowing maintainers](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/allowing-changes-to-a-pull-request-branch-created-from-a-fork)
   to make changes if you want direct assistance from maintainers.

5. Wait for CI checks to pass. Repeat steps 2-4 as necessary. **Passing CI is
   mandatory.** If the CI check does not run automatically, reach out to the
   project maintainers to enable CI jobs for your PR.

6. Await review. Everyone can comment on code changes, but only Collaborators
   and above can give final review approval. **All changes must get at least one
   approval**.

## Community Guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google.com/conduct/)
and ONF's
[Code of Conduct](https://www.opennetworking.org/wp-content/themes/onf/img/onf-code-of-conduct.pdf).
