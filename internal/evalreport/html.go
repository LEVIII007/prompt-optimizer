package evalreport

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/Conversly/prompt-opt/internal/optimizer"
)

// RenderHTML produces a single self-contained HTML dashboard covering the
// same ground as RenderMarkdown - headline numbers, per-category breakdown,
// every round's candidate prompt with its reflection analysis and worst
// examples, and the best candidate's best/worst val examples - as a page
// that opens directly from disk with no external assets or build step.
func RenderHTML(result *optimizer.Result, cmp *Comparison) string {
	var b strings.Builder
	b.WriteString(htmlHead)
	b.WriteString("<body>\n<div class=\"wrap\">\n<h1>Prompt Optimization Report</h1>\n")

	writeSummaryCards(&b, result, cmp)

	if cmp.TrainValGapWarning {
		fmt.Fprintf(&b, "<div class=\"warning\">&#9888; <strong>Possible overfitting</strong> — train score %.3f is notably higher than val score %.3f.</div>\n",
			cmp.BestTrainScore, cmp.BestValScore)
	}

	if hasRealCategories(cmp.SeedByCategory) {
		writeCategoryTable(&b, cmp)
	}

	writePoolTable(&b, result.Pool, result.BestTrainScore)

	b.WriteString("<h2>Prompts</h2>\n")
	writePromptBox(&b, "Seed prompt", result.SeedPrompt)
	writePromptBox(&b, "Best prompt (final)", result.BestPrompt)

	writeTimeline(&b, result.History)

	worst, best := bestWorstExamples(cmp.PerExample, 5)
	b.WriteString("<h2>Best candidate's val examples</h2>\n")
	writeExampleSection(&b, "Worst", worst)
	writeExampleSection(&b, "Best", best)

	writeAllExamplesTable(&b, cmp.PerExample)

	b.WriteString("</div>\n")
	b.WriteString(htmlScript)
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

func writeSummaryCards(b *strings.Builder, result *optimizer.Result, cmp *Comparison) {
	deltaClass, sign := "good", "+"
	if cmp.Delta < 0 {
		deltaClass, sign = "bad", ""
	}
	b.WriteString("<div class=\"cards\">\n")
	writeCard(b, "Rounds run", fmt.Sprintf("%d", len(result.History)), "")
	writeCard(b, "Seed val score", fmt.Sprintf("%.3f", cmp.SeedAggregate), "")
	writeCard(b, "Best val score", fmt.Sprintf("%.3f", cmp.BestAggregate), "")
	writeCard(b, "Delta", fmt.Sprintf("%s%.3f", sign, cmp.Delta), deltaClass)
	writeCard(b, "Best train score", fmt.Sprintf("%.3f", cmp.BestTrainScore), "")
	b.WriteString("</div>\n")
}

func writeCard(b *strings.Builder, label, value, valueClass string) {
	cls := "value"
	if valueClass != "" {
		cls += " " + valueClass
	}
	fmt.Fprintf(b, "<div class=\"card\"><div class=\"label\">%s</div><div class=\"%s\">%s</div></div>\n",
		html.EscapeString(label), cls, html.EscapeString(value))
}

func writeCategoryTable(b *strings.Builder, cmp *Comparison) {
	b.WriteString("<h2>Per-category val scores</h2>\n<table><thead><tr><th>Category</th><th>Seed</th><th>Best</th><th>Δ</th><th>Comparison</th></tr></thead><tbody>\n")
	for _, cat := range sortedKeys(cmp.SeedByCategory) {
		label := cat
		if label == "" {
			label = "(uncategorized)"
		}
		seedScore, bestScore := cmp.SeedByCategory[cat], cmp.BestByCategory[cat]
		delta := bestScore - seedScore
		deltaClass, sign := "good", "+"
		if delta < 0 {
			deltaClass, sign = "bad", ""
		}
		fmt.Fprintf(b, "<tr><td>%s</td><td>%.3f</td><td>%.3f</td><td class=\"%s\">%s%.3f</td><td class=\"bars\">%s</td></tr>\n",
			html.EscapeString(label), seedScore, bestScore, deltaClass, sign, delta, barsHTML(seedScore, bestScore))
	}
	b.WriteString("</tbody></table>\n")
}

// barsHTML renders two stacked mini bars (seed, then best) whose widths are
// proportional to the two 0..1 scores, so a category's shift is visible at a
// glance without pulling in a charting library.
func barsHTML(seed, best float64) string {
	bestClass := "good"
	if best < seed {
		bestClass = "bad"
	}
	return fmt.Sprintf(
		"<div class=\"bar-track\"><div class=\"bar-fill seed\" style=\"width:%.1f%%\"></div></div>"+
			"<div class=\"bar-track\"><div class=\"bar-fill best %s\" style=\"width:%.1f%%\"></div></div>",
		pct(seed), bestClass, pct(best))
}

func pct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 100
	}
	return v * 100
}

