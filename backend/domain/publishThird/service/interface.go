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

package service

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/model"
)

type PublishThird interface {

	//保存url信息
	SaveTweetUrl(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error)

	//查看数据
	GetTweetUrlList(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error)

	//修改数据
	UpdateTweetUrlById(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error)

	//根据id查询数据
	GetTweetUrlById(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error)

	//修改数据
	UpdateTweetUrl(ctx context.Context, request *ThirdUrlRequest) (response *ThirdResponse, err error)
}

type ThirdResponse struct {
	Msg              string
	PublishThirdList []*model.PublishThirdUrl
	Total            int64
	Code             int
}

type ThirdRequest struct {
	UserId       *int64
	UrlType      *int32
	Status       *int32
	Introduction *string
	Page         *int
	PageSize     *int
	Order        *int32
	Url          *string
	LikeCount    *int64
	CollectCount *int64
	ChatCount    *int64
	Id           *int64
}

type ThirdUrlRequest struct {
	PublishThirdUrl *model.PublishThirdUrl
}
