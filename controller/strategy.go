package controller

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Giantmen/prophet/config"
	"github.com/Giantmen/prophet/log"
	"github.com/Giantmen/trader/proto"
	"github.com/Giantmen/trader/util"
)

type Strategy struct {
	Name        string
	AutoAddr    string
	Bourses     []*MyBourse
	Coin        string
	Depth       float64
	Interval    int
	DefaultEarn float64
	LowEarn     float64
	HighEarn    float64
	// 根据利润调节深度
	DepthAdjustThreshold float64
	// 根据仓位调节利润
	EarnAdjustThreshold float64
	// 初始仓位
	InitialPosition float64
	// 人民币初始仓位
	InitialPositionCNY float64
	NeedRemedy         bool

	lastDepth     float64
	lastAmount    float64
	lastLeftEarn  float64
	lastRightEarn float64

	wg   *sync.WaitGroup
	done chan struct{}
}

func NewStrategy(cfg *config.Strategy, wg *sync.WaitGroup) (*Strategy, error) {
	log.Infof("new strategy:%+v", *cfg)
	s := &Strategy{
		Name:                 cfg.Name,
		AutoAddr:             cfg.AutoAddr,
		Coin:                 cfg.Coin,
		Depth:                cfg.Depth,
		Interval:             cfg.Interval,
		DefaultEarn:          cfg.DefaultEarn,
		LowEarn:              cfg.LowEarn,
		HighEarn:             cfg.HighEarn,
		DepthAdjustThreshold: cfg.DepthAdjustThreshold,
		EarnAdjustThreshold:  cfg.EarnAdjustThreshold,
		InitialPosition:      cfg.InitialPosition,
		InitialPositionCNY:   cfg.InitialPositionCNY,
		NeedRemedy:           cfg.NeedRemedy,
		wg:                   wg,
		done:                 make(chan struct{}),
	}
	s.Bourses = make([]*MyBourse, 0)
	log.Debug(len(cfg.Bourses))
	for _, b := range cfg.Bourses {
		bo, err := NewMyBourse(&b)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		s.Bourses = append(s.Bourses, bo)
	}
	return s, nil
}

func (s *Strategy) Run() {
	log.Debugf("%s run:%+v", s.Name, *s)
	if len(s.Bourses) != 2 {
		log.Fatal("error bourse amount in strategy %s", s.Name)
	}

	bourseA := s.Bourses[0]
	bourseB := s.Bourses[1]
	if s.NeedRemedy {
		go s.Remedy(bourseA, bourseB)
	}

	go s.AdjustDepth(bourseA, bourseB)
	//go s.AdjustEarn(bourseA, bourseB)

	<-s.done
}

func (s *Strategy) Remedy(bourseA, bourseB *MyBourse) {
	ticker := time.NewTicker(time.Duration(s.Interval) * time.Second)
	for {
		select {
		case <-s.done:
			log.Infof("strategy %s remedy done!", s.Name)
			return
		case <-ticker.C:
			s.remedyOrder(bourseA, bourseB)
		}
	}
}

