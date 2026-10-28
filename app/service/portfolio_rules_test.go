package service

import (
	"encoding/json"
	"strings"
	"testing"
)

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
