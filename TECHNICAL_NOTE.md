# Technical Note

## 1. Design decision

Describe one decision in your implementation, why you chose it, and its tradeoff.

`Progress` loads attempts in one query and applies the rules in a pure `calculateProgress` on every read; passes derive from the best score, so no state is stored. Tradeoff: read cost grows with history, and rule tests skip the database, so separate PostgreSQL tests cover the wiring.

## 2. Design questions

Brief answers are sufficient. No additional implementation is required.

### Retried requests

How would you enforce globally unique attempt IDs, return the original result for an identical replay, and return 409 for different content under the same ID? Consider simultaneous requests with the same ID.

Add a unique index on `attempt_id` (after resolving existing duplicates) and insert with `ON CONFLICT DO NOTHING RETURNING`. If nothing returns, load the stored row: identical content returns it; different content returns 409. Simultaneous same-ID inserts serialize on the index: the second waits for the first commit, then takes the conflict path.

### Concurrent requests

Two attempts with distinct IDs and scores of 90 and 70 arrive simultaneously for the same learner and lesson.

Can your implementation lose the higher score? What could a simultaneous progress read observe? What should a new read return after both writes have committed? Is any protection needed?

The 90 cannot be lost: attempts are append-only and the best score is an order-independent maximum. A simultaneous read is one snapshot showing neither, either, or both attempts, all valid. After both commit, reads return 90. No protection is needed unless a stored best score is updated read-modify-write rather than with `GREATEST`.

### Redis

Suppose attempt history grows and progress reads become slow.

Would you use Redis? What would you store, how would it stay consistent with PostgreSQL, and what would happen if Redis were unavailable or its data were lost? An alternative approach is welcome.

Not first. A best-score table per learner and lesson, upserted with `GREATEST` in the insert's transaction, stays consistent by construction. If still slow, cache progress per learner with a TTL and delete the key after each commit. If Redis fails or loses data, reads fall back to PostgreSQL, the source of truth; a read racing a write may cache a stale value until the TTL.

## 3. Limitations and production readiness

List any limitations or unfinished work, and what you would verify before production use.

- Retried POSTs store duplicate rows; progress is unaffected because the maximum is idempotent.
- Chi may return non-canonically escaped learner IDs undecoded.
- Tests need the setup database and leave rows (unique IDs isolate runs).
- Verify before production: concurrency and retry tests, query plans on large histories, authentication, metrics.

## 4. Time and AI tools

Time spent: about 45 minutes (16:47–17:32 +07:00).

AI tools used: Claude Code (Claude Opus 5.5) drafted the implementation, tests, and this note; I reviewed, ran, and checked them.
