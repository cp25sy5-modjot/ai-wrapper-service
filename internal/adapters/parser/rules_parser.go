package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cp25sy5-modjot/ai-wrapper-service/internal/domain"
)

type RulesParser struct{}

func NewRulesParser() *RulesParser { return &RulesParser{} }

var (
	priceRe = regexp.MustCompile(`(?i)(?:฿|\$)?\s*([0-9]+(?:\.[0-9]{1,2})?)`)
	qtyRe   = regexp.MustCompile(`(?i)\b(?:qty|quantity|x)\s*[:\-]?\s*([0-9]+)\b`)
	dateRes = []*regexp.Regexp{
		regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`),             // YYYY-MM-DD
		regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`),             // DD/MM/YYYY
		regexp.MustCompile(`\b(\d{2})-(\d{2})-(\d{4})\b`),             // DD-MM-YYYY
		regexp.MustCompile(`\b(\d{1,2})\s+([A-Za-z]{3,})\s+(\d{4})\b`), // 1 Jan 2025
	}
)

func (p *RulesParser) Parse(text string, categories []string) domain.Transaction {
	text = normalize(text)
	title := guessTitle(text)
	price := guessPrice(text)
	qty := guessQty(text)
	if qty <= 0 {
		qty = 1
	}
	date := guessDate(text)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	cat := guessCategory(text, categories)
	return domain.Transaction{
		Title:    title,
		Price:    price,
		Quantity: qty,
		Date:     date,
		Category: cat,
	}
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func guessTitle(txt string) string {
	lines := nonEmptyLines(txt)
	for _, l := range lines {
		ll := strings.ToLower(l)
		if strings.Contains(ll, "total") || strings.Contains(ll, "ยอดรวม") {
			continue
		}
		if hasLetters(l) {
			return truncate(l, 64)
		}
	}
	words := strings.Fields(txt)
	if len(words) == 0 {
		return "Unknown"
	}
	return truncate(strings.Join(words[:min(3, len(words))], " "), 64)
}

func guessPrice(txt string) float64 {
	if m := priceRe.FindStringSubmatch(txt); len(m) >= 2 {
		v, _ := parseFloat(m[1])
		return v
	}
	return 0
}

func guessQty(txt string) float64 {
	if m := qtyRe.FindStringSubmatch(txt); len(m) >= 2 {
		v, _ := parseFloat(m[1])
		return v
	}
	if m := regexp.MustCompile(`(?i)\bx([0-9]+)\b`).FindStringSubmatch(txt); len(m) >= 2 {
		v, _ := parseFloat(m[1])
		return v
	}
	return 0
}

func guessDate(txt string) string {
	for _, r := range dateRes {
		if m := r.FindStringSubmatch(txt); len(m) > 0 {
			switch len(m) {
			case 4: // DD/MM/YYYY or DD-MM-YYYY
				return fmt.Sprintf("%s-%s-%s", m[3], pad2(m[2]), pad2(m[1]))
			case 3: // YYYY-MM-DD (first regex)
				return fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
			}
		}
	}
	return ""
}

func guessCategory(txt string, categories []string) string {
	l := strings.ToLower(txt)
	for _, c := range categories {
		if c == "" {
			continue
		}
		if strings.Contains(l, strings.ToLower(c)) {
			return c
		}
	}
	if len(categories) > 0 && categories[0] != "" {
		return categories[0]
	}
	return "Uncategorized"
}

func nonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		t := strings.TrimSpace(r)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func hasLetters(s string) bool {
	for _, r := range s {
		if ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z') || (0x0E00 <= r && r <= 0x0E7F) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n])
}

func pad2(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

func monthFromName(m string) string {
	m = strings.ToLower(m)
	mp := map[string]string{
		"jan": "01", "january": "01",
		"feb": "02", "february": "02",
		"mar": "03", "march": "03",
		"apr": "04", "april": "04",
		"may": "05",
		"jun": "06", "june": "06",
		"jul": "07", "july": "07",
		"aug": "08", "august": "08",
		"sep": "09", "sept": "09", "september": "09",
		"oct": "10", "october": "10",
		"nov": "11", "november": "11",
		"dec": "12", "december": "12",
	}
	for k, v := range mp {
		if strings.HasPrefix(m, k) {
			return v
		}
	}
	return ""
}

// strict & locale-agnostic parse
func parseFloat(s string) (float64, error) {
	var intPart, fracPart string
	seenDot := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			if seenDot { fracPart += string(r) } else { intPart += string(r) }
		case r == '.' && !seenDot:
			seenDot = true
		default:
			return 0, fmt.Errorf("invalid float")
		}
	}
	if intPart == "" { intPart = "0" }
	ip := 0
	for _, r := range intPart { ip = ip*10 + int(r-'0') }
	if fracPart == "" { return float64(ip), nil }
	fp := 0
	for _, r := range fracPart { fp = fp*10 + int(r-'0') }
	pow := 1
	for i := 0; i < len(fracPart); i++ { pow *= 10 }
	return float64(ip) + float64(fp)/float64(pow), nil
}
