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

package entity

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	ImagePaths []string
}

type WherePublishThirdUrlOption struct {
	url          *string
	Introduction *string // Exact match
	Status       *int32
	UserID       *int64
	Query        *string // fuzzy match
	Page         *int
	PageSize     *int
	UrlType      *int32
	Order        *int32
}
