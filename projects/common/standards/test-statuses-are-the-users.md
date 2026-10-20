# Test statuses are the user's

Test statuses that defer or reclassify failures are issued by the user only; each project declares its framework's
configuration and concrete statuses. No agent adds one, widens one, or moves tests under an existing one on its own. A
Worker that needs a status asks the user through the Orchestrator and waits. A candidate that introduces or extends
such a status without the user's ruling is rejected as a hidden failure (see `failures-are-exposed.md`).

This covers new `deferred` and `regressed` labels; an agent that adds one on its own is judged as committing treason.
Nor may an agent create a folder or file just to defer something.