// RemedyOrder自动补单
func (s *Strategy) remedyOrder(bourseA, bourseB *MyBourse) {
	var delta float64
	var remedy bool
	var err error
	for {
		delta, err = s.getPositiondelta(bourseA, bourseB)
		if err != nil {
			log.Errorf("get position delta err,%s", err.Error())
			return
		}
		if math.Abs(delta) < 1.0 {
			return
		}
		if remedy {
			break
		}
		remedy = true
		time.Sleep(time.Second * 3)
	}
	priceA, err := bourseA.Bourse.GetPriceOfDepth(20, s.Depth, s.Coin+"_cny")
	if err != nil {
		log.Errorf("%s GetPriceOfDepth error,%s", bourseA.Name, err.Error())
		return
	}
	priceB, err := bourseB.Bourse.GetPriceOfDepth(20, s.Depth, s.Coin+"_cny")
	if err != nil {
		log.Errorf("%s GetPriceOfDepth error,%s", bourseB.Name, err.Error())
		return
	}

	accountA, err := bourseA.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s GetAccount error,%s", bourseA.Name, err.Error())
		return
	}
	// accountB, err := bourseB.Bourse.GetAccount()
	// if err != nil {
	// 	log.Errorf("%s GetAccount error,%s", bourseB.Name, err.Error())
	// 	return
	// }

	cnyA := accountA.SubAccounts[proto.CNY].Available
	coinA := accountA.SubAccounts[s.Coin].Available
	//cnyB := accountB.SubAccounts[proto.CNY].Available
	//log.Debugf("%s cny:%f,%s cny:%f", bourseA.Name, cnyA, bourseB.Name, cnyB)

	if delta < 0 { //买
		// A买,当A便宜且A余额足
		if priceA.Sell <= priceB.Sell && cnyA > math.Abs(delta)*priceA.Sell*1.1 {
			pricef := priceA.Sell * 1.5
			amountf := math.Abs(delta)
			if bourseA.Name == proto.Bter {
				amountf = amountf / 1.5
			}

			price := fmt.Sprintf("%f", pricef)
			amount := fmt.Sprintf("%f", amountf)
			_, err := bourseA.Bourse.Buy(amount, price, s.Coin+"_cny")
			if err != nil {
				log.Errorf("%s buy error,%s", bourseA.Name, err.Error())
				return
			}
			log.Infof("%s buy:%s %s,price:%s", bourseA.Name, amount, s.Coin, price)
		} else {
			pricef := priceB.Sell * 1.5
			amountf := math.Abs(delta)
			if bourseB.Name == proto.Bter {
				amountf = amountf / 1.5
			}

			price := fmt.Sprintf("%f", pricef)
			amount := fmt.Sprintf("%f", amountf)
			_, err := bourseB.Bourse.Buy(amount, price, s.Coin+"_cny")
			if err != nil {
				log.Errorf("%s buy error,%s", bourseB.Name, err.Error())
				return
			}
			log.Infof("%s buy:%s %s,price:%s", bourseB.Name, amount, s.Coin, price)
		}
	} else if delta > 0 { //卖
		if priceA.Buy >= priceB.Buy && coinA > math.Abs(delta) {
			pricef := priceA.Buy * 0.5
			amountf := math.Abs(delta)

			price := fmt.Sprintf("%f", pricef)
			amount := fmt.Sprintf("%f", amountf)
			_, err := bourseA.Bourse.Sell(amount, price, s.Coin+"_cny")
			if err != nil {
				log.Errorf("%s sell error,%s", bourseA.Name, err.Error())
				return
			}
			log.Infof("%s sell:%s %s,price:%s", bourseB.Name, amount, s.Coin, price)
		} else {
			pricef := priceB.Buy * 0.5
			amountf := math.Abs(delta)

			price := fmt.Sprintf("%f", pricef)
			amount := fmt.Sprintf("%f", amountf)

			_, err := bourseB.Bourse.Sell(amount, price, s.Coin+"_cny")
			if err != nil {
				log.Errorf("%s sell error,%s", bourseB.Name, err.Error())
				return
			}
			log.Infof("%s sell:%s %s,price:%s", bourseB.Name, amount, s.Coin, price)
		}
		//卖
	}
}

func (s *Strategy) getPositiondelta(bourseA, bourseB *MyBourse) (float64, error) {
	accountA, err := bourseA.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s get account error:%s", bourseA.Name, err.Error())
		return 0.0, err
	}

	accountB, err := bourseB.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s get account error:%s", bourseB.Name, err.Error())
		return 0.0, err
	}
	positionA := accountA.SubAccounts[s.Coin].Available
	positionB := accountB.SubAccounts[s.Coin].Available

	log.Debugf("%+v", accountA)
	log.Debugf("%+v", accountB)
	log.Info(s.Name, " position delta:", positionA+positionB-s.InitialPosition)
	return positionA + positionB - s.InitialPosition, nil
}

func (s *Strategy) AdjustDepth(bourseA, bourseB *MyBourse) {
	ticker := time.NewTicker(time.Duration(s.Interval) * time.Second)
	for {
		select {
		case <-s.done:
			log.Infof("strategy %s adjust depth done!", s.Name)
			return
		case <-ticker.C:
			s.adjustDepth(bourseA, bourseB)
		}
	}
}

