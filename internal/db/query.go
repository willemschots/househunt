package db

import (
	"errors"
	"strings"

	"github.com/willemschots/househunt/internal/krypto"
)

// Query helps build SQL queries using bind parameters.
// Use Query to construct parts of a query and use Param to add bind parameters.
// The final query and parameters can be retrieved using the Get method.
//
// The zero value is ready to use.
type Query struct {
	Encryptor     *krypto.Encryptor
	BlindIndexKey krypto.Key
	b             strings.Builder
	params        []any
	err           error
}

// Unsafe writes a non-parameterized part of a query.
func (q *Query) Unsafe(s string) {
	q.b.WriteString(s)
}

// Param writes a parameterized part of a query.
func (q *Query) Param(v any) {
	q.b.WriteString("?")
	q.params = append(q.params, v)
}

// Param writes a parameterized part of a query and encrypts the value before adding it to the query.
func (q *Query) ParamEncrypted(d []byte) {
	if q.Encryptor == nil {
		q.err = errors.New("no encryptor set")
		return
	}

	enc, err := q.Encryptor.Encrypt(d)
	if err != nil {
		q.err = errors.Join(q.err, err)
		return
	}

	q.Param(enc)
}

// ParamBlindIndex writes a parameterized part of a query and adds a blind index of the value to the query.
// Important Note: The blind indexes will need to be rebuild if the key or argon2 parameters change.
func (q *Query) ParamBlindIndex(d []byte) {
	hash, err := krypto.HashArgon2WithKey(d, q.BlindIndexKey)
	if err != nil {
		q.err = errors.Join(q.err, err)
		return
	}

	// overwrite the salt because we don't want to store it.
	hash.Salt = nil
	q.Param(hash.String())
}

// Params writes multiple parameterized parts of a query seperated by commas.
func (q *Query) Params(v ...any) {
	for i, p := range v {
		if i > 0 {
			q.b.WriteString(", ")
		}
		q.b.WriteString("?")
		q.params = append(q.params, p)
	}
}

// Get returns the constructed query and parameter values.
func (q *Query) Get() (string, []any, error) {
	return q.b.String(), q.params, q.err
}

// DecryptionTarget returns a decryptable value that can be used to scan encrypted values.
func (q *Query) DecryptionTarget() *Decryptable {
	return &Decryptable{
		encryptor: q.Encryptor,
	}
}

type Decryptable struct {
	encryptor *krypto.Encryptor
	Data      []byte
}

func (d *Decryptable) Scan(src any) error {
	b, ok := src.([]byte)
	if !ok {
		return errors.New("invalid type")
	}

	data, err := d.encryptor.Decrypt(b)
	if err != nil {
		return err
	}

	d.Data = data

	return nil
}
