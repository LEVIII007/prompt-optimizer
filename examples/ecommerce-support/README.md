# Meridian e-commerce support bot — optimization benchmark

A realistic, self-contained scenario for exercising the prompt-opt pipeline end to
end and judging whether it actually improves a prompt. Unlike the tiny fixtures in
`testdata/` (which exist only for unit tests), this is meant to be **run against a
real model** and re-run every time we change the optimizer.

## The scenario

"Meridian" is a fictional online retailer. The dataset is **36 customer messages**
across 11 policy areas, each with a `reference` field holding the ground-truth
policy the answer should match:

| category | # | probes |
|---|---|---|
| returns | 4 | 30-day window, 15-day electronics window, opened-but-unused, final-sale |
| refunds | 3 | 5–7 day timing, shipping-fee refundability, gift-card → store credit |
| shipping | 4 | prices/speeds, 2 PM cutoff, US/Canada only, PO-box limits |
| order_changes | 3 | 60-min change window, past-window, can't add items |
| damaged_defective | 3 | 48h + photos, past-window high-value, free defective returns |
| warranty | 3 | 1-yr manufacturer warranty, no in-house repair, non-electronics |
| price_match | 3 | 14-day match, marketplace exclusion, own-price-drop adjustment |
| membership | 3 | Meridian+ price/benefits, no prorated refund, 60-day returns |
| promo_codes | 3 | one per order, no retroactive apply, member-vs-promo stacking |
| payment | 3 | accepted methods, charge-at-shipment, Affirm BNPL |
| escalation | 4 | >$500 dispute, safety hazard, fabricated-discount trap, legal threat |

Every area has 3–4 examples on purpose: with `--val-split 0.3` and the splitter's
category stratification, each area lands **~2–3 in train and ~1 in val**, so a policy
the optimizer learns from a train example can actually show up as a val-set gain.

## Why the seed prompt is deliberately weak

`seed_prompt.txt` is a plausible first draft with **specific, fixable holes**. It is
weak on purpose — an optimizer that works should discover and patch these:

1. **No policy knowledge.** It contains zero Meridian facts, so the bot must guess
   return windows, fees, and timelines → low `policy_accuracy`.
2. **Actively encourages hallucination.** "try to resolve every question yourself so
   the customer always gets a … complete answer without needing to reach out to
   anyone else" pushes the bot to invent policy and to *avoid escalating* → low
   `grounding` and `escalation`.
3. **No escalation rules.** Nothing about safety, >$500 disputes, legal threats, or
   out-of-policy questions → it mishandles the whole `escalation` category.
4. **No tone/empathy guidance** → flat replies to shattered mugs and furious
   customers → low `tone`.

## What a working optimizer should produce

The judge's feedback carries the correct policy (from `reference`) into the
reflection step, so a healthy run should evolve the prompt to:

- bake in the concrete Meridian policies (windows, fees, timelines, exclusions);
- add an explicit "never invent policy — if unsure, say so and offer a human" rule;
- add escalation triggers (safety, >$500, legal, unknown → human specialist);
- add a warmth/empathy instruction for complaints and damage.

**This is the actual test of the pipeline:** open the generated `report.md` and
check that `policy_accuracy`, `grounding`, and `escalation` rose on the **frozen val
set**, and that the round-by-round `analysis` timeline shows those diagnoses — not
just that the aggregate number went up.

## The rubric

`rubric.json` scores 5 dimensions (0–5), weighted, pass threshold 0.7:

- **policy_accuracy** (weight 4, required) — matches reference policy facts
- **grounding** (weight 3, required) — no fabrication; admits the unknown
- **escalation** (weight 3) — correct human-vs-self routing
- **tone** (weight 2) — warmth/empathy
- **completeness_clarity** (weight 2) — answers fully, actionably

`policy_accuracy` and `grounding` are `required`: a 0 on either fails the response
regardless of the average, so a confident-but-wrong answer can't pass on charm.

## Running it

Set Azure credentials first (`cp .env.example .env` at the repo root and fill in
`AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_API_VERSION`, and a
chat deployment name). Then, from the repo root:

**Cheap smoke run** (validate wiring end to end, ~a couple minutes):

```
go run ./cmd/optimize \
  --seed-prompt examples/ecommerce-support/seed_prompt.txt \
  --dataset     examples/ecommerce-support/dataset.json \
  --rubric      examples/ecommerce-support/rubric.json \
  --task-deployment gpt-4o-mini \
  --iterations 2 --minibatch-size 4 --full-eval-every 2
```

**Full run** (the real benchmark):

```
go run ./cmd/optimize \
  --seed-prompt examples/ecommerce-support/seed_prompt.txt \
  --dataset     examples/ecommerce-support/dataset.json \
  --rubric      examples/ecommerce-support/rubric.json \
  --task-deployment gpt-4o \
  --iterations 10 --minibatch-size 8 --concurrency 6
```

Artifacts land in `tmp/prompt-opt-<timestamp>/`: `best_prompt.txt`,
`report.md`, `run_history.json`, `comparison_report.json`.

## Reading the result

A healthy run shows:

- **positive val-set delta** (`best_val_score` > `seed_val_score`);
- **`policy_accuracy` / `grounding` / `escalation` up** in the per-category table,
  not just the headline number;
- **no train/val-gap warning** (or a small gap) — a large gap means it overfit the
  search set rather than learning general policy;
- an `analysis` timeline that names the real failures (missing policy, hallucinated
  answers, no escalation) rather than vague churn.

If the seed already scores high, or the delta is zero/negative, that's signal about
the *pipeline* (judge too lenient, reflection not generalizing, minibatch too small),
which is exactly what this fixture is here to surface.
