package secp256k1go

import (
	"bytes"
	"fmt"
)

// DecompressPoint decompresses point
func DecompressPoint(X []byte, off bool, Y []byte) {
	var rx, ry, c, x2, x3 Field
	rx.SetB32(X)
	rx.Sqr(&x2)
	rx.Mul(&x3, &x2)
	c.SetInt(7)
	c.SetAdd(&x3)
	c.Sqrt(&ry)
	ry.Normalize()
	if ry.IsOdd() != off {
		ry.Negate(&ry, 1)
	}
	ry.Normalize()
	ry.GetB32(Y)
}

// RecoverPublicKey recovers a public key from a signature and the message it signed.
// Returns nil on error with an int error code. Returns 1 on success.
func RecoverPublicKey(sigBytes, msgBytes []byte, recid int) ([]byte, int) {
	if len(sigBytes) != 64 {
		panic("must pass in 64 byte pubkey")
	}

	var pubkey XY
	var sig Signature
	var msg Number

	sig.ParseBytes(sigBytes[0:64])

	if sig.R.Sign() <= 0 || sig.R.Cmp(&TheCurve.Order.Int) >= 0 {
		if sig.R.Sign() == 0 {
			return nil, -1
		}
		if sig.R.Sign() <= 0 {
			return nil, -2
		}
		if sig.R.Cmp(&TheCurve.Order.Int) >= 0 {
			return nil, -3
		}
		return nil, -4
	}
	if sig.S.Sign() <= 0 || sig.S.Cmp(&TheCurve.Order.Int) >= 0 {
		return nil, -5
	}

	msg.SetBytes(msgBytes)
	if !sig.Recover(&pubkey, &msg, recid) {
		return nil, -6
	}

	return pubkey.Bytes(), 1
}

// Multiply standard EC multiplacation k(xy)
// xy is the compressed public key format (33 bytes long)
func Multiply(xy, k []byte) []byte {
	var pk XY
	var xyz XYZ
	var na, nzero Number
	if err := pk.ParsePubkey(xy); err != nil {
		return nil
	}
	xyz.SetXY(&pk)
	na.SetBytes(k)
	xyz.ECmult(&xyz, &na, &nzero)
	pk.SetXYZ(&xyz)

	if !pk.IsValid() {
		panic("Multiply pk is invalid")
	}
	return pk.Bytes()
}

// test assumptions
func pubkeyTest(pk XY) {
	if !pk.IsValid() {
		panic("IMPOSSIBLE3: pubkey invalid")
	}
	var pk2 XY
	if err := pk2.ParsePubkey(pk.Bytes()); err != nil {
		panic(fmt.Sprintf("IMPOSSIBLE2: parse failed: %v", err))
	}
	if !pk2.IsValid() {
		panic("IMPOSSIBLE3: parse failed non valid key")
	}
	if PubkeyIsValid(pk2.Bytes()) != 1 {
		panic("IMPOSSIBLE4: pubkey failed")
	}
}

// BaseMultiply base multiply
func BaseMultiply(k []byte) []byte {
	var n Number
	var pk XY
	n.SetBytes(k)
	r := ECmultGen(n)
	pk.SetXYZ(&r)
	if !pk.IsValid() {
		panic("BaseMultiply pk is invalid") // should not occur
	}

	pubkeyTest(pk)

	return pk.Bytes()
}

// BaseMultiplyAdd computes G*k + xy
// Returns 33 bytes out (compressed pubkey).
func BaseMultiplyAdd(xy, k []byte) []byte {
	var n Number
	var pk XY
	if err := pk.ParsePubkey(xy); err != nil {
		return nil
	}
	n.SetBytes(k)
	r := ECmultGen(n)
	r.AddXY(&r, &pk)
	pk.SetXYZ(&r)

	pubkeyTest(pk)
	return pk.Bytes()
}

// GeneratePublicKey generates a public key from secret key bytes.
// The secret key must 32 bytes.
func GeneratePublicKey(k []byte) []byte {
	if len(k) != 32 {
		panic("secret key length must be 32 bytes")
	}
	var n Number
	var pk XY

	// must not be zero
	// must not be negative
	// must be less than order of curve
	n.SetBytes(k)
	if n.Sign() <= 0 || n.Cmp(&TheCurve.Order.Int) >= 0 {
		panic("only call for valid seckey, check that seckey is valid first")
		return nil
	}
	r := ECmultGen(n)
	pk.SetXYZ(&r)
	if !pk.IsValid() {
		panic("public key derived from secret key is unexpectedly valid") // should not occur
	}
	pubkeyTest(pk)
	return pk.Bytes()
}

func init() {
	/* Code snippet to brute force inputs whose sha256 hash is not a valid secret key*/
	/*
		randBytes := func(n int) []byte {
			b := make([]byte, n)
			_, err := rand.Read(b)
			if err != nil {
				panic(err)
			}
			return b
		}
		sha256Hash := sha256.New()

		for true {
			b := randBytes(32)

			sha256Hash.Reset()
			sha256Hash.Write(b)
			h := sha256Hash.Sum(nil)

			code := SeckeyIsValid(h)
			if code == -1 {
				fmt.Println("found sha256(value) that generates invalid secret key")
				fmt.Println("value(hex):", hex.EncodeToString(b))
				fmt.Println("hash(hex):", hex.EncodeToString(h))
				fmt.Println("validity code:", code)
				panic("done")
				// if code == -1 {
				// 	panic("done")
				// }
			}
		}
	*/
}

// SeckeyIsValid 1 on success
// must not be zero
// must not be negative
// must be less than order of curve
func SeckeyIsValid(seckey []byte) int {
	if len(seckey) != 32 {
		panic("SeckeyIsValid seckey must be 32 bytes")
	}
	var n Number
	n.SetBytes(seckey)
	// must not be zero
	// must not be negative
	// must be less than order of curve
	if n.Sign() <= 0 {
		return -1
	}
	if n.Cmp(&TheCurve.Order.Int) >= 0 {
		return -2
	}
	return 1
}

// PubkeyIsValid returns 1 on success
func PubkeyIsValid(pubkey []byte) int {
	if len(pubkey) != 33 {
		panic("public key length must be 33 bytes")
		return -2
	}
	var pubkey1 XY
	if err := pubkey1.ParsePubkey(pubkey); err != nil {
		return -1
	}

	if !bytes.Equal(pubkey1.Bytes(), pubkey) {
		panic("pubkey parses but serialize/deserialize roundtrip fails")
	}

	if !pubkey1.IsValid() {
		return -3 // invalid, point is infinity or some other problem
	}

	return 1
}
