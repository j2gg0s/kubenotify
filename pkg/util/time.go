package util

import "time"

const (
	byteh = 104
	bytem = 109
	byten = 110
	bytes = 115
	byteu = 117
)

func PrettyDuration(d time.Duration, atMost int) string {
	if d >= time.Second || d <= -1*time.Second {
		d = d.Truncate(time.Second)
	}

	s := d.String()
	ind := 0
	// h, m, s, ms, us, ns
	for ind < len(s) && atMost > 0 {
		switch s[ind] {
		case byteh, bytes:
			ind += 1
			atMost -= 1
		case bytem:
			ind += 1
			if ind < len(s) && s[ind] == bytes {
				ind += 1
			}
			atMost -= 1
		case byteu, byten:
			ind += 2
			atMost -= 1
		default:
			ind += 1
		}
	}

	return s[:ind]
}
