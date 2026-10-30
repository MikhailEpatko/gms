---
name: implement
description: "Implement a piece of work based on a spec or set of tickets: a ticket file path, or the next open ticket in ai/<N>/."
disable-model-invocation: true
---

Implement the work described by the user in the spec or tickets.

Tickets live beside their task as `ai/<N>/ticket-<M>.md`, numbered in execution order; the spec is `ai/<N>/spec.md`. Pick the ticket this way:

- **The user passed a file path** (`ai/1/ticket-2.md`): that file is the ticket.
- **No path**: take the task directory `ai/<N>/` the conversation is about (the user may name it as `ai/1` or `1`; when several task directories exist and none is named, ask). The ticket is the lowest-numbered `ticket-<M>.md` whose `**Status:**` is not `done`. When one of its blockers is not `done`, report that and stop.

State the ticket's title before starting. When `ai/<N>/spec.md` exists, read it: it is the spec the ticket implements.

Call the Skill tool with "tdd" where possible, at pre-agreed seams.

Run typechecking regularly, single test files regularly, and the full test suite once at the end.

Once done, call the Skill tool with "review" to review the work.

Close the ticket in its file: set `**Status:** done` and tick the acceptance criteria the work meets. Leave the work and the updated ticket file uncommitted: the user commits. Commit only when the user asks.
