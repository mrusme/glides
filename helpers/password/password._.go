package password

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
	"xn--gckvb8fzb.com/glides/errs"
)

const (
	HashMem    = 64 * 1024
	HashIter   = 3
	HashSaltln = 16
	HashKeyln  = 32
)

type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func RandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func Decode(encoded string) (
	params *Params,
	salt,
	key []byte,
	err error,
) {
	r := strings.NewReader(encoded)

	_, err = fmt.Fscanf(r, "$argon2id$")
	if err != nil {
		return nil, nil, nil, errs.ErrHashVariantIncompatible
	}

	var version int
	_, err = fmt.Fscanf(r, "v=%d$", &version)
	if err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, errs.ErrHashVersionIncompatible
	}

	params = &Params{}
	_, err = fmt.Fscanf(r,
		"m=%d,t=%d,p=%d$",
		&params.Memory,
		&params.Iterations,
		&params.Parallelism,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	rest, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, nil, err
	}
	if bytes.ContainsAny(rest, "\r\n") {
		return nil, nil, nil, errs.ErrHashInvalid
	}

	var i int
	if i = bytes.IndexByte(rest, '$'); i == -1 {
		return nil, nil, nil, errs.ErrHashInvalid
	}

	b64Enc := base64.RawStdEncoding.Strict()

	salt = make([]byte, b64Enc.DecodedLen(i))
	_, err = b64Enc.Decode(salt, rest[:i])
	if err != nil {
		return nil, nil, nil, err
	}
	params.SaltLength = uint32(len(salt))

	key = make([]byte, b64Enc.DecodedLen(len(rest)-i-1))
	_, err = b64Enc.Decode(key, rest[i+1:])
	if err != nil {
		return nil, nil, nil, err
	}
	params.KeyLength = uint32(len(key))

	return params, salt, key, nil
}

func Hash(password string) (encoded string, err error) {
	salt, err := RandomBytes(HashSaltln)
	if err != nil {
		return "", err
	}

	parallelism := uint8(runtime.NumCPU())

	key := argon2.IDKey(
		[]byte(password),
		salt,
		HashIter,
		HashMem,
		parallelism,
		HashKeyln,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		HashMem,
		HashIter,
		parallelism,
		b64Salt,
		b64Key,
	), nil
}

func HashRandom() (encoded string, err error) {
	b, err := RandomBytes(HashKeyln)
	if err != nil {
		return "", err
	}

	return Hash(base64.RawStdEncoding.EncodeToString(b))
}

func Check(encoded string, password string) (
	match bool,
	params *Params,
	err error,
) {
	params, salt, key, err := Decode(encoded)
	if err != nil {
		return false, nil, err
	}

	otherKey := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)

	keyLen := int32(len(key))
	otherKeyLen := int32(len(otherKey))

	if subtle.ConstantTimeEq(keyLen, otherKeyLen) == 0 {
		return false, params, nil
	}
	if subtle.ConstantTimeCompare(key, otherKey) == 1 {
		return true, params, nil
	}
	return false, params, nil
}
