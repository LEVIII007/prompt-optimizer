# prompt-opt

A GEPA-lite prompt optimization CLI: give it a seed system prompt, a dataset of
test cases, and a scoring rubric, and it iteratively rewrites the prompt using
an LLM-as-judge + reflective-mutation loop, keeping a mutation only when it
scores better. Ends with a seed-vs-best comparison on a held-out validation
slice that the search loop never sees.

## Scope (v1)

This optimizes a single system prompt for single-turn (optionally short
multi-turn) text tasks — classification, extraction, support-style Q&A, RAG
answer drafting, etc. It does **not** cover:

- Tool-calling / agent trajectory evaluation (side-effecting actions).
- Human-label judge calibration (the rubric is trusted as given).
- Statistical significance testing beyond a train/val-gap warning.
- Few-shot example search (MIPROv2-style) — instruction-only mutation only.
- Any deployment hook — it just writes files.

It also uses greedy hill-climbing (one running "best" candidate) rather than
GEPA's full Pareto candidate pool. That's a deliberate v1 simplification.

## Setup

```bash
cp .env.example .env   # fill in your Azure OpenAI credentials
go build ./...
```

## Usage

```bash
go run ./cmd/optimize \
  --seed-prompt testdata/seed_prompt.txt \
  --dataset testdata/dataset.json \
  --rubric testdata/rubric.json \
  --task-deployment gpt-4o-mini \
  --iterations 10
```

Flags (all optional except the three input paths and `--task-deployment`):

| Flag | Default | Meaning |
|---|---|---|
| `--seed-prompt` | — | path to the starting system prompt (text file) |
| `--dataset` | — | path to dataset JSON (see schema below) |
| `--rubric` | — | path to rubric JSON (see schema below) |
| `--out` | `tmp/prompt-opt-<timestamp>` | output directory |
| `--task-deployment` | — | Azure deployment used to run the candidate prompt |
| `--judge-deployment` | = task-deployment | Azure deployment used to score outputs |
| `--reflection-deployment` | = task-deployment | Azure deployment used to propose prompt rewrites |
| `--iterations` | 10 | max optimizer rounds |
| `--minibatch-size` | 8 | examples sampled from train set per round |
| `--val-split` | 0.3 | fraction of dataset held out, frozen, for final comparison |
| `--patience` | 4 | stop early after this many full-train rounds with no improvement |
| `--full-eval-every` | 3 | rounds between full-train-set rechecks (for patience) |
| `--concurrency` | 4 | concurrent LLM calls when scoring a batch |
| `--retries` | 1 | retries per LLM call on error/invalid JSON |
| `--seed` | 42 | RNG seed for the train/val split and minibatch sampling (reproducible reruns) |

### Dataset schema

```json
[
  {
    "id": "case-001",
    "category": "refund_policy",
    "input": "Can I get a refund after 40 days?",
    "reference": "Refunds are only allowed within 30 days of purchase.",
    "history": [{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}],
    "notes": "edge case: just past the window"
  }
]
```

`category` (optional) enables stratified splitting and per-segment reporting.
`reference` (optional) is the ground-truth/policy text the judge checks
groundedness against. `history` (optional) is prior conversation turns.

### Rubric schema

```json
{
  "dimensions": [
    {"name": "policy_accuracy", "description": "States the correct policy, grounded in the reference.", "scale": 1, "weight": 3, "required": true},
    {"name": "tone", "description": "Professional, empathetic, not robotic.", "scale": 5, "weight": 1, "required": false}
  ],
  "pass_threshold": 0.75
}
```

Each dimension is scored `0..scale` by the judge; scores are normalized to
`0..1` and combined into a weighted average. `pass_threshold` is the minimum
weighted average for a response to "pass"; any `required` dimension scoring 0
fails the response regardless of the weighted average.

## Output

- `best_prompt.txt` — the winning candidate (or the seed, if nothing beat it).
- `run_history.json` — every round's candidate, minibatch score, accept/reject,
  and the reflection model's failure-pattern analysis.
- `comparison_report.json` — seed vs. best on the frozen validation set:
  aggregate scores, per-category breakdown, per-example scores, train/val gap.
- `report.md` — human-readable rendering of the above.

## Testing

`go test ./...` covers the deterministic logic (split, rubric validation,
judge JSON parsing, score aggregation, accept/reject, report math) against a
scripted mock chat model — it does not call Azure OpenAI. It cannot tell you
whether the optimizer improves real prompts; that only shows up once you run
it against real credentials and real data.
# prompt-optimizer
# prompt-optimizer
