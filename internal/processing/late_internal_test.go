package processing

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

func TestComputeDeadline(t *testing.T) {
	type testCase struct {
		name        string
		fakeTime    time.Time
		expectation time.Time
	}

	cases := []testCase{
		{
			name:        "Before 2PM",
			fakeTime:    time.Date(2024, 12, 25, 13, 59, 59, 0, time.UTC),
			expectation: time.Date(2024, 12, 24, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "After 2PM",
			fakeTime:    time.Date(2024, 12, 25, 14, 0, 1, 0, time.UTC),
			expectation: time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "At midnight",
			fakeTime:    time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC),
			expectation: time.Date(2024, 12, 24, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "Change of year",
			fakeTime:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expectation: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for i := range cases {
		c := cases[i]

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert := assert.New(t)

			clock := clockwork.NewFakeClockAt(c.fakeTime)

			p := CountLateData{
				clock: clock,
			}

			deadline := p.computeDeadline()

			assert.Equal(c.expectation, deadline)
		})
	}
}
