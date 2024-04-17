package main

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServiceAURL string
	ServiceBURL string
	Port        string
}

func NewConfig() (*Config, error) {
	config := &Config{
		ServiceAURL: os.Getenv("GATEWAY_A_URL"),
		ServiceBURL: os.Getenv("GATEWAY_B_URL"),
		Port:        os.Getenv("GATEWAY_PORT"),
	}

	if config.ServiceAURL == "" {
		return nil, fmt.Errorf("env variable GATEWAY_A_URL is empty")
	}

	if config.ServiceBURL == "" {
		return nil, fmt.Errorf("env variable GATEWAY_B_URL is empty")
	}

	_, err := strconv.ParseInt(config.Port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("env variable GATEWAY_PORT is invalid")
	}

	return config, nil
}
