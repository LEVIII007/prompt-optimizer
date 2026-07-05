package config

import "os"

// Config holds the environment-derived settings this tool needs: Azure OpenAI
// credentials. Run-specific behavior (iterations, concurrency, split ratio,
// deployments, etc.) is passed via CLI flags instead, since it varies per
// invocation rather than per environment.
type Config struct {
	AzureOpenAIEndpoint       string
	AzureOpenAIAPIKey         string
	AzureOpenAIAPIVersion     string
	AzureOpenAIChatDeployment string
	LogLevel                  string
}

// LoadConfig reads config from environment variables. It does not validate
// required fields itself — llm.NewAzureOpenAIChatModel already rejects empty
// endpoint/key/deployment at construction time.
func LoadConfig() *Config {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return &Config{
		AzureOpenAIEndpoint:       os.Getenv("AZURE_OPENAI_ENDPOINT"),
		AzureOpenAIAPIKey:         os.Getenv("AZURE_OPENAI_API_KEY"),
		AzureOpenAIAPIVersion:     os.Getenv("AZURE_OPENAI_API_VERSION"),
		AzureOpenAIChatDeployment: os.Getenv("AZURE_OPENAI_CHAT_DEPLOYMENT"),
		LogLevel:                  logLevel,
	}
}
