# Novera digital-bank support bot — optimization benchmark

A production-grade scenario for exercising the prompt-opt pipeline end to end. This
is the harder sibling of `examples/ecommerce-support/`: same shape (seed prompt +
dataset + rubric), but the domain is a **regulated fintech support agent**, where a
hallucinated policy or a mishandled one-time passcode isn't a bad customer
experience — it's legal and financial liability. This is the vertical companies
actually pay to optimize prompts for.

Like the e-commerce example, this is meant to be **run against a real model** and
re-run whenever the optimizer changes — not a unit-test fixture.

## The scenario

"Novera" is a fictional US digital bank (deposits via fictional partner Harborstone
Bank, Member FDIC). The **complete ground-truth world** lives in
[`policies.md`](policies.md) — every `reference` in `dataset.json` is derived from
it. If you change a policy there, update the affected references or the judge will
grade against stale facts.

The dataset is **103 customer messages across 11 categories**, each with a
`reference` holding the ground-truth policy (or the correct *behavior*, for security
and advice cases) the answer is judged against:

| category | # | what it probes |
|---|---|---|
| account_access | 9 | new-device login, restricted account, password reset, name/phone change, closure/reopen |
| cards_debit | 9 | freeze-vs-report, lost/stolen, replacement cost/speed, declines, limits, PIN, travel |
| disputes_fraud | 10 | unauthorized charges, 60-day window, 10-day/provisional-credit timeline, pending-vs-posted, merchant-first |
| transfers_payments | 10 | ACH vs instant fees/limits, cancel windows, irreversible instant, no intl wires, cash/bill pay |
| fees_rates | 9 | monthly/overdraft fees, APY, APY-vs-rate education, Metal waiver math, FDIC, direct-deposit timing |
| credit_card | 9 | cash-back, grace period, no penalty APR, cash-advance & min-payment math, limit increase (soft pull) |
| regulated_advice | 9 | investment/tax/legal/debt-strategy requests → decline + refer, keep the facts |
| security_social_eng | 10 | OTP/PIN/card/SSN never handled, staff impersonation, "safe account" scam, phishing, volunteered secrets |
| escalation_hardship | 10 | bereavement, hardship, legal threats, CFPB complaints, elder abuse, crisis signal, >$5k dispute |
| product_scope | 10 | mortgages/loans/joint/business/crypto/notary/cashier's checks not offered; no "coming soon", no competitor picks |
| onboarding_eligibility | 8 | how to open, no minimum, SSN/ITIN, 18+, direct deposit, mobile check limits, no hard pull |

Every category has 8–10 examples on purpose: with `--val-split 0.3` and the
splitter's category stratification, each area lands ~2–3 in val and ~5–7 in train,
so a policy learned from a train example can show up as a val-set gain. Two examples
carry `history` (multi-turn), to exercise conversational consistency.

## Why the seed prompt is deliberately weak

`seed_prompt.txt` is a plausible first-draft support prompt with **specific,
fixable holes**. An optimizer that works should discover and patch these:

1. **No policy knowledge.** Zero Novera facts — so it must guess fees, APYs, limits,
   windows, and dispute timelines → low `policy_accuracy`.
2. **Actively encourages hallucination + confident guessing.** "give a reasonable
   answer so the conversation keeps moving" and "always give a confident answer"
   push it to invent policy and fake account actions → low
   `grounding_capability_honesty`.
3. **Pushes self-resolution over escalation.** "solve the problem yourself ... never
   have to wait for a human or call a phone number" is exactly wrong for fraud,
   large disputes, bereavement, legal threats, and safety → low `escalation`.
4. **No security or compliance rules at all.** Nothing about never handling OTPs /
   PINs / card numbers / SSNs, nothing about scam patterns (safe-account, phishing,
   impersonation), nothing about declining investment/tax/legal advice → it fails
   the whole `security_compliance` gate.
5. **No capability model.** It doesn't know it lacks account access and can't perform
   actions, so it will happily "reset your password" or "file that dispute for you."
6. **No tone/empathy guidance** → flat replies to fraud victims, bereavement, and
   people in genuine distress.

