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

package publishThird

import (
	"github.com/coze-dev/coze-studio/backend/api/model/base"

	publishThird_commion "github.com/coze-dev/coze-studio/backend/api/model/publishThird/commion"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
)

type GetXHSRequest struct {
	ImageContent *entity.PublishImageContent            `thrift:"ImageContent,1,optional" json:"entity_type,omitempty"`
	PublishType  *publishThird_commion.PublishThirdType `thrift:"PublishType,2,required" json:"publish_type,omitempty"`
	Title        string                                 `thrift:"Title,3,required" json:"Title,required"`
	Content      string                                 `thrift:"Content,4,required" json:"Content,required"`
	ImagePaths   []string                               `thrift:"ImagePaths,5,optional" json:"ImagePaths,omitempty"`
	UserId       int64                                  `thrift:"UserId,6,optional" json:"UserId,omitempty"`
}

type GetTweetXHSRequest struct {
	TweetType *publishThird_commion.TweetType `thrift:"TweetType,1,optional" json:"TweetType,omitempty"`
	Data      []string                        `thrift:"Data,2,required" json:"Data,omitempty"`
	UserId    int64                           `thrift:"UserId,3,optional" json:"UserId,omitempty"`
}

type GetThirdLoginRequest struct {
	LoginType *publishThird_commion.LoginType `thrift:"LoginType,1,required" json:"LoginType,omitempty"`
	UserId    int64                           `thrift:"UserId,6,optional" json:"UserId,omitempty"`
}

type GetThirdUrlRequest struct {
	ThirdUrlType *publishThird_commion.LoginType `thrift:"LoginType,1,required" json:"LoginType,omitempty"`
	UserId       *int64                          `thrift:"UserId,2,optional" json:"UserId,omitempty"`
	UrlType      *int32                          `thrift:"UrlType,3,required" json:"UrlType,omitempty"`
	Status       *int32                          `thrift:"Status,4,required" json:"Status,omitempty"`
	Introduction *string                         `thrift:"Introduction,5,optional" json:"Introduction,omitempty"`
	Page         *int                            `thrift:"Page,6,required" json:"Page,omitempty"`
	PageSize     *int                            `thrift:"PageSize,7,required" json:"PageSize,omitempty"`
	Order        *int32                          `thrift:"Order,8,required" json:"Order,omitempty"`
	Url          *string                         `thrift:"Url,9,optional" json:"Url,omitempty"`
	CreatorID    *int64                          `thrift:"CreatorId,10,optional" json:"CreatorId,omitempty"`
	LikeCount    *int64                          `thrift:"LikeCount,11,optional" json:"LikeCount,omitempty"`
	CollectCount *int64                          `thrift:"CollectCount,12,optional" json:"CollectCount,omitempty"`
	ChatCount    *int64                          `thrift:"ChatCount,13,optional" json:"ChatCount,omitempty"`
	Id           *int64                          `thrift:"Id,14,optional" json:"Id,omitempty"`
}

func NewGetXHSRequest() *GetXHSRequest {
	return &GetXHSRequest{}
}

// vo
type XHSData struct {
	LoginQr    string `thift:"LoginQr,1,optional" json:"login_qr,omitempty"`
	PublishUrl string `thrift:"PublishUrl,2,optional"  json:"publish_url"`
}
type WxData struct {
	Content string `thift:"LoginQr,1,optional" json:"content,omitempty"`
}

// resp
type PublishThirdResponse[T any] struct {
	Code     int32          `thrift:"Code,1,required" form:"code,required" json:"code,required"`
	Message  string         `thrift:"Message,2,required" form:"message,required" json:"message,required"`
	Data     T              `thrift:"Data,3,optional" json:"data,omitempty"` //也许返回小红书也许返回微信公众号
	Total    int64          `thrift:"total,4,optional" form:"total" json:"total" query:"total"`
	BaseResp *base.BaseResp `thrift:"BaseResp,255,optional" form:"BaseResp" json:"BaseResp,omitempty" query:"BaseResp"`
}

type NoteInfo struct {
	URL          string
	LikeCount    string
	CollectCount string
	ChatCount    string
}

type ModelPublishThirdUrl struct {
	ID           int64  `gorm:"column:id;primaryKey;comment:id" json:"id"`                                        // id
	Introduction string `gorm:"column:introduction;not null;comment:链接介绍" json:"introduction"`                    // knowledge's name
	Url          string `gorm:"column:url;comment:url" json:"url"`                                                // app id // space id
	CreatedAt    int64  `gorm:"column:created_at;not null;comment:Create Time in Milliseconds" json:"created_at"` // Create Time in Milliseconds
	UpdatedAt    int64  `gorm:"column:updated_at;not null;comment:Update Time in Milliseconds" json:"updated_at"` // Update Time in Milliseconds // Delete Time
	Status       int32  `gorm:"column:status;not null;default:1;comment:0 删除, 1 正常" json:"status"`
	UrlType      int32  `gorm:"column:urlType;not null;default:1;comment:1 小红书" json:"urlType"`
	CreatorID    int64  `gorm:"column:creator_id;not null;comment:creator id" json:"creator_id"`
	LikeCount    int64  `gorm:"column:likeCount;not null;comment:点赞量" json:"likeCount"`
	CollectCount int64  `gorm:"column:collectCount;not null;comment:收藏量" json:"collectCount"`
	ChatCount    int64  `gorm:"column:chatCount;not null;comment:评论量" json:"chatCount"`
}
type ThirdUrlInfo struct {
	PublishThirdUrl ModelPublishThirdUrl
}

type PublishThirdUrl struct {
	Id           int64
	Introduction string
	URL          string
	CreatedAt    int64
	UpdatedAt    int64
	Status       int32
	UrlType      int32
	CreatorID    int64
	LikeCount    int64
	CollectCount int64
	ChatCount    int64
}

func (p *GetXHSRequest) GetPublishType() (v publishThird_commion.PublishThirdType) {
	//if !p.PublishType() {
	//	return GetProductListRequest_EntityType_DEFAULT
	//}
	//return *p.EntityType

	if p.PublishType == nil {
		return publishThird_commion.PublishThirdType_XSH
	}
	return *p.PublishType
}

func (p *GetTweetXHSRequest) GetTweetInfoType() (v publishThird_commion.TweetType) {
	//if !p.PublishType() {
	//	return GetProductListRequest_EntityType_DEFAULT
	//}
	//return *p.EntityType

	if p.TweetType == nil {
		return publishThird_commion.TweetType_XSH
	}
	return *p.TweetType
}

func (p *GetThirdLoginRequest) GetThirdLoginType() (v publishThird_commion.LoginType) {
	//if !p.PublishType() {
	//	return GetProductListRequest_EntityType_DEFAULT
	//}
	//return *p.EntityType

	if p.LoginType == nil {
		return publishThird_commion.LoginType_XSH
	}
	return *p.LoginType
}

func (p *GetThirdUrlRequest) GetThirdUrlType() (v publishThird_commion.LoginType) {
	//if !p.PublishType() {
	//	return GetProductListRequest_EntityType_DEFAULT
	//}
	//return *p.EntityType

	if p.ThirdUrlType == nil {
		return publishThird_commion.LoginType_XSH
	}
	return *p.ThirdUrlType
}