func (s *Strategy) AdjustEarn(bourseA, bourseB *MyBourse) {
	ticker := time.NewTicker(time.Duration(s.Interval) * time.Second)
	for {
		select {
		case <-s.done:
			log.Infof("strategy %s adjust earn done!", s.Name)
			return
		case <-ticker.C:
			s.adjustEarn(bourseA, bourseB)
		}
	}
}

func (s *Strategy) adjustDepth(bourseA, bourseB *MyBourse) {
	priceA, err := bourseA.Bourse.GetPriceOfDepth(20, s.Depth, s.Coin+"_cny")
	if err != nil {
		log.Errorf("%s GetPriceofDepth error,%s", bourseA.Name, err.Error())
		return
	}
	priceB, err := bourseB.Bourse.GetPriceOfDepth(20, s.Depth, s.Coin+"_cny")
	if err != nil {
		log.Errorf("%s GetPriceofDepth error,%s", bourseB.Name, err.Error())
		return
	}

	gapBtoA := priceA.Buy*(1-bourseA.Fee(s.Coin)) - priceB.Sell*(1+bourseB.Fee(s.Coin))
	gapAtoB := priceB.Buy*(1-bourseB.Fee(s.Coin)) - priceA.Sell*(1+bourseA.Fee(s.Coin))

	var depth float64
	if math.Max(gapBtoA, gapAtoB) > s.DepthAdjustThreshold { //每个币利润大于DepthAdjustThreshold时
		if gapBtoA >= gapAtoB {
			depth = math.Min(priceA.Sellnum, priceB.Buynum)
		} else {
			depth = math.Min(priceA.Buynum, priceB.Sellnum)
		}
		depth = math.Ceil(depth)
		if depth != s.lastDepth {
			log.Infof("%s profit is %f > %f,adjust depth:%f", s.Name, math.Max(gapBtoA, gapAtoB), s.DepthAdjustThreshold, depth)
			if err = s.setDepth(depth); err != nil {
				log.Errorf("%s set depth error,%s", s.Name, err.Error())
				return
			}
			s.lastDepth = depth
			allowedAmount, err := s.getAllowableAmount(priceA, priceB, bourseA, bourseB)
			if err != nil {
				log.Errorf("%s get alloedAmount error,%s", s.Name, err.Error())
				return
			}
			amount := math.Ceil(math.Min(depth*0.8, allowedAmount))
			if err = s.setTradeAmount(amount); err != nil {
				log.Errorf("%s set amount error,%s", s.Name, err.Error())
				return
			}
			log.Infof("%s allowable amount:%f, set amount:%f", s.Name, allowedAmount, amount)
			s.lastAmount = amount
		}

	} else {
		if s.Depth != s.lastDepth {
			log.Infof("%s profit is %f < %f,default depth:%f", s.Name, math.Max(gapBtoA, gapAtoB), s.DepthAdjustThreshold, s.Depth)
			if err = s.setDepth(s.Depth); err != nil {
				log.Errorf("%s set depth error,%s", s.Name, err.Error())
				return
			}
			s.lastDepth = s.Depth
			allowedAmount, err := s.getAllowableAmount(priceA, priceB, bourseA, bourseB)
			if err != nil {
				log.Errorf("%s get allowableAmount error,%s", s.Name, err.Error())
				return
			}
			amount := math.Ceil(math.Min(s.Depth/4, allowedAmount))
			if err = s.setTradeAmount(amount); err != nil {
				log.Errorf("%s set amount error,%s", s.Name, err.Error())
				return
			}
			log.Infof("%s allowable amount:%f, set amount:%f", s.Name, allowedAmount, amount)
			s.lastAmount = amount
		}
	}
}

