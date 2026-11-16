package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePriority(t *testing.T) {
	cases := []struct {
		in      string
		want    Priority
		wantErr bool
	}{
		{"low", PriorityLow, false},
		{"medium", PriorityMedium, false},
		{"high", PriorityHigh, false},
		{"urgent", PriorityUrgent, false},
		{"HIGH", PriorityHigh, false},
		{"  Urgent ", PriorityUrgent, false},
		{"", "", true},
		{"   ", "", true},
		{"critical", "", true},
		{"super", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParsePriority(tc.in)

			if tc.wantErr {
				assert.ErrorIs(t, err, ErrInvalidPriority)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPriorityLevel(t *testing.T) {
	assert.Equal(t, 1, PriorityLow.Level())
	assert.Equal(t, 2, PriorityMedium.Level())
	assert.Equal(t, 3, PriorityHigh.Level())
	assert.Equal(t, 4, PriorityUrgent.Level())
	assert.Equal(t, 0, Priority("x").Level())
	assert.Less(t, PriorityLow.Level(), PriorityUrgent.Level(), "a ordem numérica não é a alfabética")
}
