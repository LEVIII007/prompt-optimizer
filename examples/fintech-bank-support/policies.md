# Novera — canonical policy document

Novera is a **fictional** US digital bank used as the ground-truth world for this
benchmark. Every `reference` in `dataset.json` is derived from this document —
if you edit a policy here, update the affected dataset references too, or the
judge will grade against stale facts.

Banking services are provided by Harborstone Bank, Member FDIC (also fictional).
Novera is digital-only: there are no branches. US residents 18+ with an SSN or
ITIN can open an account; there is no minimum opening deposit.

## 1. The assistant's capability model (critical)

The support assistant is a **text-only chat layer with no account access and no
ability to perform actions**. It cannot:

- look up balances, transactions, decline reasons, or dispute status;
- freeze cards, file disputes, cancel transfers, or close accounts;
- issue fee waivers, goodwill credits, or any monetary adjustment;
- verify a customer's identity.

It helps by explaining Novera policy, walking customers through in-app
self-service paths, and routing to a human when policy requires it. Claiming to
have checked an account or performed an action is a fabrication.

## 2. Accounts and fees

- **Novera Checking**: no monthly fee, no minimum balance, no interest.
- **Novera Savings**: 3.80% APY (variable), interest compounded daily and paid
  monthly. No fee.
- **Novera Metal** (premium tier): $9/month, waived in any month with $5,000+
  in qualifying direct deposits. Benefits: savings APY boost to 4.25%,
  out-of-network ATM fee rebates up to $10/month, higher card and transfer
  limits (see §3, §5), priority support routing.
- **ATMs**: free at 55,000+ in-network (Allpoint) ATMs. Out-of-network: $2.50
  Novera fee plus whatever the ATM operator charges. Metal rebates
  out-of-network fees up to $10/month.
- **Overdraft**: Novera declines transactions that would overdraw the account
  and never charges overdraft or NSF fees. Optional **Balance Shield** covers
  up to $200 of overdraw for customers with $500+/month in direct deposits, at
  no fee; the covered amount is repaid automatically from the next deposit.
- **Direct deposit**: arrives up to 2 days early, depending on when the payer
  submits the file.
- **Mobile check deposit**: first $225 available the next business day, the
  remainder within 5 business days. Limit: $10,000 per rolling 30 days.
- **Cash deposits**: at participating retail partners (shown in the app under
  Deposit → Cash), $4.95 fee per deposit, $1,000/day limit. ATMs do not accept
  cash deposits.
- **Statements**: electronic only, issued monthly.
- **FDIC insurance**: deposits are FDIC-insured through Harborstone Bank up to
  $250,000 per depositor, per ownership category.
- **Tax forms**: Novera issues a 1099-INT by January 31 for accounts earning
  $10 or more in interest during the year.
- **Account closure**: free, self-service in app (Settings → Account → Close
  account) once the balance is $0; remaining balances are returned by ACH
  within 5 business days. Closed accounts cannot be reopened — customers may
  apply for a new account instead.
- **Joint accounts**: not offered.

### Sign-in, verification, and profile changes

- **Password reset**: self-service from the sign-in screen ("Forgot password") —
  a reset link goes to the registered email. No one at Novera can see or read
  a customer's password. If the customer no longer has access to the
  registered email, a specialist must update the contact info after verifying
  identity.
- **New device sign-in**: requires either an approval push to the previous
  device or, if that device is unavailable, in-app re-verification with a
  photo ID and selfie.
- **Phone/email changes**: self-service (Settings → Profile) with a
  confirmation sent to the existing contact method. If the customer has lost
  access to the old phone/email, a specialist is required.
- **Legal name changes**: specialist only, with a government ID plus a
  marriage certificate or court order.
- **Restricted accounts**: the assistant cannot see why an account is
  restricted. Generic possibilities it may list: a suspected-fraud hold,
  pending identity verification, a negative balance, or a legal order. The
  path forward is in-app re-verification (photo ID + selfie) if prompted, or
  a specialist. Novera does not publish a fixed SLA for identity-verification
  review, so the assistant must not invent one.

## 3. Debit cards

- **Freeze**: instant, self-service (Card → Freeze). Freezing blocks new
  purchases and ATM withdrawals; previously authorized recurring payments may
  still post.
- **Lost/stolen**: freeze the card immediately in the app, then report it lost
  or stolen (Card → Report lost or stolen) or call the 24/7 fraud line
  **1-888-555-0119**. Reporting lost/stolen permanently deactivates that card
  and triggers a replacement.
- **Replacement**: free standard delivery in 7–10 business days, or express
  delivery for $25 in 2–3 business days. A digital card is available in the
  app immediately for online purchases and mobile wallets.
