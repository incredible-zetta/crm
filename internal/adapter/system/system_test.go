package system

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestRealClockNow(t *testing.T) {
	clock := RealClock{}
	now := clock.Now()
	systemNow := time.Now()

	diff := systemNow.Sub(now)
	if diff < 0 {
		diff = -diff
	}

	if diff > time.Second {
		t.Errorf("expected clock.Now() (%v) to be close to time.Now() (%v), diff is %v", now, systemNow, diff)
	}
}

func TestCryptoIDGen(t *testing.T) {
	idGen := CryptoIDGen{}

	// Test ExportID
	id1, err := idGen.ExportID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(id1) != 16 {
		t.Errorf("expected ExportID length of 16, got %d (value: %q)", len(id1), id1)
	}
	if _, err := hex.DecodeString(id1); err != nil {
		t.Errorf("expected ExportID to be valid hex string, got %q: %v", id1, err)
	}

	id2, err := idGen.ExportID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id1 == id2 {
		t.Errorf("expected successive ExportIDs to be unique, but both were %q", id1)
	}

	// Test UnsubCode
	code1, err := idGen.UnsubCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code1) != 16 {
		t.Errorf("expected UnsubCode length of 16, got %d (value: %q)", len(code1), code1)
	}
	if _, err := hex.DecodeString(code1); err != nil {
		t.Errorf("expected UnsubCode to be valid hex string, got %q: %v", code1, err)
	}

	code2, err := idGen.UnsubCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code1 == code2 {
		t.Errorf("expected successive UnsubCodes to be unique, but both were %q", code1)
	}
}
