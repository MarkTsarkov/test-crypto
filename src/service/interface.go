package service

import (
	"context"
	"github.com/marktsarkov/test/model"
	"time"
)

type Iservice interface {
	SaveClick(bannerID int) error
	GetStats(ctx context.Context, bannerID int, tsFrom, tsTo time.Time) (data []model.ClickStat, err error)
	ParallelSender(ctx context.Context)
	Close(ctx context.Context)
}
