package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/penzhan8451/fangclaw-go/internal/config"
	"github.com/penzhan8451/fangclaw-go/internal/hands"
	"github.com/penzhan8451/fangclaw-go/internal/memory"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/agent/tools"
	"github.com/penzhan8451/fangclaw-go/internal/runtime/llm"
	"github.com/penzhan8451/fangclaw-go/internal/skills"
	"github.com/penzhan8451/fangclaw-go/internal/types"
)

func chatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat [agent]",
		Short: "Interactive chat with an agent",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runChat,
	}
	return cmd
}

func runChat(cmd *cobra.Command, args []string) error {
	var agentID string

	if len(args) > 0 {
		agentID = args[0]
	} else {
		agentID = "default"
	}

	if isDaemonRunning() {
		return runChatWithDaemon(agentID)
	}

	return runChatLocal(agentID)
}

func runChatWithDaemon(agentID string) error {
	resp, err := http.Get("http://127.0.0.1:4200/api/agents")
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return runChatLocal(agentID)
	}

	fmt.Printf("Chatting with agent: %s\n", agentID)
	fmt.Println("Enter your message (Ctrl+C to exit):")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			fmt.Print("> ")
			continue
		}

		if strings.ToLower(text) == "exit" || strings.ToLower(text) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		messageReq := map[string]string{
			"message": text,
		}
		jsonData, _ := json.Marshal(messageReq)

		client := &http.Client{}
		req, _ := http.NewRequest("POST",
			fmt.Sprintf("http://127.0.0.1:4200/api/agents/%s/message", agentID),
			strings.NewReader(string(jsonData)))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			if respText, ok := result["response"].(string); ok {
				fmt.Printf("[%s] %s\n\n", agentID, respText)
			}
			resp.Body.Close()
		}

		fmt.Print("> ")
	}

	return nil
}

func runChatLocal(agentID string) error {
	fmt.Println("Starting interactive chat (local mode with AgentLoop)...")
	fmt.Printf("Chatting with agent: %s\n\n", agentID)

	// 1. 创建数据库
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dbDir := filepath.Join(homeDir, ".fangclaw-go")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}
	dbPath := filepath.Join(dbDir, "fangclaw-go.db")
	db, err := memory.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// 2. 创建 memory store
	semanticStore, err := memory.NewSemanticStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create semantic store: %w", err)
	}
	defer semanticStore.Close()

	sessionStore, err := memory.NewSessionStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create session store: %w", err)
	}
	defer sessionStore.Close()

	knowledgeStore := memory.NewKnowledgeStore(db)
	usageStore := memory.NewUsageStore(db)

	// 2.5. 创建 skills loader
	skillsPath := filepath.Join(dbDir, "skills")
	skillLoader, err := skills.NewLoader(skillsPath)
	if err != nil {
		fmt.Printf("Warning: failed to create skill loader: %v\n", err)
	}

	// 3. 创建 Agent Runtime
	runtime := agent.NewRuntime(semanticStore, sessionStore, knowledgeStore, usageStore, skillLoader)

	// 4. 获取 LLM driver
	driver, err := getLLMDriver()
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
		fmt.Println("Falling back to echo mode...")
		return runChatEcho(agentID)
	}

	// 5. 注册 LLM driver 和工具
	cfg, err := config.Load("")
	var providerName string
	if err == nil && cfg.DefaultModel.Provider != "" {
		providerName = cfg.DefaultModel.Provider
	} else {
		providerName = "openai"
	}
	runtime.RegisterDriver(providerName, driver)
	tools.RegisterAllTools(runtime)

	// 6. 创建 AgentContext
	var systemPrompt string
	var toolNames []string
	var modelName string
	var skillPromptContext string
	if hand, _ := hands.GetBundledHand(agentID); hand != nil {
		systemPrompt = getHandSystemPrompt(agentID)
		skillPromptContext = hand.SkillContent
	} else {
		// 默认的系统提示词
		systemPrompt = "You are a helpful assistant."
	}

	// 获取实际的 model 名称
	if cfg, err := config.Load(""); err == nil && cfg.DefaultModel.Model != "" {
		modelName = cfg.DefaultModel.Model
	} else {
		// 默认值
		if providerName == "openai" {
			modelName = "gpt-4o"
		} else if providerName == "anthropic" {
			modelName = "claude-sonnet-4-20250514"
		} else if providerName == "groq" {
			modelName = "groq/llama-3.3-70b-versatile"
		} else {
			modelName = "meta-llama/llama-3.1-8b-instruct"
		}
	}

	agentCtx := agent.NewAgentContext(
		agentID,
		agentID,
		providerName,
		modelName,
		systemPrompt,
		toolNames,
		[]string{},
		skillPromptContext,
	)

	fmt.Println("Enter your message (Ctrl+C to exit):")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			fmt.Print("> ")
			continue
		}

		if strings.ToLower(text) == "exit" || strings.ToLower(text) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// 添加用户消息
		userMsg := types.Message{
			ID:        fmt.Sprintf("msg_%d", time.Now().Unix()),
			Role:      "user",
			Content:   text,
			Timestamp: time.Now(),
		}
		agentCtx.AddMessage(userMsg)

		// 运行 AgentLoop
		ctx := context.Background()
		onPhase := func(phase agent.LoopPhase) {
			switch phase {
			case agent.PhaseThinking:
				fmt.Print("\n[Thinking...] ")
			case agent.PhaseToolUse:
				fmt.Print("\n[Using tools...] ")
			case agent.PhaseDone:
				fmt.Println("\n")
			}
		}

		result, err := runtime.RunAgentLoop(ctx, agentCtx, onPhase)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
		} else {
			fmt.Printf("[%s] %s\n\n", agentID, result.Response)
			fmt.Printf("(Tokens used: %d input, %d output, %d total)\n",
				result.TotalUsage.PromptTokens,
				result.TotalUsage.CompletionTokens,
				result.TotalUsage.TotalTokens)
		}

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

