package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/marktsarkov/test/model"
	"github.com/marktsarkov/test/repo"
)

type service struct {
	repo         repo.Irepo
	clicksCh     chan model.Click
	sendClicksCh chan map[model.Click]int
	mu           *sync.Mutex
}

func NewService(repo repo.Irepo) Iservice {
	clicksCh := make(chan model.Click, 5000)
	sendClicksCh := make(chan map[model.Click]int)
	m := &sync.Mutex{}
	return &service{
		repo:         repo,
		clicksCh:     clicksCh,
		sendClicksCh: sendClicksCh,
		mu:           m,
	}

}

func (s *service) SaveClick(bannerID int) error {
	go func() {
		click := model.Click{}
		click.BannerID = bannerID
		click.Ts = time.Now().Truncate(time.Minute)
		s.clicksCh <- click
	}()
	return nil
}
func (s *service) ParallelSender(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	clicks := make(map[model.Click]int, 100)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				s.send(ctx, clicks)
				clicks = make(map[model.Click]int, 100)
				s.mu.Unlock()
			}
		}
	}()

	go func() {
		for {
			select {
			case click := <-s.clicksCh:
				s.mu.Lock()
				clicks[click]++
				s.mu.Unlock()
			case <-ctx.Done():
				return

			}
		}
	}()
}

func (s *service) send(ctx context.Context, clicks map[model.Click]int) {
	if len(clicks) > 0 {
		err := s.repo.SaveClicks(ctx, clicks)
		if err != nil {
			log.Printf("failed to save clicks in sender: %v\nclicks: %v", err, clicks)
		}
		for click, count := range clicks {
			log.Printf("sent clicks: %v %v", click.BannerID, count)
		}
	} else {
		log.Printf("nothing to send")
	}
	return
}

func (s *service) Close(ctx context.Context) {
	select {
	case <-ctx.Done():
		close(s.clicksCh)
	default:
	}
}

func (s *service) GetStats(ctx context.Context, bannerID int, tsFrom, tsTo time.Time) (data []model.ClickStat, err error) {
	data, err = s.repo.GetStats(ctx, bannerID, tsFrom, tsTo)
	if err != nil {
		return nil, err
	}
	return data, nil
}
