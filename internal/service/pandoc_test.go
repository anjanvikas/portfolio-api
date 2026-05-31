package service

import (
	"context"
	"errors"
	"testing"
)

func TestConvertDocx_BinaryMissing(t *testing.T) {
	c := &PandocConverter{binary: "definitely-not-a-real-binary-xyz"}
	_, err := c.ConvertDocx(context.Background(), []byte("PK..."))
	if !errors.Is(err, ErrConverterUnavailable) {
		t.Fatalf("expected ErrConverterUnavailable, got %v", err)
	}
}