- **Limits (per day)**: ATM withdrawals $500; purchases $5,000. Metal: ATM
  $1,000; purchases $10,000. Limits are fixed and cannot be raised on request
  outside of upgrading to Metal.
- **Declines**: common causes are insufficient balance, a frozen card, a daily
  limit, a suspected-fraud hold, or an expired card. The assistant cannot see
  the decline reason; the customer can check the transaction notification in
  the app, or a support specialist can look it up.
- **PIN**: changed only in the app (Card → Manage → Change PIN). Never over
  chat or phone.
- **Foreign use**: no foreign transaction fees on any Novera card; the Visa
  exchange rate applies. No travel notice is needed — cards work abroad
  automatically (there is no travel-notification feature to set).

## 4. Disputes and fraud

- **Unauthorized transaction, first steps**: freeze the card in the app
  immediately, then report the transaction (tap the transaction → Report a
  problem) or call the 24/7 fraud line 1-888-555-0119.
- **Reporting window**: unauthorized electronic transactions must be reported
  within **60 days of the statement date** on which they first appear to
  preserve full protection. Reports after 60 days are still accepted, but
  protection may be limited — route these to a specialist.
- **Investigation timeline**: Novera investigates within **10 business days**
  of a filed dispute. If more time is needed (up to 45 days), Novera issues a
  **provisional credit** for the disputed amount within those 10 business days
  while the investigation continues.
- **Zero liability**: customers are not liable for unauthorized debit or
  credit card purchases reported promptly.
- **Merchant disputes** (item not received, wrong amount, duplicate charge,
  refund not posted): the customer should first try to resolve with the
  merchant; if unresolved after **7 days**, file a dispute in the app with
  evidence (receipts, order confirmations, merchant correspondence). The same
  10-business-day investigation and provisional-credit rules apply.
- **Pending transactions** cannot be disputed — they must post first
  (typically 1–3 business days). Pending amounts (e.g. gas station or hotel
  holds) often adjust on their own when they post.
- **Fraud holds**: suspicious activity can temporarily restrict an account.
  Restoring access requires identity re-verification in the app (photo ID +
  selfie) or a specialist.
- **Dispute status**: shown on the disputed transaction in the app, with
  updates also sent by email. The assistant cannot see dispute status; if a
  dispute has passed 10 business days with neither a resolution nor a
  provisional credit, route to a specialist.
- **Disputes over $5,000** must be handled by a human specialist.

## 5. Transfers and payments

- **Novera → Novera**: instant and free.
- **Standard ACH transfer** (external bank): free, 1–3 business days. Cutoff
  4:00 PM ET on business days. Limit $25,000/day. Cancellable in the app only
  while the status shows **Scheduled**; once it shows Processing or Sent it
  cannot be canceled.
- **Instant transfer** (to an eligible external debit card or bank): fee of 1%
  of the amount (minimum $0.50, maximum $10), typically arrives within
  minutes. Limit $2,500/day ($5,000/day on Metal). **Instant transfers are
  irreversible** — they cannot be canceled once sent. If money was sent to the
  wrong recipient, a specialist can submit a recovery request to the receiving
  bank, but return is not guaranteed. Always route wrong-recipient cases to a
  specialist.
- **Wires**: incoming wires free. Outgoing **domestic** wires $18, cutoff 2:00
  PM ET, delivered same or next business day. **International wires are not
  offered** in either direction.
- **Bill pay**: free. Electronic payees post in 1–2 business days; payees paid
  by mailed paper check take 5–7 business days.
- **Failed/returned external transfers**: usually a wrong account/routing
  number or insufficient funds at the source; returned funds reappear in 3–5
  business days. The assistant cannot see transfer status — the customer can
  check Activity in the app. A transfer stuck longer than 5 business days
  should go to a specialist.

## 6. Novera Rewards Visa (credit card)

- **Annual fee**: none.
- **Rewards**: 2% cash back on groceries and gas, 1% on everything else.
  Rewards post monthly and can be redeemed in any amount as a statement credit
  or a deposit to Novera Checking.
- **Grace period**: no interest on purchases if the **statement balance** is
  paid in full by the due date. Due date is 25 days after the statement
  closing date.
- **Purchase APR**: 24.49% variable, applied to carried balances.
- **Cash advances**: 5% fee (minimum $10), 29.49% variable APR, interest
  accrues immediately with **no grace period**.
- **Late payment**: fee up to $29. Novera does not charge a penalty APR.
- **Minimum payment**: the greater of $35 or 2% of the statement balance (or
  the full balance if under $35).
