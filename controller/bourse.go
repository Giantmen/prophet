package controller

import (
	"fmt"

	"github.com/Giantmen/prophet/config"
	"github.com/Giantmen/prophet/log"
	"github.com/Giantmen/trader/bourse"
	"github.com/Giantmen/trader/bourse/btc38"
	"github.com/Giantmen/trader/bourse/btctrade"
	"github.com/Giantmen/trader/bourse/bter"
	"github.com/Giantmen/trader/bourse/chbtc"
	"github.com/Giantmen/trader/bourse/jubi"
	"github.com/Giantmen/trader/bourse/yunbi"
)

type MyBourse struct {
	Name      string
	AccessKey string
	SecretKey string
	Timeout   int

	EtcFee float64
	SntFee float64
	EthFee float64
	LtcFee float64
	OmgFee float64
	PayFee float64
	Bourse bourse.Bourse
}

func NewMyBourse(cfg *config.Bourse) (*MyBourse, error) {
	mb := &MyBourse{
		Name:      cfg.Name,
		AccessKey: cfg.AccessKey,
		SecretKey: cfg.SecretKey,
		Timeout:   cfg.Timeout,
		EtcFee:    cfg.EtcFee,
		SntFee:    cfg.SntFee,
		EthFee:    cfg.EtcFee,
		LtcFee:    cfg.LtcFee,
		OmgFee:    cfg.OmgFee,
		PayFee:    cfg.PayFee,
	}
	b, err := mb.selectBourse()
	if err != nil {
		return nil, err
	}
	mb.Bourse = b
	return mb, nil
}

func (mb *MyBourse) selectBourse() (bourse.Bourse, error) {
	switch mb.Name {
	case "Huobi":
		return nil, fmt.Errorf("%s will come soon...", mb.Name)
	case "Yunbi":
		return yunbi.NewYunbi(mb.AccessKey, mb.SecretKey, mb.Timeout)

	case "Chbtc":
		return chbtc.NewChbtc(mb.AccessKey, mb.SecretKey, mb.Timeout)

	case "Btctrade":
		return btctrade.NewBtctrade(mb.AccessKey, mb.SecretKey, mb.Timeout)

	case "Btc38":
		return btc38.NewBtc38(mb.AccessKey, mb.SecretKey, mb.Timeout)

	case "Jubi":
		return jubi.NewJubi(mb.AccessKey, mb.SecretKey, mb.Timeout)

	case "Bter":
		return bter.NewBter(mb.AccessKey, mb.SecretKey, mb.Timeout)

	default:
		return nil, fmt.Errorf("not support bourse:%s", mb.Name)
	}
}

func (mb *MyBourse) Fee(coin string) float64 {
	switch coin {
	case "etc":
		return mb.EtcFee
	case "snt":
		return mb.SntFee
	case "eth":
		return mb.EthFee
	case "ltc":
		return mb.LtcFee
	case "omg":
		return mb.OmgFee
	case "pay":
		return mb.PayFee

	default:
		log.Errorf("not support coin")
		return 0.0
	}
}
