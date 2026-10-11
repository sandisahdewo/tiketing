package money

import (
	"strconv"
	"strings"
)

func Format(sen int64) string {
	sign := ""
	if sen < 0 {
		sign = "-"
		sen = -sen
	}
	rupiah := strconv.FormatInt(sen/100, 10)
	var b strings.Builder
	for i, r := range rupiah {
		if i > 0 && (len(rupiah)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	if cents := sen % 100; cents != 0 {
		b.WriteByte(',')
		if cents < 10 {
			b.WriteByte('0')
		}
		b.WriteString(strconv.FormatInt(cents, 10))
	}
	return sign + b.String()
}
