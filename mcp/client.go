package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Provider AI提供商类型
type Provider string

const (
	ProviderDeepSeek Provider = "deepseek"
	ProviderQwen     Provider = "qwen"
	ProviderCustom   Provider = "custom"
	ProviderGemini   Provider = "gemini"
)

// Client AI API配置
type Client struct {
	Provider     Provider
	APIKey       string
	SecretKey    string // 阿里云需要
	BaseURL      string
	Model        string
	Timeout      time.Duration
	UseFullURL   bool          // 是否使用完整URL（不添加/chat/completions）
	GeminiClient *genai.Client // Gemini客户端
}

func New() *Client {
	// 默认配置
	var defaultClient = Client{
		Provider: ProviderDeepSeek,
		BaseURL:  "https://api.deepseek.com/v1",
		Model:    "deepseek-chat",
		Timeout:  120 * time.Second, // 增加到120秒，因为AI需要分析大量数据
	}
	return &defaultClient
}

// SetDeepSeekAPIKey 设置DeepSeek API密钥
func (cfg *Client) SetDeepSeekAPIKey(apiKey string) {
	cfg.Provider = ProviderDeepSeek
	cfg.APIKey = apiKey
	cfg.BaseURL = "https://api.deepseek.com/v1"
	cfg.Model = "deepseek-chat"
}

// SetQwenAPIKey 设置阿里云Qwen API密钥
func (cfg *Client) SetQwenAPIKey(apiKey, secretKey string) {
	cfg.Provider = ProviderQwen
	cfg.APIKey = apiKey
	cfg.SecretKey = secretKey
	cfg.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	cfg.Model = "qwen-plus" // 可选: qwen-turbo, qwen-plus, qwen-max
}

// SetCustomAPI 设置自定义OpenAI兼容API
func (cfg *Client) SetCustomAPI(apiURL, apiKey, modelName string) {
	cfg.Provider = ProviderCustom
	cfg.APIKey = apiKey

	// 检查URL是否以#结尾，如果是则使用完整URL（不添加/chat/completions）
	if strings.HasSuffix(apiURL, "#") {
		cfg.BaseURL = strings.TrimSuffix(apiURL, "#")
		cfg.UseFullURL = true
	} else {
		cfg.BaseURL = apiURL
		cfg.UseFullURL = false
	}

	cfg.Model = modelName
	cfg.Timeout = 120 * time.Second
}

// SetGeminiAPIKey 设置Google Gemini API密钥
func (cfg *Client) SetGeminiAPIKey(apiKey string) error {
	fmt.Printf("gemini api key: %s\n", apiKey)
	cfg.Provider = ProviderGemini
	cfg.APIKey = apiKey
	cfg.Model = "gemini-3-pro-preview" // 默认使用最新的flash模型
	cfg.Timeout = 120 * time.Second

	// 创建Gemini客户端
	ctx := context.Background()
	clientConfig := &genai.ClientConfig{
		APIKey: apiKey,
	}

	// 配置代理（如果设置了环境变量）
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	// 检查代理环境变量
	proxyURL := "http://127.0.0.1:7897"

	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			fmt.Printf("⚠️ 解析代理URL失败: %v\n", err)
		} else {
			fmt.Printf("✓ 使用代理: %s\n", proxyURL)
			httpClient.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxy),
			}
		}
	}

	clientConfig.HTTPClient = httpClient

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return fmt.Errorf("创建Gemini客户端失败: %w", err)
	}
	cfg.GeminiClient = client
	return nil
}

// createHTTPClient 创建HTTP客户端，支持代理配置
func (cfg *Client) createHTTPClient() *http.Client {
	client := &http.Client{
		Timeout: cfg.Timeout,
	}
	return client
}

// SetClient 设置完整的AI配置（高级用户）
func (cfg *Client) SetClient(Client Client) {
	if Client.Timeout == 0 {
		Client.Timeout = 30 * time.Second
	}
	cfg = &Client
}

// CallWithMessages 使用 system + user prompt 调用AI API（推荐）
func (cfg *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if cfg.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用相应的设置方法")
	}

	// Gemini使用不同的调用方式
	if cfg.Provider == ProviderGemini {
		return cfg.callGemini(systemPrompt, userPrompt, nil)
	}

	// 重试配置
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("⚠️  AI API调用失败，正在重试 (%d/%d)...\n", attempt, maxRetries)
		}

		result, err := cfg.callOnce(systemPrompt, userPrompt)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("✓ AI API重试成功\n")
			}
			return result, nil
		}

		lastErr = err
		// 如果不是网络错误，不重试
		if !isRetryableError(err) {
			return "", err
		}

		// 重试前等待
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second
			fmt.Printf("⏳ 等待%v后重试...\n", waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// CallWithMessagesImage 使用 system + user prompt + image 调用AI API（支持图像）
func (cfg *Client) CallWithMessagesImage(systemPrompt, userPrompt string, imageData []byte) (string, error) {
	if cfg.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用相应的设置方法")
	}

	// 目前只有Gemini支持图像输入
	if cfg.Provider != ProviderGemini {
		return "", fmt.Errorf("图像输入目前只支持Gemini提供商")
	}

	return cfg.callGemini(systemPrompt, userPrompt, imageData)
}