func (s *Strategy) getAllowableAmount(priceA, priceB *proto.Price, bourseA, bourseB *MyBourse) (float64, error) {
	accountA, err := bourseA.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s GetAccount error,%s", bourseA.Name, err.Error())
		return 0.0, nil
	}
	accountB, err := bourseB.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s GetAccount error,%s", bourseB.Name, err.Error())
		return 0.0, err
	}

	cnyA := accountA.SubAccounts[proto.CNY].Available
	cnyB := accountB.SubAccounts[proto.CNY].Available
	log.Debugf("%s cny:%f,%s cny:%f", bourseA.Name, cnyA, bourseB.Name, cnyB)

	amount := math.Min(cnyA/priceA.Sell, cnyB/priceB.Sell)
	return amount, nil
}

// adjustEarn adjust earn,bourseA is left,bourseB is right
func (s *Strategy) adjustEarn(bourseA, bourseB *MyBourse) {
	accountA, err := bourseA.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s GetAccount error,%s", bourseA.Name, err.Error())
		return
	}
	accountB, err := bourseB.Bourse.GetAccount()
	if err != nil {
		log.Errorf("%s GetAccount error,%s", bourseB.Name, err.Error())
		return
	}

	balanceA := accountA.SubAccounts[s.Coin].Available
	balanceB := accountB.SubAccounts[s.Coin].Available

	// 当某个账户仓位低于EarnAdjustThreshold时，调整搬砖的利润阈值
	minbalance := math.Min(balanceA, balanceB)
	if minbalance < s.EarnAdjustThreshold {
		if balanceA < balanceB { //A仓位低,B->A,往左搬
			s.setLeftEarn(0.1) //降低往左搬的门槛,0.1需要从配置读
			s.setRightEarn(0.8)
		} else {
			s.setRightEarn(0.1)
			s.setLeftEarn(0.8)
		}
	} else {
		s.setLeftEarn(s.DefaultEarn)
		s.setRightEarn(s.DefaultEarn)
	}

	/* TODO
	当人民币不足时的调整
	*/

}

func (s *Strategy) setRightEarn(earn float64) error {
	log.Debug("setRightEarn:", earn)
	v := url.Values{}
	d := fmt.Sprintf("%f", earn)
	v.Add("judge", s.Name)
	v.Add("value", d)
	body := strings.NewReader(v.Encode())
	url := fmt.Sprintf("%s%s", s.AutoAddr, "judge/SetRightEarn")
	resp, err := util.Request("POST", url, "application/x-www-form-urlencoded", body, nil, 1)
	if err != nil {
		return err
	}
	log.Debug(string(resp))
	return nil
}

func (s *Strategy) setLeftEarn(earn float64) error {
	log.Debug("setLeftEarn:", earn)
	v := url.Values{}
	d := fmt.Sprintf("%f", earn)
	v.Add("judge", s.Name)
	v.Add("value", d)
	body := strings.NewReader(v.Encode())
	url := fmt.Sprintf("%s%s", s.AutoAddr, "judge/SetLeftEarn")
	resp, err := util.Request("POST", url, "application/x-www-form-urlencoded", body, nil, 1)
	if err != nil {
		return err
	}
	log.Debug(string(resp))
	return nil
}

func (s *Strategy) setDepth(depth float64) error {
	log.Debug("setDepth:", depth)
	v := url.Values{}
	d := fmt.Sprintf("%f", depth)
	v.Add("judge", s.Name)
	v.Add("value", d)
	body := strings.NewReader(v.Encode())
	url := fmt.Sprintf("%s%s", s.AutoAddr, "judge/SetDepth")
	resp, err := util.Request("POST", url, "application/x-www-form-urlencoded", body, nil, 1)
	if err != nil {
		return err
	}
	log.Debug(string(resp))
	return nil
}

func (s *Strategy) setTradeAmount(amount float64) error {
	log.Debug("setTradeAmount:", amount)
	v := url.Values{}
	d := fmt.Sprintf("%f", amount)
	v.Add("judge", s.Name)
	v.Add("value", d)
	body := strings.NewReader(v.Encode())
	url := fmt.Sprintf("%s%s", s.AutoAddr, "judge/SetAmount")
	resp, err := util.Request("POST", url, "application/x-www-form-urlencoded", body, nil, 1)
	if err != nil {
		return err
	}
	log.Debug(string(resp))
	return nil
}

func (s *Strategy) Done() {
	//s.done <- struct{}{}
	close(s.done)
}
