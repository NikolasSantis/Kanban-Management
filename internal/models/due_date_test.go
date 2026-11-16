package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var fixedNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func TestParseDueDate(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"RFC 3339 em UTC", "2026-12-01T18:00:00Z", "2026-12-01T18:00:00Z", nil},
		{"RFC 3339 com fuso vira UTC", "2026-12-01T10:00:00-03:00", "2026-12-01T13:00:00Z", nil},
		{"só a data vira fim do dia em UTC", "2026-12-01", "2026-12-01T23:59:59Z", nil},
		{"espaços nas pontas", "  2026-12-01  ", "2026-12-01T23:59:59Z", nil},
		{"data no passado é permitida", "2025-01-15", "2025-01-15T23:59:59Z", nil},
		{"texto qualquer", "amanhã", "", ErrInvalidDate},
		{"vazio", "", "", ErrInvalidDate},
		{"formato errado", "01/12/2026", "", ErrInvalidDate},
		{"dia inexistente", "2026-02-30", "", ErrInvalidDate},
		{"antes de 2000", "1990-05-01", "", ErrDueDateOutOfRange},
		{"mais de 10 anos no futuro", "2036-11-17", "", ErrDueDateOutOfRange},
		{"mesmo dia do limite, mas depois do horário atual", "2036-11-16", "2036-11-16T23:59:59Z", ErrDueDateOutOfRange},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDueDateAt(tc.in, fixedNow)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.Format(time.RFC3339))
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

func TestParseDate_HasNoRangeLimit(t *testing.T) {
	got, err := ParseDate("1990-05-01")

	assert.NoError(t, err)
	assert.Equal(t, 1990, got.Year())
}

func TestToCardResponse_IsOverdue(t *testing.T) {
	past := fixedNow.Add(-24 * time.Hour)
	future := fixedNow.Add(24 * time.Hour)

	cases := []struct {
		name string
		card Card
		want bool
	}{
		{"sem data", Card{}, false},
		{"data futura", Card{DueDate: &future}, false},
		{"data passada e não concluído", Card{DueDate: &past}, true},
		{"data passada mas concluído", Card{DueDate: &past, CompletedAt: &fixedNow}, false},
		{"exatamente agora não é atraso", Card{DueDate: &fixedNow}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ToCardResponse(tc.card, fixedNow).IsOverdue)
		})
	}
}

func TestToCardResponses_NeverNil(t *testing.T) {
	assert.NotNil(t, ToCardResponses(nil, fixedNow))
	assert.Len(t, ToCardResponses([]Card{{}, {}}, fixedNow), 2)
}
