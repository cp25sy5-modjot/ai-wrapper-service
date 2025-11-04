package domain

type Transaction struct {
	Title    string
	Price    float64
	Quantity float64
	Date     string // normalized as YYYY-MM-DD
	Category string
}
