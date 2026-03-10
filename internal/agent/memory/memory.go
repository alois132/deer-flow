package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alois132/deer-flow/pkg/llm"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/alois132/deer-flow/utils/safe"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

type Category string

const (
	CategoryPreference = "preference"
	CategoryKnowledge  = "knowledge"
	CategoryContext    = "context"
	CategoryBehavior   = "behavior"
	CategoryGoal       = "goal"
)

const (
	Prompt = "You are a memory management system. Your task is to analyze a conversation and update the user's memory profile.\n\nCurrent Memory State:\n<current_memory>\n{current_memory}\n</current_memory>\n\nNew Conversation to Process:\n<conversation>\n{conversation}\n</conversation>\n\nInstructions:\n1. Analyze the conversation for important information about the user\n2. Extract relevant facts, preferences, and context with specific details (numbers, names, technologies)\n3. Update the memory sections as needed following the detailed length guidelines below\n\nMemory Section Guidelines:\n\n**User Context** (Current state - concise summaries):\n- work_context: Professional role, company, key projects, main technologies (2-3 sentences)\n  Example: Core contributor, project names with metrics (16k+ stars), technical stack\n- personal_context: Languages, communication preferences, key interests (1-2 sentences)\n  Example: Bilingual capabilities, specific interest areas, expertise domains\n- top_of_mind: Multiple ongoing focus areas and priorities (3-5 sentences, detailed paragraph)\n  Example: Primary project work, parallel technical investigations, ongoing learning/tracking\n  Include: Active implementation work, troubleshooting issues, market/research interests\n  Note: This captures SEVERAL concurrent focus areas, not just one task\n\n**History** (Temporal context - rich paragraphs):\n- recent_months: Detailed summary of recent activities (4-6 sentences or 1-2 paragraphs)\n  Timeline: Last 1-3 months of interactions\n  Include: Technologies explored, projects worked on, problems solved, interests demonstrated\n- earlier_context: Important historical patterns (3-5 sentences or 1 paragraph)\n  Timeline: 3-12 months ago\n  Include: Past projects, learning journeys, established patterns\n- long_term_background: Persistent background and foundational context (2-4 sentences)\n  Timeline: Overall/foundational information\n  Include: Core expertise, longstanding interests, fundamental working style\n\n**Facts Extraction**:\n- Extract specific, quantifiable details (e.g., \"16k+ GitHub stars\", \"200+ datasets\")\n- Include proper nouns (company names, project names, technology names)\n- Preserve technical terminology and version numbers\n- Categories:\n  * preference: Tools, styles, approaches user prefers/dislikes\n  * knowledge: Specific expertise, technologies mastered, domain knowledge\n  * context: Background facts (job title, projects, locations, languages)\n  * behavior: Working patterns, communication habits, problem-solving approaches\n  * goal: Stated objectives, learning targets, project ambitions\n- Confidence levels:\n  * 0.9-1.0: Explicitly stated facts (\"I work on X\", \"My role is Y\")\n  * 0.7-0.8: Strongly implied from actions/discussions\n  * 0.5-0.6: Inferred patterns (use sparingly, only for clear patterns)\n\n**What Goes Where**:\n- work_context: Current job, active projects, primary tech stack\n- personal_context: Languages, personality, interests outside direct work tasks\n- top_of_mind: Multiple ongoing priorities and focus areas user cares about recently (gets updated most frequently)\n  Should capture 3-5 concurrent themes: main work, side explorations, learning/tracking interests\n- recent_months: Detailed account of recent technical explorations and work\n- earlier_context: Patterns from slightly older interactions still relevant\n- long_term_background: Unchanging foundational facts about the user\n\n**Multilingual Content**:\n- Preserve original language for proper nouns and company names\n- Keep technical terms in their original form (DeepSeek, LangGraph, etc.)\n- Note language capabilities in personalContext\n\nOutput Format (JSON):\n{{\n  \"user\": {{\n    \"work_context\": {{ \"summary\": \"...\", \"should_update\": true/false }},\n    \"personal_context\": {{ \"summary\": \"...\", \"should_update\": true/false }},\n    \"top_of_mind\": {{ \"summary\": \"...\", \"should_update\": true/false }}\n  }},\n  \"history\": {{\n    \"recent_months\": {{ \"summary\": \"...\", \"should_update\": true/false }},\n    \"earlier_context\": {{ \"summary\": \"...\", \"should_update\": true/false }},\n    \"long_term_background\": {{ \"summary\": \"...\", \"should_update\": true/false }}\n  }},\n  \"new_facts\": [\n    {{ \"content\": \"...\", \"category\": \"preference|knowledge|context|behavior|goal\", \"confidence\": 0.0-1.0 }}\n  ],\n  \"facts_to_remove\": [\"fact_id_1\", \"fact_id_2\"]\n}}\n\nImportant Rules:\n- Only set shouldUpdate=true if there's meaningful new information\n- Follow length guidelines: workContext/personalContext are concise (1-3 sentences), topOfMind and history sections are detailed (paragraphs)\n- Include specific metrics, version numbers, and proper nouns in facts\n- Only add facts that are clearly stated (0.9+) or strongly implied (0.7+)\n- Remove facts that are contradicted by new information\n- When updating topOfMind, integrate new focus areas while removing completed/abandoned ones\n  Keep 3-5 concurrent focus themes that are still active and relevant\n- For history sections, integrate new information chronologically into appropriate time period\n- Preserve technical accuracy - keep exact names of technologies, companies, projects\n- Focus on information useful for future interactions and personalization\n\nReturn ONLY valid JSON, no explanation or markdown."
)

