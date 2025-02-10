package processing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openshift-assisted/ccx-exporter/internal/processing"
)

func TestValidateDate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		date     string
		valid    bool
		expected time.Time
	}

	cases := []testCase{
		{
			name:     "happy path",
			date:     "2024-11-21T02:57:38.485Z",
			valid:    true,
			expected: time.Date(2024, 11, 21, 2, 57, 38, 485000000, time.UTC),
		},
		{
			name: "missing Z",
			date: "2024-11-21T02:57:38.485",
		},
		{
			name: "another format",
			date: "02 Jan 06 15:04 MST",
		},
		{
			name: "invalid year",
			date: "224-11-21T02:57:38.485Z",
		},
		{
			name: "invalid month",
			date: "2024-13-21T02:57:38.485Z",
		},
		{
			name: "invalid day",
			date: "2024-02-31T02:57:38.485Z",
		},
		{
			name:     "2 digits for fractional second",
			date:     "2024-11-21T02:57:38.48Z",
			valid:    true,
			expected: time.Date(2024, 11, 21, 2, 57, 38, 480000000, time.UTC),
		},
		{
			name:     "5 digits for fractional second",
			date:     "2024-11-21T02:57:38.48123Z",
			valid:    true,
			expected: time.Date(2024, 11, 21, 2, 57, 38, 481230000, time.UTC),
		},
	}

	for i := range cases {
		c := cases[i]

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ts, err := processing.ValidateDate(c.date)
			assert.Equal(t, c.valid, err == nil, err)

			if c.valid {
				assert.Equal(t, c.expected, ts)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		date     time.Time
		expected string
	}

	cases := []testCase{
		{
			name:     "happy path",
			date:     time.Date(2024, 11, 21, 2, 57, 38, 485101000, time.UTC),
			expected: "2024-11-21T02:57:38.485101Z",
		},
		{
			name:     "2 digits for fractional seconds",
			date:     time.Date(2024, 11, 21, 2, 57, 38, 480000000, time.UTC),
			expected: "2024-11-21T02:57:38.480000Z",
		},
		{
			name:     "2 digits for fractional seconds, starting with 0",
			date:     time.Date(2024, 11, 21, 2, 57, 38, 4800000, time.UTC),
			expected: "2024-11-21T02:57:38.004800Z",
		},
		{
			name:     "0 digits for fractional seconds",
			date:     time.Date(2024, 11, 21, 2, 57, 38, 0o00000000, time.UTC),
			expected: "2024-11-21T02:57:38.000000Z",
		},
		{
			name:     "7 digits for fractional seconds",
			date:     time.Date(2024, 11, 21, 2, 57, 38, 485874300, time.UTC),
			expected: "2024-11-21T02:57:38.485874Z",
		},
	}

	for i := range cases {
		c := cases[i]

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			dateStr := processing.FormatDate(c.date)
			assert.Equal(t, c.expected, dateStr)
		})
	}
}