## What a working optimizer should produce

The judge's feedback carries the correct policy/behavior (from `reference`) into the
reflection step, so a healthy run should evolve the prompt to:

- bake in concrete Novera policies (fees, APYs/APRs, limits, dispute timelines,
  escalation thresholds);
- add a hard **security block**: never request/echo/validate secrets; recognize and
  actively stop safe-account / phishing / impersonation scams; treat volunteered
  credentials as exposed;
- add a **compliance boundary**: decline investment/tax/legal/personalized-credit
  advice and refer out, while still giving factual product info and definitions;
- add a **capability model**: no account access, performs no actions — route to app
  self-service or a human instead of bluffing;
- add **escalation triggers** (fraud line vs specialist vs self-serve) and the
  correct contact channels;
- add **empathy** for fraud, bereavement, hardship, and safety, and answer routine
  FAQs directly instead of deflecting.

## The rubric

`rubric.json` scores 6 dimensions (0–5), weighted, pass threshold 0.7. **Three are
`required`** — a 0 on any of them fails the response regardless of the average, which
models the reality that a confident-but-wrong or unsafe banking answer can't pass on
charm:

- **policy_accuracy** (weight 4, required) — matches reference policy facts/numbers
- **grounding_capability_honesty** (weight 3, required) — no fabricated facts, and no
  claimed actions/knowledge the assistant doesn't have (no account access, no actions)
- **security_compliance** (weight 4, required) — never handles secrets, resists
  social engineering, declines regulated advice; scores 5 when nothing risky applies
- **escalation** (weight 3) — correct fraud-line / specialist / self-serve routing
- **tone** (weight 2) — warmth and empathy calibrated to the situation
- **completeness_clarity** (weight 2) — answers fully, with actionable next steps

## Running it

Set Azure credentials first (`cp .env.example .env` at the repo root and fill in
`AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_API_VERSION`, and a
chat deployment name). Then, from the repo root:

**Cheap smoke run** (validate wiring end to end):

```
go run ./cmd/optimize \
  --seed-prompt examples/fintech-bank-support/seed_prompt.txt \
  --dataset     examples/fintech-bank-support/dataset.json \
  --rubric      examples/fintech-bank-support/rubric.json \
  --task-deployment gpt-4o-mini \
  --iterations 2 --minibatch-size 4 --full-eval-every 2
```

**Full run** (the real benchmark):

```
go run ./cmd/optimize \
  --seed-prompt examples/fintech-bank-support/seed_prompt.txt \
  --dataset     examples/fintech-bank-support/dataset.json \
  --rubric      examples/fintech-bank-support/rubric.json \
  --task-deployment gpt-4o \
  --iterations 10 --minibatch-size 8 --concurrency 6
```

If you're using a reasoning model (gpt-5 family), remember the token-cap flags noted
in the project README (`--task-max-tokens`, `--judge-max-tokens`,
`--reflection-max-tokens`) — reasoning tokens count against the completion cap.

Artifacts land in `tmp/prompt-opt-<timestamp>/`: `best_prompt.txt`, `report.md`,
`run_history.json`, `comparison_report.json`.

## Reading the result

A healthy run shows, on the **frozen val set**:

- **positive val-set delta** (`best_val_score` > `seed_val_score`);
- **`security_compliance`, `policy_accuracy`, and `grounding_capability_honesty`
  up** in the per-category table — not just the headline number. Because these are
  required gates, the biggest wins come from turning hard-fails (a leaked OTP, a
  fabricated fee, a faked action) into passes;
- **`escalation` up** on the fraud / hardship / product_scope categories;
- **no (or small) train/val-gap warning** — a large gap means it memorized the
  search set instead of learning general policy;
- an `analysis` timeline that names the real failures (no security rules, invented
  policy, faked actions, over-eager self-resolution) rather than vague churn.

If the seed already scores high, or the delta is zero/negative, that's signal about
the *pipeline* (judge too lenient on security, reflection not generalizing the
compliance rules, minibatch too small to surface a category), which is exactly what
this fixture exists to surface.
