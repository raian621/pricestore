package clients

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	secret "github.com/andrewbenton/go-secrets"
	shared_models "github.com/raian621/pricestore/src/shared/models"
)

type AlpacaClient struct {
	apiKey    string
	apiSecret string
	baseUrl   string
	client    *http.Client
}

func NewAlpacaClient(apiKey secret.Secret[string],
	apiSecret secret.Secret[string], baseUrl string,
) *AlpacaClient {
	return &AlpacaClient{
		apiKey:    apiKey.Get(),
		apiSecret: apiSecret.Get(),
		baseUrl:   baseUrl,
		client:    &http.Client{},
	}
}

type getHistoricalBarsRequest struct {
	// Comma separatedd list of stock symbols
	Symbols string `json:"symbols"`
	// Timeframe represented by each bar
	Timeframe string `json:"timeframe"`
	// Start time in RFC-3339 format
	Start string `json:"start"`
	// End time in RFC-3339 format
	End string `json:"end"`
	// Max number of bars to return
	Limit int `json:"limit"`
	// Continuation pagination token
	PageToken string `json:"page_token"`
}

type historicalBar struct {
	// RFC-3339 format opening timestamp
	Timestamp                  string  `json:"t"`
	Open                       float64 `json:"o"`
	High                       float64 `json:"h"`
	Low                        float64 `json:"l"`
	Close                      float64 `json:"c"`
	Volume                     int64   `json:"v"`
	TradeCount                 int64   `json:"n"`
	VolumeWeightedAveragePrice float64 `json:"vw"`
}

type getHistoricalBarsResponse struct {
	Bars          map[string][]historicalBar `json:"bars"`
	NextPageToken string                     `json:"next_page_token"`
}

// See https://docs.alpaca.markets/us/reference/stockbars
func (c *AlpacaClient) GetCandlesticks(name string, startUsec, endUsec int64) ([]shared_models.Candlestick, error) {
	req := getHistoricalBarsRequest{
		Symbols:   name,
		Timeframe: "1D",
		Start:     time.UnixMicro(startUsec).Format(time.RFC3339),
		End:       time.UnixMicro(endUsec).Format(time.RFC3339),
		Limit:     100,
		PageToken: "",
	}

	// Paginate through the candlesticks returned by the Alpaca API:
	totalCandlesticks := make([]shared_models.Candlestick, 0)
	for {
		if candlesticks, pageToken, err := c.getCandlesticks(&req); err != nil {
			return nil, err
		} else {
			totalCandlesticks = append(totalCandlesticks, candlesticks...)
			req.PageToken = pageToken
		}

		// If the page token is "" then we've reached the end of the paginated
		// results
		if req.PageToken != "" {
			break
		}
	}

	return totalCandlesticks, nil
}

func (c *AlpacaClient) getCandlesticks(
	req *getHistoricalBarsRequest,
) ([]shared_models.Candlestick, string, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}
	httpReq, err := c.newRequest("GET", "/stocks/bars", bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", err
	}

	res, err := c.sendRequest(httpReq, 3)
	if err != nil {
		return nil, "", err
	}

	var barResponse getHistoricalBarsResponse
	if err := json.NewDecoder(res.Body).Decode(&barResponse); err != nil {
		return nil, "", err
	}

	bars := barResponse.Bars[req.Symbols]
	candlesticks := make([]shared_models.Candlestick, len(bars))
	for i, bar := range bars {
		t, err := time.Parse(time.RFC3339, bar.Timestamp)
		if err != nil {
			return nil, "", err
		}
		candlestick := &candlesticks[i]
		candlestick.StartUsec = t.UnixMicro()
		candlestick.EndUsec = t.Add(time.Hour * 24).UnixMicro()
		candlestick.Open = bar.Open
		candlestick.High = bar.High
		candlestick.Low = bar.Low
		candlestick.Close = bar.Close
		candlestick.Volume = float64(bar.Volume)
	}

	return []shared_models.Candlestick{}, barResponse.NextPageToken, nil
}

func (c *AlpacaClient) sendRequest(req *http.Request, retries int) (res *http.Response, err error) {
	for i := 0; i < retries; retries++ {
		res, err = c.client.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			time.Sleep(time.Millisecond * 50)
			continue
		}
		return res, nil
	}
	return nil, err
}

func (c *AlpacaClient) newRequest(method, urlPath string, bodyReader io.Reader) (*http.Request, error) {
	methodUrl, err := url.JoinPath(c.baseUrl, "/stocks/bars")
	if err != nil {
		return nil, err
	}
	if req, err := http.NewRequest(method, methodUrl, bodyReader); err != nil {
		return nil, err
	} else {
		c.setAuthenticationHeaders(req)
		return req, nil
	}
}

func (c *AlpacaClient) setAuthenticationHeaders(req *http.Request) {
	req.Header.Add("ACPA-API-KEY-ID", c.apiKey)
	req.Header.Add("ACPA-API-SECRET-KEY", c.apiSecret)
}
