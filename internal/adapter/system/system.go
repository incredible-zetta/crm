package system

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/port"
)

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

var _ port.Clock = RealClock{}

type CryptoIDGen struct{}

func (CryptoIDGen) ExportID() (string, error)  { return hex16() }
func (CryptoIDGen) UnsubCode() (string, error) { return hex16() }

func hex16() (string, error) {
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

var _ port.IDGenerator = CryptoIDGen{}
