# Flash-Mall Codex Context

Use this file as the short entry prompt for future project work.

## Goal

Build Flash-Mall into a high-value interview project. Prioritize features that
are easy to explain, defensible under technical follow-up, and backed by tests
or runnable demos.

## Current Direction

- Single Codex owner; MCP/model delegation is only for draft code or analysis.
- Core value: trading loop reliability, payment correctness, account security,
  observability, and one-click demo readiness.
- Keep changes practical: implement, verify, commit, and explain the interview
  value.

## Operating Rules

- Before new work, check `git status --short`.
- If tracked changes remain from the previous task, ask whether to clean up or
  commit before starting a new plan.
- Do not use the retired task-board workflow.
- Use targeted tests first; avoid full heavy test chains unless the change
  justifies them.
- Delegate only low-risk/high-volume drafting. Codex owns security, payment,
  transaction consistency, final review, verification, and commits.

## Context Lookup

- Read `PROJECT_NOTES.md` for the compact project map.
- Read archived docs only when a task needs historical detail.
- Avoid loading all `docs/superpowers/**` plans by default.