func runChatEcho(agentID string) error {
	fmt.Println("Enter your message (Ctrl+C to exit):")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			fmt.Print("> ")
			continue
		}

		if strings.ToLower(text) == "exit" || strings.ToLower(text) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		fmt.Printf("[%s] %s\n\n", agentID, text)
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

func getLLMDriver() (llm.Driver, error) {
	cfg, err := config.Load("")
	if err == nil && cfg.DefaultModel.Provider != "" && cfg.DefaultModel.Model != "" {
		provider := cfg.DefaultModel.Provider
		model := cfg.DefaultModel.Model
		apiKeyEnv := cfg.DefaultModel.APIKeyEnv
		if apiKeyEnv == "" {
			apiKeyEnv = strings.ToUpper(provider) + "_API_KEY"
		}
		apiKey := os.Getenv(apiKeyEnv)
		if apiKey != "" {
			driver, err := llm.NewDriver(provider, apiKey, model)
			if err == nil {
				return driver, nil
			}
		}
	}

	provider := "openrouter"
	model := "meta-llama/llama-3.1-8b-instruct"
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	if apiKey == "" {
		provider = "openai"
		model = "gpt-4o"
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		provider = "anthropic"
		model = "claude-sonnet-4-20250514"
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if apiKey == "" {
		provider = "groq"
		model = "groq/llama-3.3-70b-versatile"
		apiKey = os.Getenv("GROQ_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found. Set OPENROUTER_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, or GROQ_API_KEY")
	}

	return llm.NewDriver(provider, apiKey, model)
}

func getHandSystemPrompt(handID string) string {
	switch handID {
	case "researcher":
		return hands.ResearcherSystemPrompt
	case "lead":
		return hands.LeadSystemPrompt
	case "collector":
		return hands.CollectorSystemPrompt
	case "predictor":
		return hands.PredictorSystemPrompt
	case "clip":
		return hands.ClipSystemPrompt
	case "twitter":
		return hands.TwitterSystemPrompt
	case "browser":
		return hands.BrowserSystemPrompt
	default:
		return ""
	}
}

func messageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message <agent> <text>",
		Short: "Send a one-shot message to an agent",
		Args:  cobra.ExactArgs(2),
		RunE:  runMessage,
	}

	cmd.Flags().BoolVar(&messageJSON, "json", false, "Output as JSON")

	return cmd
}

var messageJSON bool

func runMessage(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	text := args[1]

	if !isDaemonRunning() {
		if messageJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]string{
				"error": "daemon not running",
			})
			return nil
		}
		return fmt.Errorf("daemon not running. Start with 'fangclaw-go start'")
	}

	// Send message to daemon
	messageReq := map[string]string{
		"message": text,
	}
	jsonData, _ := json.Marshal(messageReq)

	resp, err := http.Post(
		fmt.Sprintf("http://127.0.0.1:4200/api/agents/%s/message", agentID),
		"application/json",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if messageJSON {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		json.NewEncoder(os.Stdout).Encode(result)
		return nil
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if respText, ok := result["response"].(string); ok {
		fmt.Println(respText)
	}

	return nil
}
