package main

import (
	"encoding/json"
	"errors"
	"os"
)

type ServerConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	SocketType string `json:"socket_type"`
	Auth       string `json:"auth"`
}

type DomainConfig struct {
	DisplayName string       `json:"display_name"`
	ShortName   string       `json:"short_name"`
	IMAP        ServerConfig `json:"imap"`
	SMTP        ServerConfig `json:"smtp"`
}

func loadDomainConfig(path string) (*DomainConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	defer f.Close()

	var cfg DomainConfig
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
