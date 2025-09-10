/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package qwenTextToImage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/textToImage"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"io"
	"net/http"
	"time"
)

// Qwen 文生图 API 请求结构
type QwenImageGenerationRequest struct {
	Model      string         `json:"model"`
	Input      QwenInput      `json:"input"`
	Parameters QwenParameters `json:"parameters"`
}

type QwenInput struct {
	Messages []QwenMessage `json:"messages"`
}

type QwenMessage struct {
	Role    string        `json:"role"`
	Content []QwenContent `json:"content"`
}

type QwenContent struct {
	Text string `json:"text"`
}

type QwenParameters struct {
	NegativePrompt string `json:"negative_prompt,omitempty"` // 负面提示词
	PromptExtend   bool   `json:"prompt_extend,omitempty"`   // 是否扩展提示词
	Watermark      bool   `json:"watermark,omitempty"`       // 是否添加水印
	Size           string `json:"size,omitempty"`            // 图片尺寸，如 "1328*1328", "1024*1024", "1920*1080" 等
}

// Qwen API 响应结构
type QwenImageGenerationResponse struct {
	Output    QwenOutput `json:"output"`
	Usage     QwenUsage  `json:"usage"`
	RequestId string     `json:"request_id"`
}

type QwenOutput struct {
	Choices    []QwenChoices  `json:"choices"`
	TaskMetric QwenTaskMetric `json:"task_Metric"`
}
type QwenChoices struct {
	FinishReason string       `json:"finish_reason"`
	Message      QwenImageMsg `json:"message"`
}
type QwenImageMsg struct {
	Role    string             `json:"role"`
	Content []QwenImageContent `json:"content"`
}
type QwenImageContent struct {
	Image string `json:"image"`
}

type QwenTaskMetric struct {
	TOTAL     int `json:"TOTAL"`
	FAILED    int `json:"FAILED"`
	SUCCEEDED int `json:"SUCCEEDED"`
}

type QwenUsage struct {
	Width      int `json:"width"`
	Height     int `json:"height"`
	ImageCount int `json:"image_count"`
}

// 错误响应结构
type QwenErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id"`
}

// Qwen 客户端
type QwenClient struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewTextToImage(key string, url string, model string) textToImage.TextToImg {
	client := &qwenTextToImageImpl{}
	client.QwenClient = NewQwenClient(key, url, model)
	return client
}

// 创建新的 Qwen 客户端
func NewQwenClient(apiKey string, baseurl string, model string) *QwenClient {
	if baseurl == "" {
		baseurl = "https://dashscope.aliyuncs.com"
	}
	if model == "" {
		model = "qwen-image"
	}

	return &QwenClient{
		APIKey:  apiKey,
		BaseURL: baseurl,
		Model:   model,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type qwenTextToImageImpl struct {
	*QwenClient
}

func (c *qwenTextToImageImpl) GenerateImage(ctx context.Context, prompt string) (string, error) {
	req := &QwenImageGenerationRequest{
		Model: c.Model,
		Input: QwenInput{
			Messages: []QwenMessage{
				{
					Role: "user",
					Content: []QwenContent{
						{
							Text: prompt,
						},
					},
				},
			},
		},
		Parameters: QwenParameters{
			PromptExtend: true,
			Watermark:    true,
			Size:         "1328*1328",
		},
	}

	resp, err := c.GenerateImageRequest(ctx, req)
	if err != nil {
		logs.CtxErrorf(ctx, "qwen text to image failed %v", err)
		return "", err
	}

	if resp == nil {
		return "", fmt.Errorf("没有生成图片结果")
	}

	return resp.Output.Choices[0].Message.Content[0].Image, nil
}

// 生成图片
func (c *qwenTextToImageImpl) GenerateImageRequest(ctx context.Context, req *QwenImageGenerationRequest) (*QwenImageGenerationResponse, error) {
	// 设置默认值
	if req.Model == "" {
		req.Model = c.Model
	}
	if req.Parameters.Size == "" {
		req.Parameters.Size = "1328*1328"
	}

	// 序列化请求体
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	// 发送请求
	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		var errorResp QwenErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return nil, fmt.Errorf("API错误 (%d): %s", resp.StatusCode, errorResp.Message)
		}
		return nil, fmt.Errorf("HTTP错误 (%d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var imageResp QwenImageGenerationResponse
	if err := json.Unmarshal(body, &imageResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查任务状态
	if imageResp.Output.TaskMetric.FAILED > 0 {
		return nil, fmt.Errorf("任务执行失败，失败数: %s", imageResp.Output.TaskMetric.FAILED)
	}

	return &imageResp, nil
}

// 批量生成图片
func (c *qwenTextToImageImpl) GenerateBatch(ctx context.Context, prompts []string, options *QwenImageGenerationRequest) ([]string, error) {
	var urls []string

	for _, prompt := range prompts {
		req := &QwenImageGenerationRequest{
			Model: c.Model,
			Input: QwenInput{
				Messages: []QwenMessage{
					{
						Role: "user",
						Content: []QwenContent{
							{
								Text: prompt,
							},
						},
					},
				},
			},
			Parameters: QwenParameters{
				PromptExtend: true,
				Watermark:    true,
				Size:         "1328*1328",
			},
		}

		// 应用额外选项
		if options != nil {
			if options.Parameters.Size != "" {
				req.Parameters.Size = options.Parameters.Size
			}
			if options.Parameters.NegativePrompt != "" {
				req.Parameters.NegativePrompt = options.Parameters.NegativePrompt
			}
			req.Parameters.PromptExtend = options.Parameters.PromptExtend
			req.Parameters.Watermark = options.Parameters.Watermark
		}

		resp, err := c.GenerateImageRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("生成图片失败 (提示词: %s): %w", prompt, err)
		}

		if len(resp.Output.Choices) > 0 {
			urls = append(urls, resp.Output.Choices[0].Message.Content[0].Image)
		}

		// 添加延迟避免频率限制
		time.Sleep(1 * time.Second)
	}

	return urls, nil
}

// 支持自定义参数的生成方法
func (c *qwenTextToImageImpl) GenerateImageWithOptions(ctx context.Context, prompt string, options QwenParameters) (string, error) {
	req := &QwenImageGenerationRequest{
		Model: c.Model,
		Input: QwenInput{
			Messages: []QwenMessage{
				{
					Role: "user",
					Content: []QwenContent{
						{
							Text: prompt,
						},
					},
				},
			},
		},
		Parameters: options,
	}

	resp, err := c.GenerateImageRequest(ctx, req)
	if err != nil {
		logs.CtxErrorf(ctx, "qwen text to image with options failed %v", err)
		return "", err
	}

	if resp == nil {
		return "", fmt.Errorf("没有生成图片结果")
	}

	return resp.Output.Choices[0].Message.Content[0].Image, nil
}
