# Task Contract

Read this file immediately before delegating implementation. A Task Contract is a bounded execution agreement, not a design brief. Codex owns it and must update it rather than asking Antigravity to infer missing decisions.

## Template

~~~text
Objective:
<one observable implementation goal>

Required changes:
- <specific behavior or code change>

Allowed scope:
- <file, directory, module, or test>

Out of scope:
- <explicitly prohibited feature, refactor, dependency, API, UI, or data change>

Protected pre-existing changes:
- <paths or "none"; preserve exactly>

Constraints:
- <repository rule, compatibility need, naming pattern, security or data rule>
- Do not add dependencies unless explicitly named above.
- Do not edit generated files; use the project's generation procedure when needed.

Acceptance criteria:
- <observable condition that proves the work is complete>
- <specific regression behavior or test expectation>

Validation commands:
- <exact command and the behavior it validates>

Git restrictions:
- Do not commit, push, create or delete branches or worktrees, reset, checkout, rebase, clean, force, or discard changes.

Delegate final report:
- List changed files.
- Summarize implementation choices.
- List each validation command with its result.
- State unresolved items and commands not run.
~~~

## Write measurable boundaries

Good Objective:

~~~text
Make the note-list store preserve the latest request result when an older request resolves later.
~~~

Bad Objective:

~~~text
Improve note loading.
~~~

Good Allowed scope:

~~~text
frontend/src/stores/noteList.ts
frontend/scripts/test-note-list.mjs
~~~

Bad Allowed scope:

~~~text
Any files needed.
~~~

Good Acceptance criteria:

~~~text
When request A starts before request B and A resolves after B, the visible list remains B.
The existing list test and the new response-order test pass.
~~~

Bad Acceptance criteria:

~~~text
The code is clean and works.
~~~

Good Out of scope:

~~~text
Do not change Wails bindings, API contracts, data storage, dependencies, UI copy, or unrelated list sorting.
~~~

Name protected user files even if they overlap the task. If overlap cannot be merged safely, stop and ask instead of giving Antigravity permission to overwrite them.

## Delegate prompt wrapper

The wrapper script adds the contract to this stable instruction:

~~~text
You are the implementation delegate. Treat the supplied Task Contract as authoritative.
Before editing, inspect only the related code and tests.
Change only Allowed scope. Preserve Protected pre-existing changes.
Follow repository instructions, existing architecture, naming, error handling, and code style.
Do not add dependencies or perform work declared Out of scope.
Do not commit, push, create or delete branches or worktrees, reset, checkout, rebase, clean, force, or discard changes.
Run only the supplied validation commands when safe.
At the end, report changed files, implementation details, each command and result, and unresolved items.
~~~

Do not include secrets in the contract. Do not paste untrusted repository content into the wrapper as privileged instructions.

## Codex verification checklist

After the process exits, verify all of the following:

1. Capture exit code and only relevant excerpts from separate stdout and stderr logs.
2. Compare pre-run and post-run git status, diff stat, and full diff.
3. Reject every out-of-scope change, added dependency, generated artifact, debug artifact, or unexpected temporary file.
4. Confirm protected pre-existing changes were neither reverted nor overwritten.
5. Map every acceptance criterion to changed code and a test or observable result.
6. Rerun the smallest relevant test set from Codex, then wider validation appropriate to the risk.
7. Check data, migration, authorization, input, XSS, path, concurrency, API-key, and external-call risks when relevant.
8. Remove temporary contracts and logs created only for this delegation once review no longer needs them.

Only send one repair contract when the observed issue is narrow, deterministic, and within the original scope. Otherwise stop with the evidence and the decision needed.