// writePoolTable lists every candidate the search ever admitted to the pool
// - not just the winning lineage. bestTrainScore is Result.BestTrainScore, a
// direct copy of the winning candidate's Mean, so an exact float comparison
// reliably marks the winner without recomputing the argmax here too.
func writePoolTable(b *strings.Builder, pool []optimizer.Candidate, bestTrainScore float64) {
	b.WriteString("<h2>Candidate pool</h2>\n")
	if len(pool) == 0 {
		b.WriteString("<p class=\"muted\">No candidates in the pool.</p>\n")
		return
	}
	b.WriteString("<table><thead><tr><th>ID</th><th>Parent</th><th>Round</th><th>Mean train score</th><th></th></tr></thead><tbody>\n")
	for _, c := range pool {
		parent := "-"
		if c.ParentID >= 0 {
			parent = fmt.Sprintf("#%d", c.ParentID)
		}
		marker := ""
		if c.Mean == bestTrainScore {
			marker = `<span class="pill accepted">winner</span>`
		}
		fmt.Fprintf(b, "<tr><td>#%d</td><td>%s</td><td>%d</td><td>%.3f</td><td>%s</td></tr>\n",
			c.ID, html.EscapeString(parent), c.Round, c.Mean, marker)
	}
	b.WriteString("</tbody></table>\n")
}

func writePromptBox(b *strings.Builder, title, prompt string) {
	fmt.Fprintf(b, "<details class=\"prompt-box\"><summary>%s (%d chars)</summary><pre>%s</pre></details>\n",
		html.EscapeString(title), len([]rune(prompt)), html.EscapeString(prompt))
}

func writeTimeline(b *strings.Builder, history []optimizer.IterationRecord) {
	b.WriteString("<h2>Round-by-round analysis</h2>\n")
	if len(history) == 0 {
		b.WriteString("<p class=\"muted\">No rounds were run.</p>\n")
		return
	}
	for _, rec := range history {
		status := "rejected"
		if rec.Accepted {
			status = "accepted"
		}
		b.WriteString("<div class=\"round\">\n")
		fmt.Fprintf(b, "<div class=\"round-head\"><strong>Round %d</strong><span class=\"pill %s\">%s</span><span class=\"score\">%.3f → %.3f</span></div>\n",
			rec.Round, status, status, rec.PriorScore, rec.CandidateScore)
		if rec.Accepted {
			fmt.Fprintf(b, "<div class=\"muted\">Parent: candidate #%d → admitted as #%d</div>\n", rec.ParentID, rec.AcceptedID)
		} else {
			fmt.Fprintf(b, "<div class=\"muted\">Parent: candidate #%d</div>\n", rec.ParentID)
		}
		if rec.Analysis != "" {
			fmt.Fprintf(b, "<div class=\"analysis\">%s</div>\n", html.EscapeString(rec.Analysis))
		}
		fmt.Fprintf(b, "<details><summary>Candidate prompt tried this round</summary><pre>%s</pre></details>\n",
			html.EscapeString(rec.CandidatePrompt))
		writeWorstExamplesHTML(b, rec.WorstExamples)
		b.WriteString("</div>\n")
	}
}

