# Gastei End to End Verification Program

This program connects wacli to the production Gastei WhatsApp bot.
It automates text commands and interactive button selections.
It verifies response latency under one second.
The program ships two PRs in order.
First PR-1 adds the conversational driver and cached recipient resolution.
Second PR-2 adds fast sync start and eliminates button selection delays.

## How to read this

One box is one unit of work. Every box names the evidence that checks it. A nested box is a sub-step of the box above it. Check a box only when its evidence exists, a file, a log line, a screenshot, a test run, or a SHA. The body is a how-to. The appendices explain and record.

The program runs `playbooks/autopilot-stack.md`. The operator reviews PR-2 before merge.

Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

## Program checklist

### Arm the program

- [ ] State the protocol and this plan to the operator, then stop. Start execution only on her explicit go.
- [ ] On her go, arm a `/goal` with this exact text. "docs/gastei-e2e-plan.md, PR-1 and PR-2 in order, tests alone are not sufficient verification, operator lands bottom-up, done when live journey passes against production Gastei."
- [ ] Read these from trunk at program start. Re-read them at every tick.
  - [ ] `git show origin/main:pstack/skills/poteto-mode/playbooks/autopilot-stack.md`
  - [ ] `git show origin/main:pstack/skills/swarm/SKILL.md`
  - [ ] `git show origin/main:pstack/skills/poteto-mode/playbooks/opening-a-pr.md`
- [ ] Arm the 30-minute audit tick. In a local session, a real terminal `/loop`. In a cloud root, a cloud-sleeper wake chain. Never leave the cadence to memory.
- [ ] Use this tick prompt, verbatim. "Re-read the execution playbook from trunk and the armed /goal. Audit the operation against both and fix drift in this tick. Probe every active lane and judge progress by side effects only. Stand down a stuck lane and dispatch its replacement now. Then send the operator a status message, whether or not anything changed, with the queue table of PR, owner, state, and head SHA, the verdicts since the last tick, what merged, open operator gates, and blockers."
- [ ] On the operator hold or stand-down, send every owner a zero-writes order at once.

### Spawn owners

- [ ] Spawn one owner per PR with the full lifecycle the execution playbook names.
- [ ] Follow this dependency graph. Start dependent work only after its parent merges, or base it on the parent branch when the execution playbook stacks.
  - [ ] PR-1 branches from `main`.
  - [ ] PR-2 after PR-1.
- [ ] Hold the file boundaries. PR-1 touches `scripts/e2e-chat*`, `cmd/wacli/send_helpers*`, and `internal/wa/logger*`. PR-2 touches `cmd/wacli/sync*` and `cmd/wacli/send_select*`.
- [ ] Hold the review gate. PR-2 changes live button interaction and waits for operator review.

### PR mechanics, for every PR

- [ ] Resolve the forge once. Default to `gh`. Record any fallback to `gh`. Never require `gt`.
- [ ] Open the PR ready, never draft, with `gh pr create --base <base-branch>`. A stack child targets its parent branch.
- [ ] Run the repo lint and typecheck once before the PR-facing push. Push with hooks on.
- [ ] Run `/deslop` before each commit and `/no-comments` before review.
- [ ] Triage every Bugbot and security-reviewer comment per `../references/bugbot-triage.md`.
- [ ] Rebase onto current trunk before babysit and again before the merge-ready report.

### Verdict and merge, for every PR

- [ ] At the merge-ready head SHA, run the swarm per `pstack/skills/swarm/SKILL.md`. One gates lane. The ten live lanes from the PR Verify live block. The perf lane from its Verify perf block. One audit lane that reads the diff and the receipts and distrusts the PR body.
- [ ] Clean only when every lane is `PASS`. Findings go back to the owner. A new head gets a fresh swarm and a fresh verdict.
- [ ] The root appends the verified head to the base-branch stack and the operator lands it bottom-up.

### Boot recipe, for every live lane

Each live lane runs on its own cloud VM at the PR head. Drive through `control-cli`.

- [ ] `git fetch origin <head-branch> && git checkout <head SHA>`.
- [ ] Start the backend and the surface. Wait for ready.
- [ ] Deliver input only through the control skill commands. Name the read-only diagnostics.
- [ ] Save every screenshot to `/tmp/swarm-<pr-id>/worker-<n>/<slug>.png` and return the paths with the report.

## Add conversational e2e driver and cached lid resolution (PR-1)

**Depends on.** None.

**Files.**

- [ ] Edit `cmd/wacli/send_helpers.go`.
- [ ] Edit `cmd/wacli/send_helpers_test.go`.
- [ ] Edit `internal/wa/logger.go`.
- [ ] Edit `internal/wa/logger_test.go`.
- [ ] Create `scripts/e2e-chat.mjs`.
- [ ] Create `scripts/e2e-chat.test.mjs`.

**Build.**

- [ ] Add cachedLIDResolver to warmupRecipient in `cmd/wacli/send_helpers.go`.
- [ ] Route libsignal error logs to stderr in `internal/wa/logger.go`.
- [ ] Implement conversational driver in `scripts/e2e-chat.mjs`.

**You see.**

