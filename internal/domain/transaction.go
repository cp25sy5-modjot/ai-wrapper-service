package domain

type Transaction struct {
	Date  string            `json:"date"` // normalized as YYYY-MM-DD
	Items []TransactionItem `json:"items"`
}

type TransactionItem struct {
	Title    string  `json:"title"`
	Price    float64 `json:"price"`
	Category string     `json:"category"`
}
