package models

type Holdings struct {
	MainHoldings    []HoldingDetail `json:"main_holdings"`
	OtherAssets     []HoldingDetail `json:"other_assets"`
	TotalValue      float64         `json:"total_value"`
	OtherPercentage float64         `json:"other_percentage"`
}

type HoldingDetail struct {
	Ticker     string  `json:"ticker"`
	Value      float64 `json:"value"`      // CurrentPrice * Amount
	Percentage float64 `json:"percentage"` // (Value / TotalValue) * 100
} 