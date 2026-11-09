package webserver

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr                  string
	ProfileURL            string
	CommandTimeout        time.Duration
	ClientTimeout         time.Duration
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	ProfileMaxBytes       int64
	ReadHeaderTimeout     time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Addr:                  "127.0.0.1:8080",
		ProfileURL:            "http://127.0.0.1:8081",
		CommandTimeout:        700 * time.Millisecond,
		ClientTimeout:         600 * time.Millisecond,
		DialTimeout:           200 * time.Millisecond,
		ResponseHeaderTimeout: 400 * time.Millisecond,
		TLSHandshakeTimeout:   300 * time.Millisecond,
		ProfileMaxBytes:       4096,
		ReadHeaderTimeout:     2 * time.Second,
		IdleTimeout:           30 * time.Second,
		ShutdownTimeout:       3 * time.Second,
	}
	if value, ok := os.LookupEnv("HTTP_ADDR"); ok {
		cfg.Addr = value
	}
	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR: %w", err)
	}
	if value, ok := os.LookupEnv("PROFILE_URL"); ok {
		cfg.ProfileURL = value
	}
	baseURL, err := url.Parse(cfg.ProfileURL)
	if err != nil || baseURL.Hostname() == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return Config{}, fmt.Errorf("PROFILE_URL must be an http or https URL with a host")
	}

	durations := []struct {
		name  string
		value *time.Duration
	}{
		{"COMMAND_TIMEOUT", &cfg.CommandTimeout},
		{"CLIENT_TIMEOUT", &cfg.ClientTimeout},
		{"DIAL_TIMEOUT", &cfg.DialTimeout},
		{"RESPONSE_HEADER_TIMEOUT", &cfg.ResponseHeaderTimeout},
		{"TLS_HANDSHAKE_TIMEOUT", &cfg.TLSHandshakeTimeout},
		{"READ_HEADER_TIMEOUT", &cfg.ReadHeaderTimeout},
		{"IDLE_TIMEOUT", &cfg.IdleTimeout},
		{"SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout},
	}
	for _, setting := range durations {
		if value, ok := os.LookupEnv(setting.name); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil || parsed <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive duration, for example 700ms", setting.name)
			}
			*setting.value = parsed
		}
	}
	if value, ok := os.LookupEnv("PROFILE_MAX_BYTES"); ok {
		cfg.ProfileMaxBytes, err = strconv.ParseInt(value, 10, 64)
		if err != nil || cfg.ProfileMaxBytes <= 0 {
			return Config{}, fmt.Errorf("PROFILE_MAX_BYTES must be a positive integer")
		}
	}
	return cfg, nil
}
