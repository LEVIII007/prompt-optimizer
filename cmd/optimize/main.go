package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/Conversly/prompt-opt/internal/config"
	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/evalreport"
	"github.com/Conversly/prompt-opt/internal/judge"
	"github.com/Conversly/prompt-opt/internal/llm"
	"github.com/Conversly/prompt-opt/internal/optimizer"
	"github.com/Conversly/prompt-opt/internal/rubric"
	"github.com/Conversly/prompt-opt/internal/utils"
)

func main() {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		exitf("failed to locate module root: %v", err)
	}
	loadEnv(moduleRoot)

	cfg := config.LoadConfig()
	cleanup := utils.InitLogger(cfg.LogLevel)
	defer cleanup()

	defaultOut := filepath.Join(moduleRoot, "tmp", "prompt-opt-"+time.Now().Format("20060102-150405"))

	seedPromptFlag := flag.String("seed-prompt", "", "path to the starting system prompt (text file)")
	datasetFlag := flag.String("dataset", "", "path to dataset JSON")
	rubricFlag := flag.String("rubric", "", "path to rubric JSON")
	outFlag := flag.String("out", defaultOut, "directory to write output artifacts")
	taskDeploymentFlag := flag.String("task-deployment", "", "azure deployment used to run the candidate prompt")
	judgeDeploymentFlag := flag.String("judge-deployment", "", "azure deployment used to score outputs (defaults to --task-deployment)")
	reflectionDeploymentFlag := flag.String("reflection-deployment", "", "azure deployment used to propose prompt rewrites (defaults to --task-deployment)")
	iterationsFlag := flag.Int("iterations", 10, "max optimizer rounds")
	minibatchSizeFlag := flag.Int("minibatch-size", 8, "examples sampled from the train set per round")
	valSplitFlag := flag.Float64("val-split", 0.3, "fraction of the dataset held out, frozen, for the final comparison")
	patienceFlag := flag.Int("patience", 4, "stop early after this many consecutive rejected rounds; 0 disables early stopping")
	concurrencyFlag := flag.Int("concurrency", 4, "concurrent LLM calls when scoring a batch")
	retriesFlag := flag.Int("retries", 1, "retries per LLM call on error or invalid JSON")
	seedFlag := flag.Int64("seed", 42, "RNG seed for the train/val split and minibatch sampling")
	taskMaxTokensFlag := flag.Int("task-max-tokens", 4000, "max_completion_tokens ceiling for the task model; a safety cap on reasoning-model output (reasoning tokens count against it)")
	judgeMaxTokensFlag := flag.Int("judge-max-tokens", 3000, "max_completion_tokens ceiling for the judge model")
	reflectionMaxTokensFlag := flag.Int("reflection-max-tokens", 16000, "max_completion_tokens ceiling for the reflection model; needs the most room since it rewrites the whole (growing) prompt")
	flag.Parse()

	if strings.TrimSpace(*seedPromptFlag) == "" || strings.TrimSpace(*datasetFlag) == "" || strings.TrimSpace(*rubricFlag) == "" {
		exitf("--seed-prompt, --dataset, and --rubric are required")
	}
	if strings.TrimSpace(*taskDeploymentFlag) == "" {
		exitf("--task-deployment is required")
	}
	if *iterationsFlag < 1 {
		exitf("--iterations must be at least 1")
	}
	if *minibatchSizeFlag < 1 {
		exitf("--minibatch-size must be at least 1")
	}
	if *valSplitFlag <= 0 || *valSplitFlag >= 1 {
		exitf("--val-split must be between 0 and 1 (exclusive)")
	}
	if *patienceFlag < 0 {
		exitf("--patience must be 0 or greater")
	}
	if *concurrencyFlag < 1 {
		exitf("--concurrency must be at least 1")
	}
	if *retriesFlag < 0 {
		exitf("--retries must be 0 or greater")
	}
	if *taskMaxTokensFlag < 256 || *judgeMaxTokensFlag < 256 || *reflectionMaxTokensFlag < 256 {
		exitf("--task-max-tokens, --judge-max-tokens, and --reflection-max-tokens must each be at least 256 (reasoning models need headroom above their reasoning-token usage)")
	}

	judgeDeployment := strings.TrimSpace(*judgeDeploymentFlag)
	if judgeDeployment == "" {
		judgeDeployment = *taskDeploymentFlag
	}
	reflectionDeployment := strings.TrimSpace(*reflectionDeploymentFlag)
	if reflectionDeployment == "" {
		reflectionDeployment = *taskDeploymentFlag
	}

	seedPromptBytes, err := os.ReadFile(*seedPromptFlag)
	if err != nil {
		exitf("failed to read seed prompt: %v", err)
	}
	seedPrompt := strings.TrimSpace(string(seedPromptBytes))
	if seedPrompt == "" {
		exitf("seed prompt file is empty")
	}

	examples, err := dataset.Load(*datasetFlag)
	if err != nil {
		exitf("failed to load dataset: %v", err)
	}

	r, err := rubric.Load(*rubricFlag)
	if err != nil {
		exitf("failed to load rubric: %v", err)
	}

	train, val := dataset.Split(examples, *valSplitFlag, *seedFlag)
	if len(train) == 0 {
		exitf("train split is empty - dataset too small for --val-split=%.2f", *valSplitFlag)
	}
	if len(val) == 0 {
		exitf("validation split is empty - dataset too small for --val-split=%.2f", *valSplitFlag)
	}

	taskModel, err := newAzureModel(cfg, *taskDeploymentFlag, *taskMaxTokensFlag)
	if err != nil {
		exitf("failed to create task model: %v", err)
	}
	judgeModel, err := newAzureModel(cfg, judgeDeployment, *judgeMaxTokensFlag)
	if err != nil {
		exitf("failed to create judge model: %v", err)
	}
	reflectionModel, err := newAzureModel(cfg, reflectionDeployment, *reflectionMaxTokensFlag)
	if err != nil {
		exitf("failed to create reflection model: %v", err)
	}

	if err := os.MkdirAll(*outFlag, 0o755); err != nil {
		exitf("failed to create output directory: %v", err)
	}
	writeProgress := func(r *optimizer.Result) {
		if err := os.WriteFile(filepath.Join(*outFlag, "best_prompt.txt"), []byte(r.BestPrompt), 0o644); err != nil {
			utils.Logger().Warn("failed to checkpoint best_prompt.txt", zap.Error(err))
		}
		if err := utils.WriteJSON(filepath.Join(*outFlag, "run_history.json"), r); err != nil {
			utils.Logger().Warn("failed to checkpoint run_history.json", zap.Error(err))
		}
	}

	j := judge.New(judgeModel, r, *retriesFlag)
	settings := optimizer.Settings{
		Iterations:    *iterationsFlag,
		MinibatchSize: *minibatchSizeFlag,
		Patience:      *patienceFlag,
		Concurrency:   *concurrencyFlag,
		Retries:       *retriesFlag,
		Seed:          *seedFlag,
		OnUpdate:      writeProgress,
	}
	deps := optimizer.Deps{TaskModel: taskModel, ReflectionModel: reflectionModel}

	fmt.Printf("Loaded %d examples (%d train / %d val). Task=%s Judge=%s Reflection=%s\n",
		len(examples), len(train), len(val), *taskDeploymentFlag, judgeDeployment, reflectionDeployment)

	ctx := context.Background()
	result, err := optimizer.Run(ctx, deps, j, seedPrompt, train, settings)
	if err != nil {
		exitf("optimization failed: %v", err)
	}

	for _, rec := range result.History {
		fmt.Printf("round %d (parent #%d): %.3f -> %.3f (%s)\n", rec.Round, rec.ParentID, rec.PriorScore, rec.CandidateScore, acceptedLabel(rec.Accepted))
	}
	fmt.Printf("Final pool: %d candidate(s)\n", len(result.Pool))

	cmp, err := evalreport.Evaluate(ctx, taskModel, j, seedPrompt, result.BestPrompt, val, result.BestTrainScore, settings)
	if err != nil {
		exitf("validation comparison failed: %v", err)
	}

	writeProgress(result)
	if err := utils.WriteJSON(filepath.Join(*outFlag, "comparison_report.json"), cmp); err != nil {
		exitf("failed to write comparison_report.json: %v", err)
	}
	reportMD := evalreport.RenderMarkdown(result, cmp)
	if err := os.WriteFile(filepath.Join(*outFlag, "report.md"), []byte(reportMD), 0o644); err != nil {
		exitf("failed to write report.md: %v", err)
	}
	dashboardHTML := evalreport.RenderHTML(result, cmp)
	if err := os.WriteFile(filepath.Join(*outFlag, "dashboard.html"), []byte(dashboardHTML), 0o644); err != nil {
		exitf("failed to write dashboard.html: %v", err)
	}

	utils.Logger().Info("optimization run complete",
		zap.Int("rounds_run", len(result.History)),
		zap.Float64("seed_val_score", cmp.SeedAggregate),
		zap.Float64("best_val_score", cmp.BestAggregate),
		zap.Float64("delta", cmp.Delta),
		zap.Bool("train_val_gap_warning", cmp.TrainValGapWarning),
	)

	fmt.Printf("\nDone. %d round(s) run. Seed val score %.3f -> best val score %.3f (delta %+.3f).\n",
		len(result.History), cmp.SeedAggregate, cmp.BestAggregate, cmp.Delta)
	if cmp.TrainValGapWarning {
		fmt.Printf("WARNING: possible overfitting - train score %.3f vs val score %.3f.\n", cmp.BestTrainScore, cmp.BestValScore)
	}
	fmt.Printf("Artifacts:\n- %s\n- %s\n- %s\n- %s\n- %s\n",
		filepath.Join(*outFlag, "best_prompt.txt"),
		filepath.Join(*outFlag, "run_history.json"),
		filepath.Join(*outFlag, "comparison_report.json"),
		filepath.Join(*outFlag, "report.md"),
		filepath.Join(*outFlag, "dashboard.html"),
	)
}

func acceptedLabel(accepted bool) string {
	if accepted {
		return "accepted"
	}
	return "rejected"
}

func newAzureModel(cfg *config.Config, deployment string, maxTokens int) (*llm.AzureOpenAIChatModel, error) {
	return llm.NewAzureOpenAIChatModel(cfg.AzureOpenAIEndpoint, cfg.AzureOpenAIAPIKey, deployment, cfg.AzureOpenAIAPIVersion, nil, &maxTokens)
}

func loadEnv(moduleRoot string) {
	candidates := []string{
		filepath.Join(moduleRoot, "cmd", ".env"),
		filepath.Join(moduleRoot, ".env"),
	}
	for _, candidate := range candidates {
		_ = godotenv.Overload(candidate)
	}
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
