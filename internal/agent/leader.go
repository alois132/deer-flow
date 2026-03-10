package agent

import (
	"context"
	"errors"
	"fmt"
	"github.com/alois132/deer-flow/internal/agent/memory"
	"github.com/alois132/deer-flow/internal/global"
	"github.com/alois132/deer-flow/pkg/llm"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

const (
	SessionKey = "leader:session"
)

const (
	Prompt = "<role>\nYou are DeerFlow 2.0, an open-source super agent.\n</role>\n\n{memory_context}\n\n<thinking_style>\n- Think concisely and strategically about the user's request BEFORE taking action\n- Break down the task: What is clear? What is ambiguous? What is missing?\n- **PRIORITY CHECK: If anything is unclear, missing, or has multiple interpretations, you MUST ask for clarification FIRST - do NOT proceed with work**\n{subagent_thinking}- Never write down your full final answer or report in thinking process, but only outline\n- CRITICAL: After thinking, you MUST provide your actual response to the user. Thinking is for planning, the response is for delivery.\n- Your response must contain the actual answer, not just a reference to what you thought about\n</thinking_style>\n\n{clarification_system}\n\n{skills_section}\n\n{subagent_section}\n\n{work_directory}\n\n<response_style>\n- Clear and Concise: Avoid over-formatting unless requested\n- Natural Tone: Use paragraphs and prose, not bullet points by default\n- Action-Oriented: Focus on delivering results, not explaining processes\n</response_style>\n\n<citations>\n- When to Use: After web_search, include citations if applicable\n- Format: Use Markdown link format `[citation:TITLE](URL)`\n- Example: \n```markdown\nThe key AI trends for 2026 include enhanced reasoning capabilities and multimodal integration\n[citation:AI Trends 2026](https://techcrunch.com/ai-trends).\nRecent breakthroughs in language models have also accelerated progress\n[citation:OpenAI Research](https://openai.com/research).\n```\n</citations>\n\n<critical_reminders>\n- **Clarification First**: ALWAYS clarify unclear/missing/ambiguous requirements BEFORE starting work - never assume or guess\n{subagent_reminder}- Skill First: Always load the relevant skill before starting **complex** tasks.\n- Progressive Loading: Load resources incrementally as referenced in skills\n- Output Files: Final deliverables must be in `/mnt/user-data/outputs`\n- Clarity: Be direct and helpful, avoid unnecessary meta-commentary\n- Including Images and Mermaid: Images and Mermaid diagrams are always welcomed in the Markdown format, and you're encouraged to use `![Image Description](image_path)\\n\\n` or \"```mermaid\" to display images in response or Markdown files\n- Multi-task: Better utilize parallel tool calling to call multiple tools at one time for better performance\n- Language Consistency: Keep using the same language as user's\n- Always Respond: Your thinking is internal. You MUST always provide a visible response to the user after thinking.\n</critical_reminders>"
)

type Leader struct {
	agent  adk.Agent
	memory *memory.Service
}

type Session struct {
	ThreadID string `json:"thread_id"`
	UserID   string `json:"user_id"`
	Memory   *memory.Memory
}

func NewLeaderByConfig(ctx context.Context, cache *redis.Client, cfg global.AgentConfig) (*Leader, error) {
	defaultLLM, err := llm.InitLLM(ctx, cfg.DefaultLLM)
	if err != nil {
		return nil, err
	}
	memoryLLM := defaultLLM
	if cfg.MemoryLLM != nil {
		memoryLLM, err = llm.InitLLM(ctx, cfg.MemoryLLM)
		if err != nil {
			return nil, err
		}
	}
	mem := memory.NewService(cache, memoryLLM)
	return NewLeader(ctx, defaultLLM, mem)
}

func genInput(ctx context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
	msgs := make([]adk.Message, 0, len(input.Messages)+1)

	if instruction != "" {
		sp := schema.SystemMessage(instruction)

		session, err := GetSession(ctx)
		if err != nil {
			return nil, err
		}

		memoryContext, err := getMemoryContext(ctx, session.Memory)
		if err != nil {
			return nil, err
		}

		vs := map[string]any{
			"memory_context":       memoryContext,
			"subagent_thinking":    "",
			"skills_section":       "",
			"subagent_section":     "",
			"subagent_reminder":    "",
			"clarification_system": "",
			"work_directory":       "",
		}
		ct := prompt.FromMessages(schema.FString, sp)
		ms, err := ct.Format(ctx, vs)
		if err != nil {
			return nil, fmt.Errorf("defaultGenModelInput: failed to format instruction using FString template. "+
				"This formatting is triggered automatically when SessionValues are present. "+
				"If your instruction contains literal curly braces (e.g., JSON), provide a custom GenModelInput that uses another format. If you are using "+
				"SessionValues for purposes other than instruction formatting, provide a custom GenModelInput that does no formatting at all: %w", err)
		}

		sp = ms[0]

		msgs = append(msgs, sp)
	}

	msgs = append(msgs, input.Messages...)

	return msgs, nil
}

func NewLeader(ctx context.Context, llm model.ToolCallingChatModel, mem *memory.Service) (*Leader, error) {
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "deer flow",
		Description:   "An agent is able to do anything",
		Instruction:   Prompt,
		Model:         llm,
		GenModelInput: genInput,
	})
	if err != nil {
		return nil, err
	}

	a := &Leader{
		agent:  agent,
		memory: mem,
	}

	return a, nil
}
func (l *Leader) BuildMemoryContext(ctx context.Context, userID, threadID string) (*memory.Memory, error) {
	mem, err := l.memory.GetMemory(ctx, userID, threadID)
	if err != nil {
		return nil, err
	}

	return mem, err
}

