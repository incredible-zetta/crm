package config

import (
	"os"
	"testing"
)

func TestLoadRequiresAPIKey(t *testing.T) {
	os.Clearenv()
	if _, err := Load(); err == nil {
		t.Fatal("expected error when MCP_API_KEY missing")
	}
}

func TestLoadRequiresDSN(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("BASE_URL", "http://x")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DB_DSN missing")
	}
}

func TestLoadRequiresBaseURL(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when BASE_URL missing")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" {
		t.Fatalf("want default port 8080, got %s", c.Port)
	}
	if c.SchedulerIntervalSec != 15 {
		t.Fatalf("want default 15, got %d", c.SchedulerIntervalSec)
	}
}

func TestLoadInvalidInterval(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("SCHEDULER_INTERVAL_SEC", "notanumber")
	if _, err := Load(); err == nil {
		t.Fatal("expected error on invalid interval")
	}
}
