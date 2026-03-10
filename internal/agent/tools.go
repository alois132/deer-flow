package agent

import (
	"context"
	"errors"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"strings"
)

const (
	ToolBash       = "bash"
	ToolLs         = "ls"
	ToolReadFile   = "read_file"
	ToolWriteFile  = "write_file"
	ToolStrReplace = "str_replace"

	ToolBashDesc       = "Execute a bash command in a Linux environment.\n\n\n    - Use `python` to run Python code.\n    - Use `pip install` to install Python packages.\n\n    Args:\n        description: Explain why you are running this command in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        command: The bash command to execute. Always use absolute paths for files and directories."
	ToolLsDesc         = "List the contents of a directory up to 2 levels deep in tree format.\n\n    Args:\n        description: Explain why you are listing this directory in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the directory to list."
	ToolReadFileDesc   = "Read the contents of a text file. Use this to examine source code, configuration files, logs, or any text-based file.\n\n    Args:\n        description: Explain why you are reading this file in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the file to read.\n        start_line: Optional starting line number (1-indexed, inclusive). Use with end_line to read a specific range.\n        end_line: Optional ending line number (1-indexed, inclusive). Use with start_line to read a specific range."
	ToolWriteFileDesc  = "Write text content to a file.\n\n    Args:\n        description: Explain why you are writing to this file in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the file to write to. ALWAYS PROVIDE THIS PARAMETER SECOND.\n        content: The content to write to the file. ALWAYS PROVIDE THIS PARAMETER THIRD."
	ToolStrReplaceDesc = "Replace a substring in a file with another substring.\n    If `replace_all` is False (default), the substring to replace must appear **exactly once** in the file.\n\n    Args:\n        description: Explain why you are replacing the substring in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the file to replace the substring in. ALWAYS PROVIDE THIS PARAMETER SECOND.\n        old_str: The substring to replace. ALWAYS PROVIDE THIS PARAMETER THIRD.\n        new_str: The new substring. ALWAYS PROVIDE THIS PARAMETER FOURTH.\n        replace_all: Whether to replace all occurrences of the substring. If False, only the first occurrence will be replaced. Default is False."
)

type BashArg struct {
	Command string `json:"command"`
}
type LsArg struct {
	Path string `json:"path"`
}
type ReadFileArg struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type WriteFileArg struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append"`
}

type StrReplaceArg struct {
	Path       string `json:"path"`
	OldStr     string `json:"old_str"`
	NewStr     string `json:"new_str"`
	ReplaceAll bool   `json:"replace_all"`
}

func GetToolsConfig() (adk.ToolsConfig, []*schema.ToolInfo, error) {
	tools, err := GetTools()
	if err != nil {
		zlog.Errorf("get tools failed:%v", err)
		return adk.ToolsConfig{}, nil, err
	}
	ctx := context.Background()
	var toolsInfo []*schema.ToolInfo
	for _, baseTool := range tools {
		info, err := baseTool.Info(ctx)
		if err != nil {
			zlog.Errorf("get tool info failed:%v", err)
			return adk.ToolsConfig{}, nil, err
		}
		toolsInfo = append(toolsInfo, info)
	}

	cfg := adk.ToolsConfig{
		ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: tools,
		},
		EmitInternalEvents: true,
	}

	return cfg, toolsInfo, nil
}

func GetTools() (tools []tool.BaseTool, err error) {
	bashTool, err := utils.InferTool(ToolBash, ToolBashDesc, func(ctx context.Context, input BashArg) (output string, err error) {
		zlog.CtxInfof(ctx, "bash command:%s", input.Command)
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			zlog.Errorf("ensure sandbox initialized failed:%v", err)
			return "", err
		}
		executeCommand, err := sb.ExecuteCommand(ctx, input.Command)
		if err != nil {
			zlog.Errorf("execute command failed:%v", err)
			return "", err
		}
		zlog.CtxInfof(ctx, "execute command:%s", executeCommand)
		return executeCommand, nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, bashTool)

	lsTool, err := utils.InferTool(ToolLs, ToolLsDesc, func(ctx context.Context, input LsArg) (output string, err error) {
		zlog.CtxInfof(ctx, "list dir:%s", input.Path)
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		dir, err := sb.ListDir(ctx, input.Path, 2)
		if err != nil {
			zlog.Errorf("list dir failed:%v", err)
			return "", err
		}
		join := strings.Join(dir, "\n")
		zlog.CtxInfof(ctx, "list dir:%s", join)
		return join, nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, lsTool)
	readFileTool, err := utils.InferTool(ToolReadFile, ToolReadFileDesc, func(ctx context.Context, input ReadFileArg) (output string, err error) {
		zlog.CtxInfof(ctx, "read file:%s", input.Path)
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		content, err := sb.ReadFile(ctx, input.Path)
		if errors.Is(err, sandbox.ErrNoData) {
			zlog.CtxInfof(ctx, "file not exists:%s", input.Path)
			return "", nil
		}
		if err != nil {
			zlog.Errorf("read file failed:%v", err)
			return "", err
		}
		lines := strings.Split(content, "\n")
		if input.StartLine < 1 {
			input.StartLine = 1
		}
		if input.EndLine > len(lines) {
			input.EndLine = len(lines)
		}
		if input.StartLine > input.EndLine {
			return "", nil
		}
		content = strings.Join(lines[input.StartLine-1:input.EndLine], "\n")
		zlog.CtxInfof(ctx, "read file:%s", content)
		return content, nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, readFileTool)

	writeFileTool, err := utils.InferTool(ToolWriteFile, ToolWriteFileDesc, func(ctx context.Context, input WriteFileArg) (output string, err error) {
		zlog.CtxInfof(ctx, "write file:%s", input.Path)
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		err = sb.WriteFile(ctx, input.Path, input.Content, input.Append)
		if err != nil {
			zlog.Errorf("write file failed:%v", err)
			return "", err
		}
		return "OK", nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, writeFileTool)

	strReplaceTool, err := utils.InferTool(ToolStrReplace, ToolStrReplaceDesc, func(ctx context.Context, input StrReplaceArg) (output string, err error) {
		zlog.CtxInfof(ctx, "replace string arg:%+v", input)
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		content, err := sb.ReadFile(ctx, input.Path)
		if errors.Is(err, sandbox.ErrNoData) || content == "" {
			return "OK", nil
		}
		if err != nil {
			zlog.Errorf("read file failed:%v", err)
			return "", err
		}
		if !input.ReplaceAll {
			content = strings.Replace(content, input.OldStr, input.NewStr, 1)
		} else {
			content = strings.ReplaceAll(content, input.OldStr, input.NewStr)
		}
		err = sb.WriteFile(ctx, input.Path, content, false)
		if err != nil {
			zlog.Errorf("write file failed:%v", err)
			return "", err
		}
		zlog.CtxInfof(ctx, "replace file:%s", content)
		return "OK", nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, strReplaceTool)

	return tools, nil
}

func ensureSandboxInitialized(ctx context.Context) (sandbox.Sandbox, error) {
	session, err := GetSession(ctx)
	if err != nil {
		return nil, err
	}
	p := session.Provider
	sandboxID := session.SandboxID
	s, err := p.Get(ctx, sandboxID)
	if err != nil {
		zlog.CtxWarnf(ctx, "get sandbox error: %v", err)
		sandboxID, err = p.Acquire(ctx, session.ThreadID)
		session.SandboxID = sandboxID
		return p.Get(ctx, sandboxID)
	}
	return s, nil
}
