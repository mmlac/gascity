# S2 Handoff — beads ConditionalWriter machinery

Pick-up doc for the next session. Stage 1 of the `internal/rollout` feature-flag
subsystem is **complete**; this hands off **S2** (land the beads compare-and-swap
machinery in `internal/beads`, with zero consumers — still inert).

---

## KICKOFF PROMPT (paste this to start the next session)

> Continue the gascity feature-flag rollout on branch `worktree-reconciler`. Stage 1
> is complete (PR-1b `62e4c6828`, PR-1c `c2f5a3252`, PR-1a `05039d399`, PR-1d
> `7b93731d3` — all local/unpushed, inert). Now build **S2** (the beads
> ConditionalWriter machinery, PR-S2a then PR-S2b) per
> `engdocs/plans/feature-flags/EXECUTION-PLAN.md` (S2 section, lines ~62-251;
> acceptance gates ~758-767) and this handoff `engdocs/plans/feature-flags/S2-HANDOFF.md`.
> The Opus surface exploration is already done (captured below) and the three open
> design decisions are RESOLVED (below). Follow the process: Fable design pass →
> TDD build → **Fable red-team workflow before every commit** → full gates → commit.
> S2 stays inert (no consumers; wire byte-untouched). Do NOT start S3 without
> checking in — S3 has outward-facing steps (deploy-lineage sync + the live
> maintainer-city flip). Do NOT push; the stack is a local integration branch.

---

## Status

- **Branch:** `worktree-reconciler` at `/data/projects/gascity/.claude/worktrees/reconciler`. No upstream, UNPUSHED. Do not push (local integration stack).
- **Stage 1 (done, inert, all red-teamed):**
  - PR-1b `62e4c6828` — `internal/rollout` foundation.
  - PR-1c `c2f5a3252` — composition-root boot-latch + `gc doctor` gates.
  - PR-1a `05039d399` — prompt-boundary lint (`cmd/gc/prompt_rollout_boundary_test.go`).
  - PR-1d `7b93731d3` — freeze / env-baseline / graduation / RetiredKey teeth.
- **Two untracked dirs are NOT mine** — `engdocs/plans/beads-cas/`, `engdocs/plans/reconciler-redesign/`. Leave them; exclude from any `git add`.

## S2 goal + PR breakdown

Land the full ConditionalWriter machinery in `internal/beads` with **zero
consumers** (S3 wires C4/C6). Ships as two mergeable PRs:

- **PR-S2a** = S2-T1..T8: interface + typed errors + internal revision; the
  conformance harness (written first, red until impls land); Mem/File impls; the
  bd-message classifier; the capability probe/latch; BdStore CAS verbs + a
  dedicated CAS retry policy (NOT the blind transient loop); bounded metadata-CAS
  emulation; CachingStore forward-and-EVICT (the livelock regression is a MERGE
  GATE). No S1 dependency.
- **PR-S2b** = S2-T10..T12: factory mode-stamp via a `ModeStamped` optional
  interface + `ResolveConditionalWriter(store)` thin adapter over the GENERAL
  `rollout` resolver (NO mode branching in `internal/beads` outside the stamp);
  the `beads.conditional_writes.degraded` event REGISTERED-only (emission is S3);
  the integration conformance row. Depends on PR-1c (merged) + PR-S2a.

Plan detail: `EXECUTION-PLAN.md` S2 section (lines ~62-251) and S2 acceptance
gates (~758-767). Typed-error semantics: `DESIGN.md` §8.1 (verbatim). Metadata-CAS
emulation spike: `DESIGN.md` §8.4.

## RESOLVED design decisions (from the user)

