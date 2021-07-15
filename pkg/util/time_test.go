package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrettyDuration(t *testing.T) {
	fixtures := []struct {
		d        time.Duration
		atMost   int
		expected string
	}{
		{
			3*time.Hour + time.Minute + time.Second + time.Nanosecond,
			2,
			"3h1m",
		},
		{
			-time.Minute - time.Second - time.Millisecond,
			2,
			"-1m1s",
		},
	}

	for _, f := range fixtures {
		fixture := f
		t.Run(fixture.d.String(), func(t *testing.T) {
			actual := PrettyDuration(fixture.d, fixture.atMost)
			require.Equal(t, fixture.expected, actual)
		})
	}
}
