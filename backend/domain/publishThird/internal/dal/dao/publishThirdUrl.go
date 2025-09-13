package dao

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/model"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/query"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"gorm.io/gorm"
	"time"
)

type PublishThirdDAO struct {
	DB    *gorm.DB
	Query *query.Query
}

func (dao *PublishThirdDAO) Create(ctx context.Context, publishUrl *model.PublishThirdUrl) error {
	return dao.Query.PublishThirdUrl.WithContext(ctx).Create(publishUrl)
}

func (dao *PublishThirdDAO) Update(ctx context.Context, publishThirdUrl *model.PublishThirdUrl) error {
	k := dao.Query.PublishThirdUrl
	publishThirdUrl.UpdatedAt = time.Now().UnixMilli()
	err := k.WithContext(ctx).Where(k.ID.Eq(publishThirdUrl.ID)).Save(publishThirdUrl)
	return err
}

func (dao *PublishThirdDAO) GetByID(ctx context.Context, id int64) (*model.PublishThirdUrl, error) {
	k := dao.Query.PublishThirdUrl
	publishThirdUrl, err := k.WithContext(ctx).Where(k.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return publishThirdUrl, nil
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
	//fmt.Println(do.Debug().Find())
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
