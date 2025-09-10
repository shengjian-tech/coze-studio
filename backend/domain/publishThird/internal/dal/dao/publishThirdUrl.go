package dao

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"

	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/model"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/query"
	"gorm.io/gorm"
)

type PublishThirdDAO struct {
	DB    *gorm.DB
	Query *query.Query
}

func (dao *PublishThirdDAO) Create(ctx context.Context, publishUrl *model.PublishThirdUrl) error {
	return dao.Query.PublishThirdUrl.WithContext(ctx).Create(publishUrl)
}
func (dao *PublishThirdDAO) FindXhsUrlByList(ctx context.Context, opts *entity.WherePublishThirdUrlOption) (publishThirdUrl []*model.PublishThirdUrl, total int64, err error) {
	k := dao.Query.PublishThirdUrl
	do := k.WithContext(ctx).Debug()
	if opts == nil {
		return nil, 0, nil
	}
	if opts.Introduction != nil && len(*opts.Introduction) > 0 {
		do = do.Where(k.Introduction.Like("%" + *opts.Introduction + "%"))
	}
	if opts.Status != nil {
		do = do.Where(k.Status.Eq(*opts.Status))
	} else {
		return nil, 0, nil
	}
	if opts.UserID != nil && ptr.From(opts.UserID) != 0 {
		do = do.Where(k.CreatorID.Eq(*opts.UserID))
	} else {
		return nil, 0, nil
	}

	if opts.Order != nil {
		do = do.Order(k.CreatedAt.Asc())
	} else {
		do = do.Order(k.CreatedAt.Desc())
	}
	if opts.Page != nil && opts.PageSize != nil {
		offset := (*opts.Page - 1) * (*opts.PageSize)
		do = do.Limit(*opts.PageSize).Offset(offset)
	}
	publishThirdUrl, err = do.Find()
	if err != nil {
		return nil, 0, err
	}
	total, err = do.Limit(-1).Offset(-1).Count()
	if err != nil {
		return nil, 0, err
	}
	return publishThirdUrl, total, err
}
func (dao *PublishThirdDAO) InitTx() (tx *gorm.DB, err error) {
	tx = dao.DB.Begin()
	if tx.Error != nil {
		return nil, err
	}
	return
}