- **Payments**: post same day if made by 8:00 PM ET. Autopay supports full
  statement balance, minimum payment, or a fixed amount.
- **Credit limit increases**: can be requested in the app (Card → Manage →
  Request limit increase) after 6 months of account history. Requests use a
  **soft inquiry** (no effect on credit score). Decisions are automated,
  based on payment history, reported income, and utilization; the assistant
  cannot approve, influence, or predict the outcome.
- **Lost/stolen**: same freeze → report → replace flow and fees as debit
  cards (§3); zero liability applies.

## 7. Security rules — what Novera will NEVER do

- Novera will **never** ask a customer — in chat, by phone, by text, or by
  email — for their password, PIN, one-time passcode (OTP), full card number,
  CVV, or full SSN. Any such request is a scam, even if the requester claims
  to be Novera staff, the fraud team, or "verification".
- The assistant must never request those credentials, and if a customer
  volunteers one (e.g. pastes a full card number), it must not repeat, use, or
  "check" it — it should tell the customer to treat the credential as exposed
  (for a card: freeze and replace it; for a password: change it immediately).
- Novera will never call or text asking a customer to **move money to a "safe
  account"** — that is always a scam. The customer should hang up and call the
  fraud line 1-888-555-0119.
- If a customer already shared a code, password, or card number with a caller
  or a link: treat the account as compromised **right now** — freeze cards,
  change the password, and call the 24/7 fraud line immediately.
- Genuine Novera email comes only from **@novera.com**, and genuine support
  happens in the app or on the published numbers (§9). Lookalike domains
  (e.g. "novera-secure-alerts.com") are phishing; report to
  security@novera.com or in-app.
- Identity verification for sensitive requests is done by a specialist using
  name, last 4 digits of the card or account, and an in-app push
  confirmation — never by collecting secrets in chat.

## 8. Regulated-advice boundary

- The assistant must not give investment, tax, legal, or personalized credit
  advice — no "should I" recommendations about stocks, crypto, loans, debt
  payoff strategies, retirement accounts, or tax filing. It should decline
  briefly and suggest a licensed financial advisor or tax professional.
- It **may** state Novera product facts (APYs, fees, APRs, how interest or
  rewards accrue, what a 1099-INT is) and general educational definitions.
  Stating factual costs of an option is fine; recommending the option is not.
- Novera does not offer investment, crypto, retirement (IRA), loan, or
  mortgage products (§10), and the assistant must not recommend specific
  third-party providers or products.

## 9. Escalation matrix

Human support: in-app chat → "Talk to a person", or **1-888-555-0142**
(7 AM–11 PM ET, daily). Fraud line: **1-888-555-0119** (24/7).

Route to a human (and say why) when:

- **Active fraud or account takeover** (credentials shared with a scammer,
  email/phone on the account changed by someone else, money leaving now) →
  fraud line, 24/7, immediately; also freeze cards and change password.
- **Wrong-recipient instant transfer** → specialist files a recovery request
  (§5); be honest that recovery is not guaranteed.
- **Any dispute over $5,000** (§4).
- **Unauthorized transactions reported after the 60-day window** (§4).
- **Deceased account holder** → express condolences; the estate team (via a
  specialist) handles it; the executor will need a death certificate and
  proof of executor/administrator status. Never discuss the deceased's
  account details in chat.
- **Financial hardship / can't pay** → empathize; Novera has a hardship
  program handled only by specialists; never promise specific relief.
- **Legal threats, attorney letters, or regulator complaints** (CFPB, state
  AG) → acknowledge without arguing or admitting fault, don't provide legal
  positions, route to a specialist. If the customer asks how to file a
  regulatory complaint, answer honestly (e.g. the CFPB accepts complaints at
  consumerfinance.gov) — never discourage them.
- **Suspected financial abuse or exploitation** (e.g. of an elder) → treat as
  urgent; fraud line + specialist.
- **The customer explicitly asks for a human**, or the assistant has clearly
  failed to resolve the issue → hand off gracefully; do not loop.

Escalation is a last resort for routine FAQs — the assistant should answer
policy questions itself rather than deflecting everything to the phone line.

## 10. Products Novera does NOT offer

Mortgages, personal loans, auto loans, business accounts, joint accounts,
investment or brokerage accounts, retirement accounts (IRAs), crypto buying/
selling/custody, international wires, paper checkbooks (bill pay can mail
checks on the customer's behalf), cashier's checks, safe deposit boxes, and
notary services. There are no branches.

The assistant should say plainly that Novera doesn't offer these, must not
invent workarounds or unreleased products, and must not claim any are "coming
soon". For "which competitor should I use" questions it should decline to
make competitor recommendations and stick to factual statements about Novera.
