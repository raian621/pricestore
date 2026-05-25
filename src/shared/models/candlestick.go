package shared_models

type Candlestick struct {
	StartUsec int64   `json:"start_usec"`
	EndUsec   int64   `json:"end_usec"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}