// writeWorstExamplesHTML renders the judge's per-example scores and feedback
// for the examples reflection was shown - the evidence behind each round's
// Analysis line, not just the aggregate score movement.
func writeWorstExamplesHTML(b *strings.Builder, worst []optimizer.JudgedExample) {
	if len(worst) == 0 {
		return
	}
	fmt.Fprintf(b, "<details><summary>Worst examples shown to reflection (%d)</summary>\n", len(worst))
	for _, je := range worst {
		cat := je.Example.Category
		if cat == "" {
			cat = "-"
		}
		b.WriteString("<div class=\"example\">\n")
		fmt.Fprintf(b, "<div class=\"meta\"><strong>%s</strong> (%s) — overall <strong>%.2f</strong>",
			html.EscapeString(je.Example.ID), html.EscapeString(cat), je.Verdict.Overall)
		if je.Verdict.HallucinationFlag {
			b.WriteString(" &middot; <span class=\"flag\">hallucination flagged</span>")
		}
		b.WriteString("</div>\n")
		if je.Output != "" {
			fmt.Fprintf(b, "<div><span class=\"muted\">Response:</span> %s</div>\n", html.EscapeString(je.Output))
		}
		if je.Verdict.Feedback != "" {
			fmt.Fprintf(b, "<div><span class=\"muted\">Judge feedback:</span> %s</div>\n", html.EscapeString(je.Verdict.Feedback))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</details>\n")
}

func writeExampleSection(b *strings.Builder, title string, scores []ExampleScore) {
	fmt.Fprintf(b, "<h3>%s</h3>\n", html.EscapeString(title))
	if len(scores) == 0 {
		b.WriteString("<p class=\"muted\">none</p>\n")
		return
	}
	b.WriteString("<table><thead><tr><th>ID</th><th>Category</th><th>Seed</th><th>Best</th></tr></thead><tbody>\n")
	for _, s := range scores {
		cat := s.Category
		if cat == "" {
			cat = "-"
		}
		fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td><td>%.3f</td><td>%.3f</td></tr>\n",
			html.EscapeString(s.ID), html.EscapeString(cat), s.SeedScore, s.BestScore)
	}
	b.WriteString("</tbody></table>\n")
}

// writeAllExamplesTable renders every val example with a client-side text
// filter (plain JS, no dependency) since the full set can run into the
// dozens and a user hunting for one category or ID shouldn't have to scroll.
func writeAllExamplesTable(b *strings.Builder, scores []ExampleScore) {
	b.WriteString("<h2>All validation examples</h2>\n")
	b.WriteString("<input id=\"filter\" type=\"text\" placeholder=\"Filter by id or category…\">\n")

	sorted := append([]ExampleScore(nil), scores...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Category != sorted[j].Category {
			return sorted[i].Category < sorted[j].Category
		}
		return sorted[i].ID < sorted[j].ID
	})

	b.WriteString("<table id=\"all-examples\"><thead><tr><th>ID</th><th>Category</th><th>Seed</th><th>Best</th><th>Δ</th><th>Seed</th><th>Best</th></tr></thead><tbody>\n")
	for _, s := range sorted {
		cat := s.Category
		if cat == "" {
			cat = "-"
		}
		delta := s.BestScore - s.SeedScore
		deltaClass, sign := "good", "+"
		if delta < 0 {
			deltaClass, sign = "bad", ""
		}
		fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td><td>%.3f</td><td>%.3f</td><td class=\"%s\">%s%.3f</td><td>%s</td><td>%s</td></tr>\n",
			html.EscapeString(s.ID), html.EscapeString(cat), s.SeedScore, s.BestScore,
			deltaClass, sign, delta, passPill(s.SeedPass), passPill(s.BestPass))
	}
	b.WriteString("</tbody></table>\n")
}

func passPill(pass bool) string {
	if pass {
		return "<span class=\"pill pass\">pass</span>"
	}
	return "<span class=\"pill fail\">fail</span>"
}

