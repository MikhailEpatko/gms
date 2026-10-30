---
name: to-tickets
description: Break a plan, spec, or the current conversation into tracer-bullet tickets saved as ai/<N>/ticket-<M>.md, numbered in execution order, each declaring its blocking edges.
disable-model-invocation: true
---

# To Tickets

Break a plan, spec, or conversation into a set of **tickets**: tracer-bullet vertical slices, each declaring the tickets that **block** it.

Tickets live beside their task: `ai/<N>/ticket-<M>.md`, where `ai/<N>/task.md` is the task the conversation is about, `<N>` is that task's number (the `Задача <N>:` header of `task.md`), and `<M>` is the ticket's position in execution order, starting from 1.

## Process

### 1. Gather context

Work from whatever is already in the conversation context. When `ai/<N>/spec.md` exists, read it in full: it is the spec these tickets implement. If the user passes another reference (a spec path, an issue number or URL) as an argument, fetch it and read its full body and comments.

### 2. Explore the codebase (optional)

If you have not already explored the codebase, do so to understand the current state of the code. Ticket titles and descriptions should use the project's domain glossary vocabulary, and respect ADRs in the area you're touching.

Look for opportunities to prefactor the code to make the implementation easier. "Make the change easy, then make the easy change."

### 3. Draft vertical slices

Break the work into **tracer bullet** tickets.

<vertical-slice-rules>

- Each slice cuts a narrow but COMPLETE path through every layer (schema, API, UI, tests): vertical, NOT a horizontal slice of one layer
- A completed slice is demoable or verifiable on its own
- Each slice is sized to fit in a single fresh context window
- Any prefactoring should be done first

</vertical-slice-rules>

Give each ticket its **blocking edges**: the other tickets that must complete before it can start. A ticket with no blockers can start immediately.

**Wide refactors are the exception to vertical slicing.** A **wide refactor** is one mechanical change (rename a column, retype a shared symbol) whose **blast radius** fans across the whole codebase, so a single edit breaks thousands of call sites at once and no vertical slice can land green. Don't force it into a tracer bullet; sequence it as **expand–contract**. First expand: add the new form beside the old so nothing breaks. Then migrate the call sites over in batches sized by blast radius (per package, per directory), each batch its own ticket blocked by the expand, keeping CI green batch to batch because the old form still exists. Finally contract: delete the old form once no caller remains, in a ticket blocked by every migrate batch. When even the batches can't stay green alone, keep the sequence but let them share an integration branch that all block a final integrate-and-verify ticket; green is promised only there.

### 4. Quiz the user

Present the proposed breakdown as a numbered list. For each ticket, show:

- **Title**: short descriptive name
- **Blocked by**: which other tickets (if any) must complete first
- **What it delivers**: the end-to-end behaviour this ticket makes work

Ask the user:

- Does the granularity feel right? (too coarse / too fine)
- Are the blocking edges correct: does each ticket only depend on tickets that genuinely gate it?
- Should any tickets be merged or split further?

Iterate until the user approves the breakdown.

### 5. Write the tickets to `ai/<N>/`

1. Resolve the directory. Take `<N>` from the `Задача <N>:` header of the task file and check that the task file sits in `ai/<N>/`. When the header and the directory name disagree, or no task file is in play, report it to the user and stop.
2. Number the approved tickets in **execution order**: `ticket-1.md` is done first, and every ticket's blockers carry lower numbers than the ticket itself. Executing them top to bottom always respects the blocking edges.
3. When `ai/<N>/` already holds `ticket-*.md` files, read them and confirm with the user before replacing the set; a replaced set leaves no stale higher-numbered files behind.
4. Write one file per ticket with the template below, never a single combined file. "Blocked by" names the blocking tickets by number and title.

Done when every approved ticket has its file and you have listed the paths to the user in execution order.

<ticket-template>

# <M>: <Ticket title>

**What to build:** the end-to-end behaviour this ticket makes work, from the user's perspective, not a layer-by-layer implementation list.

**Blocked by:** the numbers/titles of the tickets that gate this one, or "None (can start immediately)".

**Status:** ready-for-agent

- [ ] Acceptance criterion 1
- [ ] Acceptance criterion 2

</ticket-template>

Write each acceptance criterion as an observable outcome. When a criterion names its test, state what the test observes ("all pair requests are in flight at once") and leave the mechanism to `/tdd`: a prescribed technique, such as comparing wall-clock time, is where flaky tests come from.

Avoid specific file paths or code snippets in tickets: they go stale fast. Exception: if a prototype produced a snippet that encodes a decision more precisely than prose can (state machine, reducer, schema, type shape), inline it and note briefly that it came from a prototype. Trim to the decision-rich parts, not a working demo, just the important bits.
