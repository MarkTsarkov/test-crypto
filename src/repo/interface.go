package repo

import (
	"context"
	"github.com/marktsarkov/test/model"
	"time"
)

type Irepo interface {
	SaveClicks(ctx context.Context, data map[model.Click]int) error
	GetStats(ctx context.Context, bannerID int, tsFrom, tsTo time.Time) ([]model.ClickStat, error)
}
