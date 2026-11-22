package checker

import (
	"fmt"
	"strings"
)

type Checker interface {
	Match(candidate []byte) bool
	Algorithm() string
	Parameters() string
	MemoryKiB() uint64
}

func New(algorithm, encoded string) (Checker, error) {
	switch strings.ToLower(algorithm) {
	case "md5", "sha1":
		return newDigest(strings.ToLower(algorithm), encoded)
	case "bcrypt":
		return newBcrypt(encoded)
	case "argon2id":
		return newArgon2id(encoded)
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", algorithm)
	}
}
