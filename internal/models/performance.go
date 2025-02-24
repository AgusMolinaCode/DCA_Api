package models

type Performance struct {
	TopGainer PerformanceDetail `json:"top_gainer"`
	TopLoser  PerformanceDetail `json:"top_loser"`
}

type PerformanceDetail struct {
	Ticker       string  `json:"ticker"`
	ChangePct24h float64 `json:"change_percent_24h"`
	PriceChange  float64 `json:"price_change"`
}
