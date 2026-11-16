package services_test

import (
	"strings"
	"testing"

	"kanban-management/internal/services"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLabelName(t *testing.T) {
	assert.Equal(t, "bug", services.NormalizeLabelName("Bug"))
	assert.Equal(t, "bug", services.NormalizeLabelName("  BUG  "))
	assert.Equal(t, "front end", services.NormalizeLabelName(" Front End "))
	assert.Equal(t, "", services.NormalizeLabelName("   "))
}

func TestCleanLabelName(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"normal", "Bug", "Bug", false},
		{"aparado", "  Bug  ", "Bug", false},
		{"vazio", "", "", true},
		{"só espaços", "   ", "", true},
		{"30 caracteres", strings.Repeat("a", 30), strings.Repeat("a", 30), false},
		{"31 caracteres", strings.Repeat("a", 31), "", true},
		{"30 runes multibyte", strings.Repeat("é", 30), strings.Repeat("é", 30), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := services.CleanLabelName(tc.in)

			if tc.wantErr {
				assert.ErrorIs(t, err, services.ErrInvalidLabelName)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateColor(t *testing.T) {
	for _, valid := range []string{"#000000", "#FFFFFF", "#61bd4f", "#AbCdEf"} {
		assert.True(t, services.ValidateColor(valid), valid)
	}

	for _, invalid := range []string{"", "red", "#fff", "#12345", "#1234567", "123456", "#GGGGGG", " #123456", "#123456 "} {
		assert.False(t, services.ValidateColor(invalid), "%q", invalid)
	}
}

func TestPaletteColor(t *testing.T) {
	for i := -3; i < 30; i++ {
		assert.True(t, services.ValidateColor(services.PaletteColor(i)), "a paleta só tem #RRGGBB válidos")
	}

	assert.Equal(t, services.PaletteColor(0), services.PaletteColor(len(services.LabelPalette)))
}
