package id

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropyPool = sync.Pool{
		New: func() any {
			return rand.Reader
		},
	}
)

// Prefixes defined in the API contract
const (
	PrefixUser      = "usr_"
	PrefixPractice  = "ps_"
	PrefixExam      = "exam_"
	PrefixEvent     = "evt_"
	PrefixRequest   = "req_"
	PrefixDevice    = "dev_"
	PrefixPurchase  = "pur_"
	PrefixAnalysis  = "ai_"
	PrefixAudit     = "aud_"
)

// New generates a ULID with the given prefix, e.g. "usr_01J..."
func New(prefix string) string {
	reader := entropyPool.Get().(io.Reader)
	defer entropyPool.Put(reader)

	entropy := ulid.Monotonic(reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), entropy)
	if err != nil {
		// Fallback
		var b [16]byte
		_, _ = rand.Read(b[:])
		return fmt.Sprintf("%s%x", prefix, b)
	}
	return prefix + id.String()
}

// FastRequestId creates a request id for tracing
func FastRequestId() string {
	return New(PrefixRequest)
}
