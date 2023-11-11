package main

import (
	"embed"
	"fmt"
)

//go:embed assets/*
var assetFS embed.FS

func asset(name string) ([]byte, error) {
	data, err := assetFS.ReadFile(name)
	if err != nil {
		return data, fmt.Errorf("reading asset: %w", err)
	}

	return data, nil
}

func mustAsset(name string) []byte {
	data, err := asset(name)
	if err != nil {
		panic(err)
	}
	return data
}