1. **Internal revision storage — do the EASIEST thing (it all merges in S4).**
   Add `Revision int64 \`json:"-"\`` to `beads.Bead`. `json:"-"` makes Huma skip it,
   so `TestOpenAPISpecInSync` + `genclient` stay byte-untouched (S2 exit gate #1).
   It sits on the struct where S4 wants it — S4-T5 just flips the tag to
   `json:"revision,omitempty"` + regenerates the spec/client. Stores populate it
   internally: BdStore reads it from bd JSON when present (pre-#4682 bd omits it →
   0); Mem/File maintain a per-bead counter bumped on every mutation. Expose via an
   unexported accessor + `PreconditionFailedError.Current`. (This is simpler than a
   side-map and discards nothing — chosen over the plan's "side-map/decode-envelope"
   wording per the user's "easiest, all merges together" directive.)

2. **bd error classifier — message-substring matching (no numeric exit path).**
   BdStore has NO exit-9/13 code path today; every classifier
   (`isBdTransientWriteError`, `isBdNotFound`, …) matches on the error message
   string. Do the SAME for CAS: a pure, table-tested function classifying a bd
   error into {precondition-failed → `PreconditionFailedError`, gate-refusal →
   `GateRefusalError`, transient → CAS-retry, unsupported → `ErrConditionalWriteUnsupported`}
   by message substrings, driven by the scripted `fakeRunner` double
   (`bdstore_test.go:21`). `*exec.ExitError.ExitCode()` is available but the user
   confirmed message-substring is the pragmatic answer. The plan's "exit-9/exit-13
   classifier" is a misnomer for this codebase — implement it message-based and say
   so in the PR.

3. **Method names + CachingStore — do the MOST NATURAL thing.**
   - The Store interface is `Update`/`Close`/`Delete` (NOT `UpdateIssue*`). Name the
     CAS methods to match: `UpdateIfMatch(id string, expectedRevision int64, opts UpdateOpts) error`,
     `CloseIfMatch(id string, expectedRevision int64) error`,
     `DeleteIfMatch(id string, expectedRevision int64) error`,
     `CompareAndSetMetadataKey(id, key, expected, next string) (bool, error)`.
   - Model the `ConditionalWriter` optional interface on
     `ConditionalAssignmentReleaser` (`beads.go:109`) and mirror the
     `GraphApplyStore` + `GraphApplyFor(store)` resolver-helper idiom
     (`graph_apply.go:11-24`) — add `ConditionalWriterFor(store) (ConditionalWriter, bool)`
     that tries a direct assertion then a delegated-handle provider (so wrappers
     forward without claiming the interface globally). Optional capabilities are NOT
     promoted through embedded `Store` wrappers (`class_store.go:14-20`) — forward
     explicitly.
   - CachingStore: on a successful CAS, **EVICT** the cache entry (invalidate via the
     `dirty`/`deletedSeq`/`markFreshLocked`/`clearDependentReadyProjectionsLocked`
     machinery), never patch it like the current `ReleaseIfCurrent`
     (`caching_store_writes.go:138-180`). The livelock regression test (evict on
     CAS-success-refresh-failed AND every PreconditionFailed; retry converges) is a
     MERGE GATE.

## Surface map (from the Opus exploration — file:line)

- **`beads.Bead`** struct: `beads.go:48-93`. **Store interface:** `beads.go:337-443`
  (`Update`/`Close`/`Delete`/`SetMetadata`/`SetMetadataBatch`/`Tx`). **`UpdateOpts`:**
  `beads.go:96-107`.
- **Model interface** `ConditionalAssignmentReleaser`: `beads.go:109-114`. Idiom to
  mirror: `GraphApplyStore` + `GraphApplyFor`: `graph_apply.go:11-24`. `AtomicTxStore`
  + `StoreSupportsAtomicTx` free-function helper: `beads.go:123-131`.
- **Existing sentinels** to sit beside: `beads.go:13-46` (`ErrNotFound`,
  `ErrConditionalReleaseUnsupported` at :38, …). Typed-error structs: `PartialResultError`
  (`bdstore.go:633`), `LookupLimitError` (`lookup_limit_error.go:9`) — the `IsX(err)`
  helper convention.
- **MemStore:** `memstore.go:15-20`; writes `Update:112`/`Close:191`/`Delete:435`;
  CAS precedent `ReleaseIfCurrent:171`; NO revision field today; toggle goes on the
  struct + a `NewMemStore` option/setter. **FileStore:** `filestore.go:24-31`; every
  write is reload→snapshot→delegate-to-MemStore→save→rollback (`Update:209`,
  `Close:256`, `Delete:302`, `ReleaseIfCurrent:232`); persisted shape `fileData:15`
  serializes `[]Bead` so a Bead field flows to disk.
- **BdStore:** `bdstore.go:294-305`; `CommandRunner` seam `bdstore.go:27`; the blind
  transient loop to AVOID `runBDTransientWriteOutputWhen:1802` (budget 3, `:308`);
  bd CAS precedent `ReleaseIfCurrent:1097` (issues `bd sql --json UPDATE … WHERE …
  AND status/assignee`, reads rows_affected, embedded-dolt fallback `:1118`, SQL
  literal escaping `bdSQLStringLiteral:1263`). Message classifiers to extend:
  `isBdTransientWriteError:1873`, `isBdNotFound:812`, `isBdSQLUnsupportedInEmbeddedMode:1253`.
- **CachingStore:** `caching_store.go:28-84`; the patch-path to REPLACE with evict:
  `caching_store_writes.go:138-180`; graph-apply forwarding idiom
  `caching_store_graph_apply.go:12-53`.
- **Factory / mode-stamp attach point:** `StoreOpenOptions` (`factory.go:52-63`) +
  `StoreOpenResult`/`BeadsDiagnostic` (`factory.go:45-69`); `OpenStoreAtForCity:77`;
  production wiring `cmd/gc/main.go:1203`, `cmd/gc/api_state.go:344`.
- **Conformance harness:** `internal/beads/beadstest/conformance.go` —
  `RunStoreTests(t, newStore):31` → `RunStoreTestsWithOptions:38`; `Options:21` gated
  by the skip ledger (`conformance_skips.go`, needs a tracking bead + expiry). Store
  factory tables: `memstore_test.go:13`, `filestore_test.go:89`,
  `native_dolt_store_conformance_test.go:11`. Scripted bd double:
  `fakeRunner` (`bdstore_test.go:21`). Recording double: `beadstest/recording_store.go`.
  NO CAS subtest exists yet — S2-T2 adds the first here.
- **Events:** `RegisterPayload` (`internal/events/payload.go:51`), `KnownEventTypes`
  (`internal/events/events.go:212`), bead payloads register in
  `internal/api/event_payloads.go:519`. `OnConditionalWritesDegraded` nil-safe
  callback SHAPE goes on `StoreOpenOptions` (`factory.go:52`), wired in S3.
- **WIRE GUARD (critical):** `beads.Bead` IS the Huma response type (no separate DTO;
  `huma_handlers_beads.go:211/394`); `TestOpenAPISpecInSync` (`openapi_sync_test.go:25`)
  fails on any drift. Hence Decision 1's `json:"-"`.

## Process (the user's standing method)

1. **Fable design pass** (fable model) over PR-S2a: resolve the conformance-suite
   shape, the classifier table, the CAS retry policy, the revision-population per
   store, and the ConditionalWriter/resolver signatures — grounded in the decisions
   above. No code.
2. **TDD build** in the main loop: conformance suite first (red), then impls green.
   Phase in ≤5-file chunks; verify each.
3. **Fable red-team workflow BEFORE every commit** (this is non-negotiable — it has
   repeatedly caught real defects: seam leaks, contract lies, false-negatives). Fold
   confirmed findings; document accepted residue.
4. **Full gates** then commit. Commit message ends with:
   `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

## Gates / verification

- `go build ./internal/beads/...`
- `go test ./internal/beads/...` (includes conformance) — **run the FULL package test**, not just `-run`; the pre-commit hook does NOT run package guards, so latent failures (e.g. the PR-1b stray-import lint) only show in a full run.
- `go vet ./internal/beads/...`
- `golangci-lint run ./internal/beads/...`  (fleet lock may say "parallel golangci-lint is running" — just retry)
- `gofumpt -l <changed files>` (binary at `/home/ubuntu/go/bin/gofumpt`)
- **Wire gate:** `go test ./internal/api/ -run 'OpenAPISpecInSync|EventPayload'` must stay green (proves Decision 1 kept the wire byte-untouched). Also `make spec-ci` for the S2-T11 event-registration commit.
- The pre-commit hook runs lint-changed + doc-gen + vet + docsync automatically on commit; it DID work on this worktree.

## Gotchas (learned this stack)

- **NEVER `git checkout <file>` to revert a mutation-test edit when the file has UNCOMMITTED legit changes** — it wipes them all. Use Edit-back or a throwaway file you `rm`.
- **Dolt is LOCAL-ONLY** — `git push` only; never `bd dolt push/pull/remote`.
- **Red-team on the shared worktree races** — verify agents must isolate (`git archive HEAD` to a tmpdir + copy untracked files) or work read-only; in-tree mutation by parallel agents corrupts each other.
- **New test package** needs a generated `testenv_import_test.go` (`go run scripts/add-testenv-import.go`); and a non-testenv test file must NOT import `internal/testenv` (stray-import lint) — put testenv-coupled checks in `internal/testenv` (dir is lint-exempt).
- **`go clean -cache` is banned**; cold build via `GOCACHE=$(mktemp -d) go build ...`.
- **Pre-existing, unrelated:** `TestStreamSessionPeekAcceptsPeekCapability` fails under `-race` in `internal/api` (bd `ga-69hv8k`) — NOT the rollout work; filter it out of race sweeps.
- **Deferred:** the `internal/prompt` extraction is bd `ga-b1ii8y` (post-S3).

## After S2

S3 = PR-S3-main (C6 drain reservation + C4 attach epoch fence → CAS, observability,
runbook) + PR-S3-deploy (sqlite ConditionalWriter + the 5-leg integration test on the
deploy lineage). **Checkpoint with the user before S3**: S3.0b needs a deploy-lineage
sync against `deploy/sqlite-b36-probe-attribution` (the live maintainer-city store
shape), and S3.8 flips maintainer-city to `conditional_writes=auto` — both outward-facing.
S4+ (bd library bump + C2 wire) is HARD-GATED on a tagged beads release carrying
beads#4682 (still untagged).
