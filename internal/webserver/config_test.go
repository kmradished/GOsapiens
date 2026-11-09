package webserver_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/webserver"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("PROFILE_URL", "http://127.0.0.1:9091")
	t.Setenv("COMMAND_TIMEOUT", "950ms")
	t.Setenv("PROFILE_MAX_BYTES", "2048")
	cfg, err := webserver.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9090" || cfg.ProfileURL != "http://127.0.0.1:9091" ||
		cfg.CommandTimeout != 950*time.Millisecond || cfg.ProfileMaxBytes != 2048 {
		t.Fatalf("environment settings were lost: %+v", cfg)
	}
}

func TestLoadConfigRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"HTTP_ADDR", "wrong address"},
		{"PROFILE_URL", "file:///tmp/profiles"},
		{"PROFILE_URL", "http://"},
		{"COMMAND_TIMEOUT", "0s"},
		{"CLIENT_TIMEOUT", "later"},
		{"SHUTDOWN_TIMEOUT", "-1s"},
		{"PROFILE_MAX_BYTES", "0"},
	} {
		t.Run(tc.name+"/"+tc.value, func(t *testing.T) {
			t.Setenv(tc.name, tc.value)
			_, err := webserver.LoadConfig()
			if err == nil || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("LoadConfig error = %v; want error naming %s", err, tc.name)
			}
		})
	}
}
