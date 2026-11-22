package checker

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type argon2idChecker struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	target      []byte
}

func newArgon2id(encoded string) (Checker, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return nil, fmt.Errorf("invalid argon2id PHC string")
	}

	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return nil, fmt.Errorf("invalid argon2id parameters")
	}

	if !strings.HasPrefix(params[0], "m=") {
		return nil, fmt.Errorf("invalid argon2id parameter order")
	}
	memory, err := strconv.ParseUint(strings.TrimPrefix(params[0], "m="), 10, 32)
	if err != nil || memory == 0 {
		return nil, fmt.Errorf("invalid argon2id memory")
	}

	if !strings.HasPrefix(params[1], "t=") {
		return nil, fmt.Errorf("invalid argon2id iterations")
	}
	iterations, err := strconv.ParseUint(strings.TrimPrefix(params[1], "t="), 10, 32)
	if err != nil || iterations == 0 {
		return nil, fmt.Errorf("invalid argon2id iterations")
	}

	if !strings.HasPrefix(params[2], "p=") {
		return nil, fmt.Errorf("invalid argon2id parallelism")
	}
	parallelism, err := strconv.ParseUint(strings.TrimPrefix(params[2], "p="), 10, 8)
	if err != nil || parallelism == 0 {
		return nil, fmt.Errorf("invalid argon2id parallelism")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return nil, fmt.Errorf("invalid argon2id salt")
	}

	target, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(target) == 0 {
		return nil, fmt.Errorf("invalid argon2id key")
	}

	return &argon2idChecker{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
		salt:        salt,
		target:      target,
	}, nil
}

func (c *argon2idChecker) Match(candidate []byte) bool {
	got := argon2.IDKey(candidate, c.salt, c.iterations, c.memory, c.parallelism, uint32(len(c.target)))
	return subtle.ConstantTimeCompare(got, c.target) == 1
}

func (c *argon2idChecker) Algorithm() string { return "argon2id" }

func (c *argon2idChecker) Parameters() string {
	return fmt.Sprintf("m=%d,t=%d,p=%d", c.memory, c.iterations, c.parallelism)
}

func (c *argon2idChecker) MemoryKiB() uint64 { return uint64(c.memory) }
