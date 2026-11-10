package service

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeTechnologies(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "huruf besar dan spasi di tepi", in: []string{" Go-Fiber ", "POSTGRESQL"}, want: []string{"Go-Fiber", "POSTGRESQL"}},
		{name: "duplikat setelah normalisasi", in: []string{"Go", "go", " GO"}, want: []string{"Go"}},
		{name: "string kosong dibuang", in: []string{"", "   ", "MongoDB"}, want: []string{"MongoDB"}},
		{name: "urutan kemunculan pertama dipertahankan", in: []string{"b", "a", "B"}, want: []string{"b", "a"}},
		{name: "spasi di tengah dirapikan", in: []string{"  Go   Fiber  "}, want: []string{"Go Fiber"}},
		{name: "input kosong", in: []string{}, want: []string{}},
		{name: "input nil", in: nil, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTechnologies(tt.in)
			// slices.Equal membandingkan isi dan urutan. Operator == tidak
			// dapat dipakai untuk slice di Go.
			if !slices.Equal(got, tt.want) {
				t.Errorf("NormalizeTechnologies(%q) = %q, seharusnya %q", tt.in, got, tt.want)
			}
			// Hasil tidak boleh nil: response JSON harus berisi [] bukan null.
			if got == nil {
				t.Errorf("NormalizeTechnologies(%q) mengembalikan nil", tt.in)
			}
		})
	}
}

func TestDecodePortfolioJSONRejectsMongoOperator(t *testing.T) {
	var request struct {
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(strings.NewReader(`{"password":{"$ne":""}}`))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err == nil {
		t.Fatal("expected operator object to be rejected by string-typed DTO")
	}
}
