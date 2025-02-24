package models

import "encoding/json"

func UnmarshalWelcome(data []byte) (Welcome, error) {
	var r Welcome
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Welcome) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Welcome struct {
	Raw     map[string]map[string]Raw     `json:"RAW"`
	Display map[string]map[string]Display `json:"DISPLAY"`
}

type Raw struct {
	PRICE           float64 `json:"PRICE"`
	CHANGE24HOUR    float64 `json:"CHANGE24HOUR"`
	CHANGEPCT24HOUR float64 `json:"CHANGEPCT24HOUR"`
}

type Display struct {
	PRICE        string `json:"PRICE"`
	LASTUPDATE   string `json:"LASTUPDATE"`
	CHANGE24HOUR string `json:"CHANGE24HOUR"`
}

type Instrument struct {
	Type                string  `json:"TYPE"`
	Market              string  `json:"MARKET"`
	Value               float64 `json:"VALUE"`
	CurrentDayChange    float64 `json:"CURRENT_DAY_CHANGE"`
	CurrentDayChangePct float64 `json:"CURRENT_DAY_CHANGE_PERCENTAGE"`
}

type Err struct{}
