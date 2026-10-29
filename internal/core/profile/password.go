package profile

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"io"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
)

// Parameters of the password hash (SPEC §3.1).
const (
	// hashScheme starts every stored hash.
	hashScheme = "pbkdf2-sha256"
	// DefaultIterations is the PBKDF2 iteration count of new hashes.
	DefaultIterations = 600_000
	saltLength        = 16
	keyLength         = 32

	// Limits applied to a stored hash before it is used, so that a damaged or
	// hand-edited file cannot make a login run for minutes.
	maxIterations = 10_000_000
	minSaltLength = 8
	maxSaltLength = 64
	maxKeyLength  = 64
)

// hasher makes and checks password hashes. Its fields are replaceable so the
// tests can use few iterations and make the random source or the derivation fail.
type hasher struct {
	iterations int
	random     io.Reader
	derive     func(password string, salt []byte, iterations, length int) ([]byte, error)
}

func newHasher() hasher {
	return hasher{
		iterations: DefaultIterations,
		random:     rand.Reader,
		derive: func(password string, salt []byte, iterations, length int) ([]byte, error) {
			return pbkdf2.Key(sha256.New, password, salt, iterations, length)
		},
	}
}

// hash returns "pbkdf2-sha256$<iterations>$<salt b64>$<key b64>" for password,
// with a fresh 16-byte random salt and a 32-byte key.
func (h hasher) hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(h.random, salt); err != nil {
		return "", errors.NewErr("SaltGenerationFailed", err)
	}
	key, err := h.derive(password, salt, h.iterations, keyLength)
	if err != nil {
		return "", errors.NewErr("KeyDerivationFailed", err)
	}
	return hashScheme + "$" + strconv.Itoa(h.iterations) + "$" +
		base64.StdEncoding.EncodeToString(salt) + "$" +
		base64.StdEncoding.EncodeToString(key), nil
}

// verify reports whether password matches the stored hash. The iteration count
// comes from the stored hash. The keys are compared in constant time.
func (h hasher) verify(password, stored string) (bool, error) {
	p, err := parseHash(stored)
	if err != nil {
		return false, err
	}
	key, err := h.derive(password, p.salt, p.iterations, len(p.key))
	if err != nil {
		return false, errors.NewErr("KeyDerivationFailed", err)
	}
	return subtle.ConstantTimeCompare(key, p.key) == 1, nil
}

// parsedHash is a stored hash taken apart.
type parsedHash struct {
	iterations int
	salt, key  []byte
}

// parseHash takes a stored hash apart and checks its limits. The hash itself is
// kept out of the errors: it is a value derived from a secret.
func parseHash(stored string) (parsedHash, error) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != hashScheme {
		return parsedHash{}, errors.New("PasswordHashMalformed", "stored password hash is not pbkdf2-sha256$iterations$salt$key")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 || iterations > maxIterations {
		return parsedHash{}, errors.New("PasswordHashIterationsInvalid", "stored password hash has a bad iteration count")
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < minSaltLength || len(salt) > maxSaltLength {
		return parsedHash{}, errors.New("PasswordHashSaltInvalid", "stored password hash has a bad salt")
	}
	key, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil || len(key) == 0 || len(key) > maxKeyLength {
		return parsedHash{}, errors.New("PasswordHashKeyInvalid", "stored password hash has a bad key")
	}
	return parsedHash{iterations: iterations, salt: salt, key: key}, nil
}
