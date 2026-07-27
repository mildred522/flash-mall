package existencefilter

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

type Result string

const (
	ResultPossible Result = "possible"
	ResultAbsent   Result = "absent"
	ResultUnready  Result = "unready"
)

var (
	ErrUnready           = errors.New("existence filter is not ready")
	ErrRebuildInProgress = errors.New("existence filter rebuild is in progress")
)

type IDSource interface {
	ProductIDBatch(ctx context.Context, afterID int64, limit int) ([]int64, error)
}

type Filter interface {
	Check(context.Context, int64) (Result, error)
	Add(context.Context, int64) error
	Rebuild(context.Context) error
}

type NegativeCache interface {
	Contains(context.Context, int64) (bool, error)
	Mark(context.Context, int64) error
	Invalidate(context.Context, int64) error
}

type Status struct {
	Enabled    bool   `json:"enabled"`
	Ready      bool   `json:"ready"`
	State      string `json:"state"`
	Generation string `json:"generation,omitempty"`
	Items      int64  `json:"items,omitempty"`
}

type Config struct {
	Prefix            string
	NegativePrefix    string
	ExpectedItems     uint64
	FalsePositiveRate float64
	BatchSize         int
	OperationTimeout  time.Duration
	NegativeTTL       time.Duration
	RebuildLockTTL    time.Duration
	OldGenerationTTL  time.Duration
}

type normalizedConfig struct {
	Config
	bitCount  uint64
	hashCount uint64
}

func (c Config) normalize() normalizedConfig {
	c.Prefix = strings.Trim(strings.TrimSpace(c.Prefix), ":")
	if c.Prefix == "" {
		c.Prefix = "flashmall:hertz:existence:product"
	}
	c.NegativePrefix = strings.Trim(strings.TrimSpace(c.NegativePrefix), ":")
	if c.NegativePrefix == "" {
		c.NegativePrefix = c.Prefix + ":negative"
	}
	if c.ExpectedItems == 0 {
		c.ExpectedItems = 1_000_000
	}
	if c.FalsePositiveRate <= 0 || c.FalsePositiveRate >= 1 {
		c.FalsePositiveRate = 0.01
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 1000
	}
	if c.OperationTimeout <= 0 {
		c.OperationTimeout = 200 * time.Millisecond
	}
	if c.NegativeTTL <= 0 {
		c.NegativeTTL = 30 * time.Second
	}
	if c.RebuildLockTTL <= 0 {
		c.RebuildLockTTL = 10 * time.Minute
	}
	if c.OldGenerationTTL <= 0 {
		c.OldGenerationTTL = time.Hour
	}
	n := float64(c.ExpectedItems)
	m := uint64(math.Ceil(-n * math.Log(c.FalsePositiveRate) / math.Pow(math.Ln2, 2)))
	k := uint64(math.Round(float64(m) / n * math.Ln2))
	if k == 0 {
		k = 1
	}
	return normalizedConfig{Config: c, bitCount: m, hashCount: k}
}
