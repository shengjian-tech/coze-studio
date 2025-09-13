package service

import (
	"context"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/entity"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/internal/dal/model"
	"github.com/coze-dev/coze-studio/backend/domain/publishThird/repository"
	"github.com/coze-dev/coze-studio/backend/infra/impl/cache/redis"
	"github.com/coze-dev/coze-studio/backend/infra/impl/idgen"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
	"gorm.io/gorm"
	"time"
)

func NewPublishThirdSVC(config *ThirdSVCConfig) PublishThird {
	svc := &publishThirdSVC{
		publishThirdRepo: repository.NewPublishThirdDAO(config.DB),
	}

	return svc
}

type ThirdSVCConfig struct {
	DB *gorm.DB // required
}

type publishThirdSVC struct {
	publishThirdRepo repository.PublishThirdRepo
}

// 保存
func (p *publishThirdSVC) SaveTweetUrl(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error) {
	now := time.Now().UnixMilli()
	cmdable := redis.New()
	genID, err := idgen.New(cmdable)
	if err != nil {
		return nil, errorx.New(500)
	}
	ID, ID_err := genID.GenID(ctx)
	if ID_err != nil {
		return nil, errorx.New(500)
	}

	if err = p.publishThirdRepo.Create(ctx, &model.PublishThirdUrl{
		ID:           ID,
		Introduction: *request.Introduction,
		CreatorID:    *request.UserId,
		Url:          *request.Url,
		UrlType:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
		Status:       1, // At present, the initialization of the vector library is triggered by the document, and the knowledge base has no init process
		LikeCount:    0,
		CollectCount: 0,
		ChatCount:    0,
	}); err != nil {
		return nil, errorx.New(500, errorx.KV("msg", err.Error()))
	}
	return &ThirdResponse{
		Msg: "ok",
	}, nil
}

// getInfo 获取点赞量、收藏量、评论量
func (p *publishThirdSVC) GetTweetUrlList(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error) {

	if request.UserId == nil {
		return nil, errorx.New(500, errorx.KV("msg", "用户id不能为空"))
	}
	opts := &entity.WherePublishThirdUrlOption{
		Status:       request.Status,
		UserID:       request.UserId,
		Introduction: request.Introduction,
		Page:         request.Page,
		PageSize:     request.PageSize,
		Order:        request.Order,
	}
	pos, total, err := p.publishThirdRepo.FindXhsUrlByList(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrKnowledgeDBCode, errorx.KV("msg", err.Error()))
	}
	return &ThirdResponse{
		PublishThirdList: pos,
		Total:            total,
		Msg:              "ok",
	}, nil
}

// 修改
func (p *publishThirdSVC) UpdateTweetUrlById(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error) {
	ptModel, err := p.publishThirdRepo.GetByID(ctx, *request.Id)
	if err != nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}
	if ptModel == nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}

	now := time.Now().UnixMilli()
	ptModel.LikeCount = *request.LikeCount
	ptModel.CollectCount = *request.CollectCount
	ptModel.ChatCount = *request.ChatCount
	ptModel.UpdatedAt = now
	if err = p.publishThirdRepo.Update(ctx, ptModel); err != nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}
	return &ThirdResponse{
		Msg:  "ok",
		Code: 0,
	}, nil
}

// 根据id获取TweetUrl
func (p *publishThirdSVC) GetTweetUrlById(ctx context.Context, request *ThirdRequest) (response *ThirdResponse, err error) {
	ptModel, err := p.publishThirdRepo.GetByID(ctx, *request.Id)
	if err != nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}
	if ptModel == nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}
	list := []*model.PublishThirdUrl{}
	list = append(list, ptModel)
	return &ThirdResponse{
		PublishThirdList: list,
		Msg:              "ok",
		Code:             0,
	}, nil
}

// 修改
func (p *publishThirdSVC) UpdateTweetUrl(ctx context.Context, request *ThirdUrlRequest) (response *ThirdResponse, err error) {
	ptModel := request.PublishThirdUrl
	if ptModel == nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}

	now := time.Now().UnixMilli()
	ptModel.UpdatedAt = now
	if err = p.publishThirdRepo.Update(ctx, ptModel); err != nil {
		return &ThirdResponse{
			Msg:  err.Error(),
			Code: 1,
		}, nil
	}
	return &ThirdResponse{
		Msg:  "ok",
		Code: 0,
	}, nil
}
