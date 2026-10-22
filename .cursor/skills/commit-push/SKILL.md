---
name: commit-push
description: >-
  Split reviewed code into Conventional Commits and push them to a new or
  existing branch following the Chimera git workflow. Use when the user
  invokes /commit-push.
disable-model-invocation: true
---

# Commit and push

Split by commits reviewed code by user, and push to new branch or to existing in accord to git-workflow rule.

Invoking `/commit-push` is the review of the current working tree and the permission to create these commits and push them. Do not open a pull request. Do not merge.

## Before the commits

Read `.cursor/rules/git-workflow.mdc`.

In parallel, inspect:

- `git status`
- `git diff` (staged and unstaged)
- `git log` for the recent message style
- `git branch --show-current`

Leave out `.env`, credentials, and generated `allure-results/`. If those are part of the request, warn and stop before committing them.

Do not change git config. Do not use `git add -i`, `git rebase -i`, or `--no-verify`. Do not force-push `develop` or `main`. Do not commit on `main` or `develop`.

## Branch

Keep the current branch when its prefix already matches the work:

- `feature/*` — new behavior, cut from `develop`
- `fix/*` or `bugfix/*` — a defect in existing behavior, cut from `develop` or the current working branch
- `hotfix/*` — a critical production fix, cut from `main`

Git cannot create `feature/<existing-branch>/<sub>` while `feature/<existing-branch>` itself exists. Use a sibling name such as `feature/<name>` or `bugfix/<name>` instead.

When the current branch is `main`, `develop`, or the wrong kind for this diff, create a new kebab-case branch from HEAD with the matching prefix, then commit on it. Do not rename or delete a remote branch unless the user asks.

## Commits

Split the reviewed diff into one commit per concern. Order the commits so each one leaves the tree consistent.

Use the git-workflow message:

`<type>(<scope>): <short summary in imperative mood>`

Types: `feat`, `fix`, `refactor`, `perf`, `docs`, `chore`, `test`.

Pass every message through a HEREDOC. If a hook modifies files, fix the cause and make a new commit. Do not amend unless the user asked and the amend conditions are already met.

## Push

Push with `git push -u origin HEAD`. Do not force-push.

Run `git status` afterward. Report the branch name and the commits that were pushed.
