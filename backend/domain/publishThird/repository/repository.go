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

package repository

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/dao"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/model"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/query"
	"gorm.io/gorm"
)

func NewPublishThirdDAO(db *gorm.DB) PublishThirdRepo {
	return &dao.PublishThirdDAO{DB: db, Query: query.Use(db)}
}

//go:generate mockgen -destination ../internal/mock/dal/dao/pubilshThirdUrl.go --package dao -source pubilshThirdUrl.go
type PublishThirdRepo interface {
	Create(ctx context.Context, pubilshThirdUrl *model.PublishThirdUrl) error
	FindXhsUrlByList(ctx context.Context, opts *entity.WherePublishThirdUrlOption) ([]*model.PublishThirdUrl, int64, error)
}