const htmlHead = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Prompt Optimization Report</title>
<style>
:root{
  --bg:#f6f7f9; --card:#ffffff; --border:#e3e6ea; --text:#1c2128; --muted:#5b6472;
  --accent:#3457d5; --good:#1a7f37; --bad:#cf222e; --warn:#9a6700;
  --good-bg:#e9f7ee; --bad-bg:#fdedee; --warn-bg:#fff8e6;
}
* { box-sizing: border-box; }
body { margin:0; background:var(--bg); color:var(--text); font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif; line-height:1.5; }
.wrap { max-width:1080px; margin:0 auto; padding:32px 20px 80px; }
h1 { font-size:26px; margin:0 0 20px; }
h2 { font-size:19px; margin:40px 0 14px; padding-bottom:8px; border-bottom:1px solid var(--border); }
h3 { font-size:15px; margin:18px 0 8px; }
.cards { display:flex; flex-wrap:wrap; gap:12px; margin-bottom:20px; }
.card { background:var(--card); border:1px solid var(--border); border-radius:10px; padding:14px 18px; min-width:150px; flex:1; }
.card .label { font-size:12px; color:var(--muted); text-transform:uppercase; letter-spacing:.04em; }
.card .value { font-size:24px; font-weight:600; margin-top:4px; }
.value.good { color:var(--good); }
.value.bad { color:var(--bad); }
.warning { background:var(--warn-bg); color:var(--warn); border:1px solid #f2dca0; border-radius:8px; padding:12px 16px; margin-bottom:20px; font-size:14px; }
table { width:100%; border-collapse:collapse; background:var(--card); border:1px solid var(--border); border-radius:10px; overflow:hidden; font-size:14px; margin-bottom:12px; }
th, td { text-align:left; padding:9px 12px; border-bottom:1px solid var(--border); vertical-align:middle; }
th { background:#fafbfc; font-size:11px; text-transform:uppercase; color:var(--muted); letter-spacing:.03em; }
tr:last-child td { border-bottom:none; }
td.bars { min-width:200px; }
.bar-track { position:relative; background:#eef0f3; border-radius:4px; height:7px; margin-top:4px; }
.bar-track:first-child { margin-top:0; }
.bar-fill { position:absolute; left:0; top:0; height:100%; border-radius:4px; }
.bar-fill.seed { background:#9aa4b2; }
.bar-fill.best.good { background:var(--good); }
.bar-fill.best.bad { background:var(--bad); }
.pill { display:inline-block; font-size:12px; font-weight:600; padding:2px 9px; border-radius:20px; }
.pill.accepted, .pill.pass { background:var(--good-bg); color:var(--good); }
.pill.rejected, .pill.fail { background:var(--bad-bg); color:var(--bad); }
.round { background:var(--card); border:1px solid var(--border); border-radius:10px; padding:16px 18px; margin-bottom:14px; }
.round-head { display:flex; align-items:center; gap:10px; flex-wrap:wrap; margin-bottom:8px; }
.round-head .score { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; color:var(--muted); font-size:13px; }
.analysis { font-size:14px; margin:6px 0 10px; }
details { margin-top:8px; }
summary { cursor:pointer; color:var(--accent); font-size:13px; font-weight:500; }
summary:hover { text-decoration:underline; }
pre { white-space:pre-wrap; word-break:break-word; background:#fafbfc; border:1px solid var(--border); border-radius:8px; padding:10px 12px; font-size:13px; margin:8px 0 0; }
.prompt-box pre { max-height:340px; overflow:auto; }
.example { border-top:1px dashed var(--border); padding:10px 0; font-size:14px; }
.example:first-child { border-top:none; }
.example .meta { font-size:12px; color:var(--muted); margin-bottom:4px; }
.flag { color:var(--bad); font-weight:600; }
input#filter { width:100%; max-width:320px; padding:7px 10px; border:1px solid var(--border); border-radius:8px; font-size:13px; margin-bottom:10px; }
.muted { color:var(--muted); }
</style>
</head>
`

const htmlScript = `<script>
(function () {
  var input = document.getElementById('filter');
  if (!input) return;
  var rows = document.querySelectorAll('#all-examples tbody tr');
  input.addEventListener('input', function () {
    var q = input.value.toLowerCase();
    rows.forEach(function (row) {
      row.style.display = row.textContent.toLowerCase().indexOf(q) === -1 ? 'none' : '';
    });
  });
})();
</script>
`
