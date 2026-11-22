package checker

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

type digestChecker struct {
	algorithm string
	target    []byte
}

func newDigest(algorithm, encoded string) (Checker, error) {
	target, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid %s hash: %w", algorithm, err)
	}

	want := md5.Size
	if algorithm == "sha1" {
		want = sha1.Size
	}
	if len(target) != want {
		return nil, fmt.Errorf("invalid %s hash length", algorithm)
	}

	return &digestChecker{algorithm: algorithm, target: target}, nil
}

func (c *digestChecker) Match(candidate []byte) bool {
	if c.algorithm == "md5" {
		sum := md5.Sum(candidate)
		return subtle.ConstantTimeCompare(sum[:], c.target) == 1
	}

	sum := sha1.Sum(candidate)
	return subtle.ConstantTimeCompare(sum[:], c.target) == 1
}

func (c *digestChecker) Algorithm() string  { return c.algorithm }
func (c *digestChecker) Parameters() string { return "" }
func (c *digestChecker) MemoryKiB() uint64  { return 0 }
