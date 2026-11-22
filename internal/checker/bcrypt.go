package checker

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type bcryptChecker struct {
	encoded []byte
	cost    int
}

func newBcrypt(encoded string) (Checker, error) {
	cost, err := bcrypt.Cost([]byte(encoded))
	if err != nil {
		return nil, fmt.Errorf("invalid bcrypt hash: %w", err)
	}

	return &bcryptChecker{encoded: []byte(encoded), cost: cost}, nil
}

func (c *bcryptChecker) Match(candidate []byte) bool {
	return bcrypt.CompareHashAndPassword(c.encoded, candidate) == nil
}

func (c *bcryptChecker) Algorithm() string  { return "bcrypt" }
func (c *bcryptChecker) Parameters() string { return fmt.Sprintf("cost=%d", c.cost) }
func (c *bcryptChecker) MemoryKiB() uint64  { return 0 }
