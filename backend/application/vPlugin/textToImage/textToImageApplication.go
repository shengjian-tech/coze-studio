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

package textToImage

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/infra/contract/document/textToImage"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

// 发布请求
type TextToImageRequest struct {
	Prompt     string   `json:"prompt"`
	Content    string   `json:"content"`
	ImagePaths []string `json:"image_paths"`
}

type TextToImageApplicationService struct {
	//DomainSVC   service.PublishThird
	ttImg textToImage.TextToImg
}

func InitService(ctx context.Context, image textToImage.TextToImg) *TextToImageApplicationService {
	TextToImageApplicationSVC.ttImg = image
	return TextToImageApplicationSVC
}

var TextToImageApplicationSVC = &TextToImageApplicationService{}

// 响应结构体
type gentImageResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func (t *TextToImageApplicationService) GentImage(ctx context.Context, req *TextToImageRequest) (*gentImageResponse, error) {
	if req.Prompt == "" {
		return &gentImageResponse{Code: 500, Msg: "请输入提示词", Data: nil}, nil
	}
	img, err := t.ttImg.GenerateImage(ctx, req.Prompt)

	if err != nil {
		logs.CtxErrorf(ctx, "gentrate img failed %v", err)
		return &gentImageResponse{
			Code: 500,
			Msg:  "生成失败",
			Data: nil,
		}, nil
	}
	resp := &gentImageResponse{
		Code: 0,
		Msg:  "生成图片成功",
		Data: img,
	}
	return resp, nil
}
