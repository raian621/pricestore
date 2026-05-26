package clients

import shared_models "github.com/raian621/pricestore/src/shared/models"

type HistoricalDataClient interface {
	GetCandlesticks(name string, startUsec, endUsec int64) ([]shared_models.Candlestick, error)
}