// callOnce 单次调用AI API（内部使用）
func (cfg *Client) callOnce(systemPrompt, userPrompt string) (string, error) {
	// 构建 messages 数组
	messages := []map[string]string{}

	// 如果有 system prompt，添加 system message
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	// 添加 user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userPrompt,
	})

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       cfg.Model,
		"messages":    messages,
		"temperature": 0.5,  // 降低temperature以提高JSON格式稳定性
		"max_tokens":  4000, // 增加到4000以容纳思维链和JSON决策
	}

	// 注意：response_format 参数仅 OpenAI 支持，DeepSeek/Qwen 不支持
	// 我们通过强化 prompt 和后处理来确保 JSON 格式正确

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	var url string
	if cfg.UseFullURL {
		// 使用完整URL，不添加/chat/completions
		url = cfg.BaseURL
	} else {
		// 默认行为：添加/chat/completions
		url = fmt.Sprintf("%s/chat/completions", cfg.BaseURL)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 根据不同的Provider设置认证方式
	switch cfg.Provider {
	case ProviderDeepSeek:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	case ProviderQwen:
		// 阿里云Qwen使用API-Key认证
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
		// 注意：如果使用的不是兼容模式，可能需要不同的认证方式
	default:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	}

	// 发送请求
	client := cfg.createHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"` // DeepSeek reasoner特有字段
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		// 如果解析失败，打印原始响应用于调试
		fmt.Printf("⚠️ 解析响应JSON失败: %v\n原始响应: %s\n", err, string(body))
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API返回空响应")
	}

	// DeepSeek reasoner模型的推理内容在reasoning_content字段
	// 最终答案在content字段
	content := result.Choices[0].Message.Content
	reasoningContent := result.Choices[0].Message.ReasoningContent

	// 如果content为空但有reasoning_content，使用reasoning_content
	if content == "" && reasoningContent != "" {
		fmt.Printf("⚠️ content字段为空，使用reasoning_content字段\n")
		return reasoningContent, nil
	}

	// 如果两者都有内容，合并它们（推理过程 + 最终答案）
	if reasoningContent != "" && content != "" {
		return reasoningContent + "\n\n" + content, nil
	}

	if content == "" {
		fmt.Printf("⚠️ API返回的content和reasoning_content都为空\n原始响应: %s\n", string(body))
		return "", fmt.Errorf("API返回空内容")
	}

	return content, nil
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	errStr := err.Error()
	// 网络错误、超时、EOF等可以重试
	retryableErrors := []string{
		"EOF",
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"no such host",
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}

// callGemini 调用Gemini API（内部使用）
func (cfg *Client) callGemini(systemPrompt, userPrompt string, imageData []byte) (string, error) {
	if cfg.GeminiClient == nil {
		return "", fmt.Errorf("Gemini客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// 构建输入内容
	var parts []*genai.Part

	// 添加system prompt（如果有）
	if systemPrompt != "" {
		parts = append(parts, genai.NewPartFromText(systemPrompt+"\n\n"))
	}

	// 添加用户文本
	parts = append(parts, genai.NewPartFromText(userPrompt))

	// 添加图像（如果有）
	if imageData != nil {
		parts = append(parts, genai.NewPartFromBytes(imageData, "image/jpeg"))
	}

	// 构建内容
	content := genai.NewContentFromParts(parts, genai.RoleUser)

	// 调用Gemini API
	result, err := cfg.GeminiClient.Models.GenerateContent(
		ctx,
		cfg.Model,
		[]*genai.Content{content},
		&genai.GenerateContentConfig{},
	)
	if err != nil {
		return "", fmt.Errorf("gemini API调用失败: %w", err)
	}

	// 提取响应文本
	if result == nil || len(result.Candidates) == 0 {
		return "", fmt.Errorf("Gemini返回空响应")
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("Gemini返回空内容")
	}

	// 提取文本内容
	var responseText strings.Builder
	for _, part := range candidate.Content.Parts {
		// 根据实际的Part结构提取文本
		responseText.WriteString(fmt.Sprintf("%v", part))
	}

	if responseText.Len() == 0 {
		return "", fmt.Errorf("Gemini响应中没有文本内容")
	}

	return responseText.String(), nil
}