- [ ] Node test suite runs `scripts/e2e-chat.test.mjs` with zero failures.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Test e2e chat driver. Run `node --test scripts/e2e-chat.test.mjs`.
- [ ] Test send warmup caching. Run `go test -v ./cmd/wacli -run TestWarmupRecipient`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.6-fast-xhigh` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run test suite at trunk and head. Save `lane-1-regression.png`. Pass when all tests pass.
- [ ] Lane 2. Mock sync daemon startup. Save `lane-2-mock-sync.png`. Pass when ready event appears.
- [ ] Lane 3. Text command dispatch through socket. Save `lane-3-text-socket.png`. Pass when message persists.
- [ ] Lane 4. Interactive prompt detection. Save `lane-4-prompt-detect.png`. Pass when buttons parse cleanly.
- [ ] Lane 5. Button selection dispatch. Save `lane-5-select-dispatch.png`. Pass when select command succeeds.
- [ ] Lane 6. Post-selection reply capture. Save `lane-6-reply-capture.png`. Pass when reply matches expectation.
- [ ] Lane 7. Signal error logging isolation. Save `lane-7-signal-log.png`. Pass when stdout contains zero log noise.
- [ ] Lane 8. Fake daemon timeout handling. Save `lane-8-timeout-handling.png`. Pass when timeout error surfaces cleanly.
- [ ] Lane 9. Malformed NDJSON resilience. Save `lane-9-ndjson-resilience.png`. Pass when parser rejects corrupted lines.
- [ ] Lane 10. End to end report file generation. Save `lane-10-report-output.png`. Pass when JSON report writes to disk.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Round trip execution duration in milliseconds.
- [ ] Probe. Run `node scripts/e2e-chat.test.mjs`.
- [ ] Baseline. Record trunk duration of 120ms first.
- [ ] Rule. Head execution must complete within 200ms.

**Review gate.** None. PR-1 is not review-gated.

**Merge.**

- [ ] Root clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after verdict.
- [ ] Root appends PR-1 to the base branch stack.

## Tune sync latency and automate live production journey (PR-2)

**Depends on.** PR-1.

**Files.**

- [ ] Edit `cmd/wacli/sync.go`.
- [ ] Edit `cmd/wacli/send_select_cmd.go`.
- [ ] Edit `cmd/wacli/send_ipc.go`.

**Build.**

- [ ] Default post-send-wait to zero during IPC delegated button selections.
- [ ] Add fast-start option to `wacli sync` to skip initial history backfill.
- [ ] Run live end to end onboarding conversation against production Gastei.

**You see.**

- [ ] Interactive button response arrives from Gastei in under 1500ms.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Test delegated button selection zero wait. Run `go test -v ./cmd/wacli -run TestDelegatedSelect`.
- [ ] Test sync fast-start flag validation. Run `go test -v ./cmd/wacli -run TestSyncFastStart`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on `grok-4.6-fast-xhigh` at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run mock e2e driver on trunk and head. Save `lane-1-regression.png`. Pass when latency improves.
- [ ] Lane 2. Production daemon handshake. Save `lane-2-live-ready.png`. Pass when ready event returns real JID.
- [ ] Lane 3. Outbound text to Gastei DM. Save `lane-3-live-send.png`. Pass when command returns in under 300ms.
- [ ] Lane 4. Inbound interactive buttons prompt. Save `lane-4-live-buttons.png`. Pass when Gastei buttons parse.
- [ ] Lane 5. Button selection execution. Save `lane-5-live-select.png`. Pass when select command completes.
- [ ] Lane 6. Bot conversational reply. Save `lane-6-live-reply.png`. Pass when onboarding message arrives.
- [ ] Lane 7. End to end latency budget. Save `lane-7-live-latency.png`. Pass when total round trip is under 2000ms.
- [ ] Lane 8. Hostinger API watcher correlation. Save `lane-8-api-watcher.png`. Pass when Dokploy logs match message ID.
- [ ] Lane 9. Chat state read receipt sync. Save `lane-9-read-receipt.png`. Pass when unread count decrements.
- [ ] Lane 10. Clean daemon shutdown. Save `lane-10-clean-shutdown.png`. Pass when stopping event is emitted.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Button selection delegated dispatch latency in milliseconds.
- [ ] Probe. Measure elapsed time of `wacli send select` delegated call.
- [ ] Baseline. Record trunk latency of 2200ms with default wait.
- [ ] Rule. Head latency must not exceed 250ms.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane screenshots into `docs/media/pr-2-review-run.png`.
- [ ] Record a 30 to 60 second video of the live conversation. Save it as `docs/media/pr-2-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for operator click.

**Merge.**

- [ ] Root clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Rebased onto current trunk after verdict.
- [ ] Operator lands PR-1 and PR-2 bottom-up.

## Close the program

- [ ] Every box above is checked with its evidence.
- [ ] Reply to the operator with the report the execution playbook names.

## Appendix A. Prototype evidence

Prototype 1 verified live streaming NDJSON over stdout. The daemon held the store lock and emitted ready and message events in real time.
Prototype 2 verified live message dispatch to production Gastei at `553492009508@s.whatsapp.net`. The Dokploy API log confirmed request processing in 242ms.
Prototype 3 verified button payload decoding from the Gastei prompt into QuickReply structs.

## Appendix B. Alternatives rejected

Direct polling of SQLite messages table was rejected. It introduces artificial sleep delays and races against database writes.
Webhook HTTP server requirement was rejected for local testing. It forces users to expose local ports or run reverse proxies.
External puppeteer automation was rejected. WhatsApp Web changes DOM structures frequently and consumes high memory.

## Appendix C. Risks

Risk 1 is WhatsApp rate limiting during rapid test iterations. The owner throttles automated runs with a minimum three second pause between sends.
Risk 2 is session desynchronization after network disruption. The owner asserts auto-reconnect recovery in the live test driver.

## Appendix D. Links and reading list

Read `docs/sync.md` before editing sync lifecycle logic.
Read `cmd/wacli/send_select_cmd.go` to understand protobuf message assembly for button clicks.
