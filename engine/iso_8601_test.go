package engine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestISO8601Duration(t *testing.T) {
	assert := assert.New(t)

	v := time.Date(2023, 12, 24, 13, 14, 15, 0, time.UTC)

	tests := map[string]time.Time{
		"": time.Date(2023, 12, 24, 13, 14, 15, 0, time.UTC),

		"P1Y": time.Date(2024, 12, 24, 13, 14, 15, 0, time.UTC),
		"P1M": time.Date(2024, 1, 24, 13, 14, 15, 0, time.UTC),
		"P1W": time.Date(2023, 12, 31, 13, 14, 15, 0, time.UTC),
		"P1D": time.Date(2023, 12, 25, 13, 14, 15, 0, time.UTC),

		"PT1H": time.Date(2023, 12, 24, 14, 14, 15, 0, time.UTC),
		"PT1M": time.Date(2023, 12, 24, 13, 15, 15, 0, time.UTC),
		"PT1S": time.Date(2023, 12, 24, 13, 14, 16, 0, time.UTC),

		"P1Y1M1W1D":        time.Date(2025, 2, 1, 13, 14, 15, 0, time.UTC),
		"P1Y1M1W1DT1H1M1S": time.Date(2025, 2, 1, 14, 15, 16, 0, time.UTC),
		"PT1H1M1S":         time.Date(2023, 12, 24, 14, 15, 16, 0, time.UTC),

		"P":     {},
		"PT":    {},
		"P1T":   {},
		"PDT":   {},
		"PT1":   {},
		"PTS":   {},
		"P1DT":  {},
		"P1DT1": {},
		"P1DTS": {},
		"T":     {},
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			d, err := NewISO8601Duration(input)
			if expected.IsZero() {
				assert.Error(err)
			} else {
				assert.Equal(expected, d.Calculate(v))
			}
		})
	}
}

func TestUnmarshalISO8601Duration(t *testing.T) {
	assert := assert.New(t)

	zero := ISO8601Duration("")
	invalid := ISO8601Duration("P")
	valid := ISO8601Duration("P1D")

	tests := map[string]struct {
		json     string
		expected *ISO8601Duration
	}{
		"null": {
			json:     "null",
			expected: &zero,
		},
		"empty": {
			json:     `""`,
			expected: &zero,
		},
		"number": {
			json:     "1",
			expected: nil,
		},
		"invalid": {
			json:     `"P"`,
			expected: &invalid,
		},
		"valid": {
			json:     `"P1D"`,
			expected: &valid,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var actual ISO8601Duration
			err := json.Unmarshal([]byte(test.json), &actual)

			if test.expected != nil {
				assert.Equal(test.expected, &actual)
			} else {
				assert.Error(err)
			}
		})
	}
}
