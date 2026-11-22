package search

import (
	"fmt"
	"math"
	"math/big"
	"time"
)

type Charset struct {
	Name    string
	Symbols []byte
}

type Config struct {
	Charset          Charset
	MinLength        int
	MaxLength        int
	Workers          int
	ProgressInterval time.Duration
	Total            uint64
}

func ResolveCharset(value string) (Charset, error) {
	profiles := map[string]string{
		"digits":    "0123456789",
		"lower":     "abcdefghijklmnopqrstuvwxyz",
		"alnum":     "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		"printable": " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~",
	}

	symbols, ok := profiles[value]
	if !ok {
		return Charset{}, fmt.Errorf("unknown charset %q", value)
	}

	return Charset{Name: value, Symbols: []byte(symbols)}, nil
}

func SearchSpaceSize(charsetSize, minLength, maxLength int) (uint64, error) {
	if charsetSize <= 0 || minLength < 1 || minLength > maxLength || maxLength > 64 {
		return 0, fmt.Errorf("invalid search bounds")
	}

	base := big.NewInt(int64(charsetSize))
	term := new(big.Int)
	total := new(big.Int)

	for length := minLength; length <= maxLength; length++ {
		term.Exp(base, big.NewInt(int64(length)), nil)
		total.Add(total, term)
	}

	if total.Cmp(new(big.Int).SetUint64(math.MaxUint64)) > 0 {
		return 0, fmt.Errorf("search space exceeds uint64")
	}

	return total.Uint64(), nil
}

func NewConfig(charset Charset, minLength, maxLength, workers int, progress time.Duration) (Config, error) {
	if workers <= 0 || progress < 0 {
		return Config{}, fmt.Errorf("invalid workers or progress")
	}

	total, err := SearchSpaceSize(len(charset.Symbols), minLength, maxLength)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Charset:          charset,
		MinLength:        minLength,
		MaxLength:        maxLength,
		Workers:          workers,
		ProgressInterval: progress,
		Total:            total,
	}, nil
}
