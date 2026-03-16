package main

import (
	"fmt"
	"github.com/alois132/deer-flow/internal/agent"
	"github.com/alois132/deer-flow/internal/global"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/alois132/deer-flow/pkg/sandbox/aio"
	"github.com/cloudwego/eino/schema"
	"os"
	"strings"
)

func main() {
	global.Load("./config.yaml")
	cfg := global.GetCfg()
	provider, err := aio.NewProvider(&cfg.Sandbox)
	defer provider.Shutdown(global.GetCtx())
	if err != nil {
		panic(err)
	}
	leader, err := agent.NewLeaderByConfig(global.GetCtx(), global.GetCache(), provider, cfg.Agent)
	if err != nil {
		panic(err)
	}

	userID := strings.TrimSpace(os.Getenv("DEERFLOW_USER_ID"))
	if userID == "" {
		userID = "u_123456"
	}
	threadID := strings.TrimSpace(os.Getenv("DEERFLOW_THREAD_ID"))
	if threadID == "" {
		threadID = "t_123456"
	}
	messages := getMessages()
	newCtx, iter, err := leader.Run(global.GetCtx(), userID, threadID, messages)
	if err != nil {
		panic(err)
	}
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			zlog.CtxErrorf(newCtx, "error: %v", event.Err)
			break
		}
		output := event.Output.MessageOutput
		//st := output.MessageStream
		//defer st.Close()
		//for {
		//	message, err := st.Recv()
		//	if err != nil {
		//		if !errors.Is(err, io.EOF) {
		//			zlog.CtxErrorf(newCtx, "error: %v", err)
		//		}
		//		break
		//	}
		//	fmt.Printf("[role]%s: [content]%s\n[reason]%s\n", message.Role, message.Content, message.ReasoningContent)
		//}

		message, err := output.GetMessage()
		if err != nil {
			zlog.CtxErrorf(newCtx, "error: %v", err)
		}
		messages = append(messages, message)
		fmt.Printf("[role]%s: [content]%s\n[reason]%s\n", message.Role, message.Content, message.ReasoningContent)
	}

	err = leader.Close(newCtx, messages)
	if err != nil {
		panic(err)
	}
}

func getMessages() []*schema.Message {
	prompt := strings.TrimSpace(os.Getenv("DEERFLOW_PROMPT"))
	if prompt == "" {
		prompt = "请将tool工具都用一遍"
	}
	return []*schema.Message{
		schema.UserMessage(prompt),
	}
}
