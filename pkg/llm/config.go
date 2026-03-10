package llm

const (
	ProviderArk    = "ark"
	ProviderOpenAI = "openai"
)

type Provider string

type Config struct {
	Provider Provider `mapstructure:"provider"`
	APIKey   string   `mapstructure:"api_key"`
	Model    string   `mapstructure:"model"`
	BaseURL  string   `mapstructure:"base_url"`
}
