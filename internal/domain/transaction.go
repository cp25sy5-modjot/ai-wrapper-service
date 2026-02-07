package domain

type Transaction struct {
	Title string            `json:"title"`
	Date  string            `json:"date"` // normalized as YYYY-MM-DDTHH:MM:SS(+TZ)
	Items []TransactionItem `json:"items"`
}

type TransactionItem struct {
	Title    string  `json:"title"`
	Price    float64 `json:"price"`
	Category string     `json:"category"`
}
