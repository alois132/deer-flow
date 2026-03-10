package llm

import (
	"context"
	"errors"
	"github.com/alois132/deer-flow/pkg/llm/openai"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

var (
	ErrCfg = errors.New("invalid config")
)

func InitLLM(ctx context.Context, cfg *Config) (llm model.ToolCallingChatModel, err error) {
	if cfg == nil {
		return nil, ErrCfg
	}
	switch cfg.Provider {
	case ProviderArk:
		llm, err = ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey: cfg.APIKey,
			Model:  cfg.Model,
		})
	case ProviderOpenAI:
		llm, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
			BaseURL: cfg.BaseURL,
			//ReasoningEffort: openai.ReasoningEffortLevelHigh,
		})
	}

	return llm, err
}

func FString(ctx context.Context, str string, vs map[string]any) (string, error) {
	temp := prompt.FromMessages(schema.FString, schema.SystemMessage(str))
	result, err := temp.Format(ctx, vs)
	if err != nil {
		return "", err
	}
	return result[0].Content, err
}
