package config

type Config struct {
	Listen string

	Debug    bool
	LogPath  string
	LogLevel string

	Strategies []*Strategy
}

type Strategy struct {
	Name                 string
	AutoAddr             string
	Bourses              []Bourse
	Coin                 string
	Depth                float64
	Interval             int
	DefaultEarn          float64
	LowEarn              float64
	HighEarn             float64
	DepthAdjustThreshold float64
	EarnAdjustThreshold  float64
	InitialPosition      float64
	InitialPositionCNY   float64
	NeedRemedy           bool
}

type Bourse struct {
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
}