const (
	StoreKeyFMT = "deerFlow:userID,%s:memory"
)

type Memory struct {
	LastUpdated string  `json:"last_updated"`
	User        User    `json:"user"`
	History     History `json:"history"`
	Facts       []Fact  `json:"facts"`
}

type User struct {
	WorkContext     Context `json:"work_context"`
	PersonalContext Context `json:"personal_context"`
	TopOfMind       Context `json:"top_of_mind"`
}

type History struct {
	RecentMonths       Context `json:"recent_months"`
	EarlierContext     Context `json:"earlier_context"`
	LongTermBackground Context `json:"long_term_background"`
}

type Fact struct {
	ID         string   `json:"id"`
	Context    string   `json:"context"`
	Category   Category `json:"category"`
	Confidence float64  `json:"confidence"`
	CreatedAt  string   `json:"created_at"`
	Source     string   `json:"source"`
}

type Context struct {
	Summary   string `json:"summary"`
	UpdatedAt string `json:"updated_at"`
}

type Output struct {
	User          OutputUser    `json:"user"`
	History       OutputHistory `json:"history"`
	NewFacts      []OutputFact  `json:"new_facts"`
	FactsToRemove []string      `json:"facts_to_remove"`
}

type OutputUser struct {
	WorkContext     OutputContext `json:"work_context"`
	PersonalContext OutputContext `json:"personal_context"`
	TopOfMind       OutputContext `json:"top_of_mind"`
}
type OutputHistory struct {
	RecentMonths       OutputContext `json:"recent_months"`
	EarlierContext     OutputContext `json:"earlier_context"`
	LongTermBackground OutputContext `json:"long_term_background"`
}

type OutputFact struct {
	Content    string   `json:"content"`
	Category   Category `json:"category"`
	Confidence float64  `json:"confidence"`
}

type OutputContext struct {
	Summary      string `json:"summary"`
	ShouldUpdate bool   `json:"should_update"`
}
type Service struct {
	cache *redis.Client
	llm   model.ToolCallingChatModel
}

func NewService(cache *redis.Client, llm model.ToolCallingChatModel) *Service {
	return &Service{
		cache: cache,
		llm:   llm,
	}
}

func (m *Service) GetMemory(ctx context.Context, userID, threadID string) (*Memory, error) {
	mem, err := m.load(ctx, userID, threadID)
	if err != nil {
		zlog.CtxErrorf(ctx, "[memory] failed to load memory for user %s, err:%+v", userID, err)
		return nil, err
	}
	zlog.CtxInfof(ctx, "[memory] load memory for user %s, memory:%+v", userID, mem)
	return mem, nil
}

func (m *Service) GenMemory(ctx context.Context, userID, threadID string, memory *Memory, messages []*schema.Message) error {
	marshal, err := json.Marshal(memory)
	if err != nil {
		zlog.CtxErrorf(ctx, "[memory] failed to marshal memory for user %s, err:%+v", userID, err)
		return err
	}
	var conversation string
	for _, message := range messages {
		conversation += message.String() + "\n"
	}

	fString, err := llm.FString(ctx, Prompt, map[string]any{
		"current_memory": string(marshal),
		"conversation":   conversation,
	})
	if err != nil {
		zlog.CtxErrorf(ctx, "[memory] failed to execute template for user %s, err:%+v", userID, err)
		return err
	}
	system := schema.SystemMessage(fString)
	generate, err := m.llm.Generate(ctx, []*schema.Message{system})
	if err != nil {
		zlog.CtxErrorf(ctx, "[memory] failed to generate memory for user %s, err:%+v", userID, err)
		return err
	}
	outputStr := generate.Content
	var output Output
	err = safe.ParseJSON(outputStr, &output)
	if err != nil {
		zlog.CtxErrorf(ctx, "[memory] failed to unmarshal memory for user %s, content:%s", userID, outputStr)
		return err
	}
	newMemory := updateMemory(threadID, output, memory)

	if err := m.save(ctx, userID, threadID, newMemory); err != nil {
		zlog.CtxErrorf(ctx, "[memory] failed to save memory for user %s, err:%+v", userID, err)
		return err
	}
	zlog.CtxInfof(ctx, "[memory] update memory for user %s, memory:%s", userID, outputStr)
	return nil
}

