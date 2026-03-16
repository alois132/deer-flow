package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/alois132/deer-flow/internal/global"
	"github.com/alois132/deer-flow/pkg/log/zlog"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/alois132/deer-flow/pkg/skills"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"path"
	"strings"
)

const (
	ToolBash       = "bash"
	ToolLs         = "ls"
	ToolReadFile   = "read_file"
	ToolWriteFile  = "write_file"
	ToolStrReplace = "str_replace"
	ToolReadSkill  = "read_skill"
	ToolReadRef    = "read_reference"
	ToolUseScript  = "use_script"
	ToolTask       = "task"

	ToolBashDesc       = "Execute a bash command in a Linux environment.\n\n\n    - Use `python` to run Python code.\n    - Use `pip install` to install Python packages.\n\n    Args:\n        description: Explain why you are running this command in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        command: The bash command to execute. Always use absolute paths for files and directories."
	ToolLsDesc         = "List the contents of a directory up to 2 levels deep in tree format.\n\n    Args:\n        description: Explain why you are listing this directory in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the directory to list."
	ToolReadFileDesc   = "Read the contents of a text file. Use this to examine source code, configuration files, logs, or any text-based file.\n\n    Args:\n        description: Explain why you are reading this file in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the file to read.\n        start_line: Optional starting line number (1-indexed, inclusive). Use with end_line to read a specific range.\n        end_line: Optional ending line number (1-indexed, inclusive). Use with start_line to read a specific range."
	ToolWriteFileDesc  = "Write text content to a file.\n\n    Args:\n        description: Explain why you are writing to this file in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the file to write to. ALWAYS PROVIDE THIS PARAMETER SECOND.\n        content: The content to write to the file. ALWAYS PROVIDE THIS PARAMETER THIRD."
	ToolStrReplaceDesc = "Replace a substring in a file with another substring.\n    If `replace_all` is False (default), the substring to replace must appear **exactly once** in the file.\n\n    Args:\n        description: Explain why you are replacing the substring in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        path: The **absolute** path to the file to replace the substring in. ALWAYS PROVIDE THIS PARAMETER SECOND.\n        old_str: The substring to replace. ALWAYS PROVIDE THIS PARAMETER THIRD.\n        new_str: The new substring. ALWAYS PROVIDE THIS PARAMETER FOURTH.\n        replace_all: Whether to replace all occurrences of the substring. If False, only the first occurrence will be replaced. Default is False."
	ToolReadSkillDesc  = "Read a skill definition from the skills directory.\n\n    Args:\n        description: Explain why you are reading this skill in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        skill: The skill name (directory name under the skills root)."
	ToolReadRefDesc    = "Read a reference file for a skill from the skills directory.\n\n    Args:\n        description: Explain why you are reading this reference in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        skill: The skill name (directory name under the skills root).\n        reference: The reference file path relative to the skill's references directory."
	ToolUseScriptDesc  = "Run a script for a skill using a specified interpreter.\n\n    Args:\n        description: Explain why you are running this script in short words. ALWAYS PROVIDE THIS PARAMETER FIRST.\n        skill: The skill name (directory name under the skills root).\n        script: Script path. If only a filename is provided, it is resolved under the skill's scripts directory. If a relative path with a directory is provided (for example `eval-viewer/generate_review.py`), it is resolved from the skill root.\n        interpreter: Optional interpreter (e.g., bash, sh, python, python3). Defaults to bash.\n        args: Optional arguments passed to the script."
	ToolTaskDesc       = "Delegate work to a subagent and collect results.\n\n    Args:\n        description: The subtask content for the subagent when operation is `run`.\n        operation: Optional. `run` (default) or `result`.\n        mode: Optional when operation is `run`. `sync` (default) waits for subagent result, `async` returns task_id immediately.\n        subagent_type: Optional subagent type. Default is `general`.\n        task_id: Required when operation is `result`."
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

type ReadSkillArg struct {
	Skill string `json:"skill"`
}

type ReadReferenceArg struct {
	Skill     string `json:"skill"`
	Reference string `json:"reference"`
}

type UseScriptArg struct {
	Skill       string `json:"skill"`
	Script      string `json:"script"`
	Interpreter string `json:"interpreter"`
	Args        string `json:"args"`
}

type TaskArg struct {
	Description  string `json:"description"`
	Operation    string `json:"operation"`
	Mode         string `json:"mode"`
	SubagentType string `json:"subagent_type"`
	TaskID       string `json:"task_id"`
}

type toolsOptions struct {
	taskExecutor TaskExecutor
}

type ToolsOption func(*toolsOptions)

func WithTaskExecutor(executor TaskExecutor) ToolsOption {
	return func(opts *toolsOptions) {
		opts.taskExecutor = executor
	}
}

func GetToolsConfig(options ...ToolsOption) (adk.ToolsConfig, []*schema.ToolInfo, error) {
	tools, err := GetTools(options...)
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

func GetTools(options ...ToolsOption) (tools []tool.BaseTool, err error) {
	opts := getToolsOptions(options...)

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

	readSkillTool, err := utils.InferTool(ToolReadSkill, ToolReadSkillDesc, func(ctx context.Context, input ReadSkillArg) (output string, err error) {
		cfg := global.GetCfg()
		if cfg == nil || !skillsEnabledForTools(cfg.Sandbox.Skills) {
			return "", fmt.Errorf("skills disabled")
		}
		skillName, err := sanitizeSkillName(input.Skill)
		if err != nil {
			return "", err
		}
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		skillPath := path.Join(skillsContainerPath(), skillName, "SKILL.md")
		content, err := sb.ReadFile(ctx, skillPath)
		if errors.Is(err, sandbox.ErrNoData) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return content, nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, readSkillTool)

	readRefTool, err := utils.InferTool(ToolReadRef, ToolReadRefDesc, func(ctx context.Context, input ReadReferenceArg) (output string, err error) {
		cfg := global.GetCfg()
		if cfg == nil || !skillsEnabledForTools(cfg.Sandbox.Skills) {
			return "", fmt.Errorf("skills disabled")
		}
		skillName, err := sanitizeSkillName(input.Skill)
		if err != nil {
			return "", err
		}
		refPath, err := normalizeSkillSubPath(input.Reference, "references")
		if err != nil {
			return "", err
		}
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		fullPath := path.Join(skillsContainerPath(), skillName, "references", refPath)
		content, err := sb.ReadFile(ctx, fullPath)
		if errors.Is(err, sandbox.ErrNoData) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return content, nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, readRefTool)

	useScriptTool, err := utils.InferTool(ToolUseScript, ToolUseScriptDesc, func(ctx context.Context, input UseScriptArg) (output string, err error) {
		cfg := global.GetCfg()
		if cfg == nil || !skillsEnabledForTools(cfg.Sandbox.Skills) {
			return "", fmt.Errorf("skills disabled")
		}
		skillName, err := sanitizeSkillName(input.Skill)
		if err != nil {
			return "", err
		}
		scriptPath, err := resolveSkillScriptPath(input.Script)
		if err != nil {
			return "", err
		}
		interpreter, err := sanitizeInterpreter(input.Interpreter)
		if err != nil {
			return "", err
		}
		sb, err := ensureSandboxInitialized(ctx)
		if err != nil {
			return "", err
		}
		skillRoot := path.Join(skillsContainerPath(), skillName)
		fullPath := path.Join(skillRoot, scriptPath)
		cmd := buildSkillScriptCommand(skillRoot, fullPath, interpreter, input.Args)
		executeCommand, err := sb.ExecuteCommand(ctx, cmd)
		if err != nil {
			return "", err
		}
		return executeCommand, nil
	})
	if err != nil {
		return nil, err
	}
	tools = append(tools, useScriptTool)

	if opts.taskExecutor != nil {
		taskTool, err := utils.InferTool(ToolTask, ToolTaskDesc, func(ctx context.Context, input TaskArg) (string, error) {
			normalized, err := normalizeTaskArg(input)
			if err != nil {
				return "", err
			}

			switch normalized.Operation {
			case TaskOperationRun:
				if normalized.Mode == TaskModeSync {
					result, err := opts.taskExecutor.RunSync(ctx, normalized.SubagentType, normalized.Description)
					if err != nil {
						return "", err
					}
					return sonic.MarshalString(map[string]any{
						"operation":     TaskOperationRun,
						"mode":          TaskModeSync,
						"status":        TaskStatusSucceeded,
						"subagent_type": normalized.SubagentType,
						"result":        result,
					})
				}
				taskID, err := opts.taskExecutor.RunAsync(ctx, normalized.SubagentType, normalized.Description)
				if err != nil {
					return "", err
				}
				return sonic.MarshalString(map[string]any{
					"operation":     TaskOperationRun,
					"mode":          TaskModeAsync,
					"status":        "accepted",
					"subagent_type": normalized.SubagentType,
					"task_id":       taskID,
				})
			case TaskOperationResult:
				result, err := opts.taskExecutor.GetAsyncResult(ctx, normalized.TaskID)
				if err != nil {
					return "", err
				}
				return sonic.MarshalString(result)
			default:
				return "", fmt.Errorf("unsupported operation: %s", normalized.Operation)
			}
		})
		if err != nil {
			return nil, err
		}
		tools = append(tools, taskTool)
	}

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

func skillsContainerPath() string {
	cfg := global.GetCfg()
	if cfg == nil {
		return skills.DefaultContainerPath
	}
	if !skillsEnabledForTools(cfg.Sandbox.Skills) {
		return skills.DefaultContainerPath
	}
	if strings.TrimSpace(cfg.Sandbox.Skills.ContainerPath) == "" {
		return skills.DefaultContainerPath
	}
	return cfg.Sandbox.Skills.ContainerPath
}

func skillsEnabledForTools(cfg sandbox.Skills) bool {
	if cfg.Enabled == nil {
		return true
	}
	return *cfg.Enabled
}

func sanitizeSkillName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("skill name is required")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid skill name: %s", name)
	}
	return name, nil
}

func sanitizeRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	clean := path.Clean(rel)
	if strings.HasPrefix(clean, "/") || clean == "." || clean == ".." {
		return "", fmt.Errorf("invalid path: %s", rel)
	}
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("invalid path: %s", rel)
	}
	return clean, nil
}

func normalizeSkillSubPath(rel, parent string) (string, error) {
	clean, err := sanitizeRelativePath(rel)
	if err != nil {
		return "", err
	}
	parent = strings.Trim(strings.TrimSpace(parent), "/")
	if parent == "" {
		return clean, nil
	}
	prefix := parent + "/"
	if clean == parent {
		return "", fmt.Errorf("invalid path: %s", rel)
	}
	if strings.HasPrefix(clean, prefix) {
		clean = strings.TrimPrefix(clean, prefix)
	}
	if strings.TrimSpace(clean) == "" {
		return "", fmt.Errorf("invalid path: %s", rel)
	}
	return clean, nil
}

func resolveSkillScriptPath(script string) (string, error) {
	clean, err := sanitizeRelativePath(script)
	if err != nil {
		return "", err
	}
	if strings.Contains(clean, "/") {
		return clean, nil
	}
	return path.Join("scripts", clean), nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func buildSkillScriptCommand(skillRoot, scriptPath, interpreter, args string) string {
	cmd := fmt.Sprintf("%s %s", interpreter, shellQuote(scriptPath))
	if interpreter == "python" || interpreter == "python3" {
		cmd = fmt.Sprintf("PYTHONPATH=%s %s %s", shellQuote(skillRoot), interpreter, shellQuote(scriptPath))
	}
	if strings.TrimSpace(args) != "" {
		cmd += " " + args
	}
	return cmd
}

func sanitizeInterpreter(interpreter string) (string, error) {
	interpreter = strings.TrimSpace(interpreter)
	if interpreter == "" {
		interpreter = "bash"
	}
	allowed := allowedInterpreters()
	for _, item := range allowed {
		if interpreter == item {
			return interpreter, nil
		}
	}
	return "", fmt.Errorf("interpreter not allowed: %s", interpreter)
}

func allowedInterpreters() []string {
	cfg := global.GetCfg()
	if cfg != nil && len(cfg.Sandbox.Skills.AllowedInterpreters) > 0 {
		return cfg.Sandbox.Skills.AllowedInterpreters
	}
	return []string{"bash", "sh", "python", "python3"}
}

func getToolsOptions(options ...ToolsOption) *toolsOptions {
	opts := &toolsOptions{}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(opts)
	}
	return opts
}

func normalizeTaskArg(input TaskArg) (TaskArg, error) {
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	if input.Operation == "" {
		input.Operation = TaskOperationRun
	}
	switch input.Operation {
	case TaskOperationRun:
		input.SubagentType = strings.TrimSpace(input.SubagentType)
		if input.SubagentType == "" {
			input.SubagentType = DefaultSubagentType
		}
		input.Description = strings.TrimSpace(input.Description)
		if input.Description == "" {
			return input, fmt.Errorf("description is required when operation=run")
		}
		input.Mode = strings.ToLower(strings.TrimSpace(input.Mode))
		if input.Mode == "" {
			input.Mode = TaskModeSync
		}
		if input.Mode != TaskModeSync && input.Mode != TaskModeAsync {
			return input, fmt.Errorf("mode must be one of [%s, %s]", TaskModeSync, TaskModeAsync)
		}
		return input, nil
	case TaskOperationResult:
		input.TaskID = strings.TrimSpace(input.TaskID)
		if input.TaskID == "" {
			return input, fmt.Errorf("task_id is required when operation=result")
		}
		return input, nil
	default:
		return input, fmt.Errorf("operation must be one of [%s, %s]", TaskOperationRun, TaskOperationResult)
	}
}