func getMemoryContext(ctx context.Context, mem *memory.Memory) (string, error) {
	content := mem.ToString()
	str := "<memory>\n{memory_content}\n</memory>"

	if content == "" {
		return "", nil
	}

	fString, err := llm.FString(ctx, str, map[string]any{"memory_content": content})
	if err != nil {
		return "", err
	}

	return fString, nil
}

// 之后都要用新的ctx

func (l *Leader) Run(ctx context.Context, userID, threadID string, messages []*schema.Message) (context.Context, *adk.AsyncIterator[*adk.AgentEvent], error) {
	newCtx, opts, err := l.Start(ctx, userID, threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "with options error: %v", err)
		return nil, nil, err
	}
	adkMessages := &adk.AgentInput{
		Messages:        messages,
		EnableStreaming: true,
	}
	iterator := l.agent.Run(newCtx, adkMessages, opts...)

	return newCtx, iterator, nil
}

func (l *Leader) Start(ctx context.Context, userID, threadID string) (context.Context, []adk.AgentRunOption, error) {
	// 初始化会话
	newCtx, session := InitCtx(ctx)

	// 获取记忆
	mem, err := l.memory.GetMemory(ctx, userID, threadID)

	if err != nil {
		zlog.CtxErrorf(ctx, "build memory context error: %v", err)
		return nil, nil, err
	}

	// 保持会话值
	session.UserID = userID
	session.ThreadID = threadID
	session.Memory = mem

	// 这个会话是eino的，封闭
	var opts []adk.AgentRunOption
	//sessionValue := map[string]any{
	//	"user_id":              userID,
	//	"thread_id":            threadID,
	//	"memory_context":       memoryContext,
	//	"subagent_thinking":    "",
	//	"skills_section":       "",
	//	"subagent_section":     "",
	//	"subagent_reminder":    "",
	//	"clarification_system": "",
	//	"work_directory":       "",
	//}
	//
	//opt := adk.WithSessionValues(sessionValue)
	//opts = append(opts, opt)
	return newCtx, opts, nil
}

func (l *Leader) Close(ctx context.Context, output []*schema.Message) error {
	session, err := GetSession(ctx)
	if err != nil {
		return err
	}

	// 更新记忆
	err = l.memory.GenMemory(ctx, session.UserID, session.ThreadID, session.Memory, output)
	if err != nil {
		zlog.CtxErrorf(ctx, "gen memory error: %v", err)
		return err
	}
	return nil
}

func InitCtx(ctx context.Context) (context.Context, *Session) {
	session := &Session{}
	return context.WithValue(ctx, SessionKey, session), session
}

func GetSession(ctx context.Context) (*Session, error) {
	session, ok := ctx.Value(SessionKey).(*Session)
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}