func (m *Service) load(ctx context.Context, userID, threadID string) (*Memory, error) {
	memory := new(Memory)
	res := m.cache.Get(context.Background(), fmt.Sprintf(StoreKeyFMT, userID))
	result, err := res.Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return memory, nil
		}
		zlog.CtxErrorf(ctx, "[memory] failed to get memory for user %s, err:%+v", userID, err)
		return nil, err
	}
	err = json.Unmarshal([]byte(result), memory)
	if err != nil {
		return nil, err
	}

	return memory, nil
}

func (m *Service) save(ctx context.Context, userID, threadID string, memory *Memory) error {
	str, err := json.Marshal(memory)
	if err != nil {
		return err
	}
	return m.cache.Set(context.Background(), fmt.Sprintf(StoreKeyFMT, userID), string(str), redis.KeepTTL).Err()
}

func updateMemory(threadID string, output Output, memory *Memory) *Memory {
	nowStr := time.Now().String()
	newMemory := memory
	newMemory.LastUpdated = nowStr
	if output.User.WorkContext.ShouldUpdate {
		newMemory.User.WorkContext.Summary = output.User.WorkContext.Summary
		newMemory.User.WorkContext.UpdatedAt = nowStr
	}
	if output.User.PersonalContext.ShouldUpdate {
		newMemory.User.PersonalContext.Summary = output.User.PersonalContext.Summary
		newMemory.User.PersonalContext.UpdatedAt = nowStr
	}
	if output.User.TopOfMind.ShouldUpdate {
		newMemory.User.TopOfMind.Summary = output.User.TopOfMind.Summary
		newMemory.User.TopOfMind.UpdatedAt = nowStr
	}
	if output.History.EarlierContext.ShouldUpdate {
		newMemory.History.EarlierContext.Summary = output.History.EarlierContext.Summary
		newMemory.History.EarlierContext.UpdatedAt = nowStr
	}
	if output.History.LongTermBackground.ShouldUpdate {
		newMemory.History.LongTermBackground.Summary = output.History.LongTermBackground.Summary
		newMemory.History.LongTermBackground.UpdatedAt = nowStr
	}
	if output.History.RecentMonths.ShouldUpdate {
		newMemory.History.RecentMonths.Summary = output.History.RecentMonths.Summary
		newMemory.History.RecentMonths.UpdatedAt = nowStr
	}

	factsMap := make(map[string]Fact, len(newMemory.Facts))
	for _, fact := range newMemory.Facts {
		factsMap[fact.ID] = fact
	}
	for _, s := range output.FactsToRemove {
		if _, ok := factsMap[s]; ok {
			delete(factsMap, s)
		}
	}
	newFacts := make([]Fact, 0, len(factsMap)+len(output.NewFacts))
	for _, fact := range factsMap {
		newFacts = append(newFacts, fact)
	}

	for _, fact := range output.NewFacts {
		newFacts = append(newFacts, Fact{
			ID:         uuid.NewString(),
			Context:    fact.Content,
			Category:   fact.Category,
			Confidence: fact.Confidence,
			CreatedAt:  nowStr,
			Source:     threadID,
		})
	}

	newMemory.Facts = newFacts
	return newMemory
}

func (m *Memory) ToString() string {
	var buf bytes.Buffer
	work := m.User.WorkContext.Summary
	personal := m.User.PersonalContext.Summary
	topOfMind := m.User.TopOfMind.Summary
	if work != "" || personal != "" || topOfMind != "" {
		buf.WriteString("User Context:\n")
		if work != "" {
			buf.WriteString(fmt.Sprintf("- Work: %s\n", work))
		}
		if personal != "" {
			buf.WriteString(fmt.Sprintf("- Personal: %s\n", personal))
		}
		if topOfMind != "" {
			buf.WriteString(fmt.Sprintf("- Top Of Mind: %s\n", topOfMind))
		}
	}

	recent := m.History.RecentMonths.Summary
	earlier := m.History.EarlierContext.Summary
	longTermBackground := m.History.LongTermBackground.Summary
	if recent != "" || earlier != "" || longTermBackground != "" {
		buf.WriteString("History:\n")
		if recent != "" {
			buf.WriteString(fmt.Sprintf("- Recent: %s\n", recent))
		}
		if earlier != "" {
			buf.WriteString(fmt.Sprintf("- Earlier: %s\n", earlier))
		}
		if longTermBackground != "" {
			buf.WriteString(fmt.Sprintf("- Long Term Background: %s\n", longTermBackground))
		}
	}
	return buf.String()
}
