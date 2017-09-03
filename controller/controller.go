package controller

import (
	"sync"

	"github.com/Giantmen/prophet/config"
	"github.com/Giantmen/prophet/log"
)

type Controller struct {
	wg         *sync.WaitGroup
	done       chan struct{}
	strategies []*Strategy
}

func NewController(cfg *config.Config, ch chan struct{}) (*Controller, error) {
	stras := make([]*Strategy, 0)
	wg := new(sync.WaitGroup)
	for _, stra := range cfg.Strategies {
		s, _ := NewStrategy(stra, wg)
		stras = append(stras, s)
	}
	return &Controller{
		wg:         wg,
		done:       ch,
		strategies: stras,
	}, nil
}

func (c *Controller) Run() {
	log.Info("controller run")
	if len(c.strategies) == 0 {
		log.Warning("no strategy")
		return
	}
	for _, s := range c.strategies {
		c.wg.Add(1)
		go s.Run()
	}
	for {
		select {
		case <-c.done:
			for _, s := range c.strategies {
				s.Done()
			}
			c.wg.Wait()
			log.Info("controller done")
			return
		}
	}
}
