package domain

type Transaction struct {
	Title    string  `json:"title"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Date     string  `json:"date"` // normalized as YYYY-MM-DD
	Category string  `json:"category"`
}
