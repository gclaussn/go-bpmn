package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalPartition(t *testing.T) {
	assert := assert.New(t)

	valid, _ := NewPartition("2025-02-01")

	tests := map[string]struct {
		json     string
		expected *Partition
	}{
		"null": {
			json:     "null",
			expected: &Partition{},
		},
		"empty": {
			json:     `""`,
			expected: nil,
		},
		"number": {
			json:     "1",
			expected: nil,
		},
		"too long": {
			json:     "2025-02-01 ",
			expected: nil,
		},
		"too short": {
			json:     "2025",
			expected: nil,
		},
		"invalid": {
			json:     `"2025-02-XX"`,
			expected: nil,
		},
		"valid": {
			json:     `"2025-02-01"`,
			expected: &valid,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var actual Partition
			err := json.Unmarshal([]byte(test.json), &actual)

			if test.expected != nil {
				assert.Equal(test.expected, &actual)
			} else {
				assert.Error(err)
			}
		})
	}
}
