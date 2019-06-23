/*
Package bip44 implements the bip44 spec https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki
*/
package bip44

import (
	"errors"
	"fmt"

	"github.com/skycoin/skycoin/src/cipher/bip32"
)

// Bip44's bip32 path: m / purpose' / coin_type' / account' / change / address_index

var (
	// ErrInvalidCoinType coin_type is less than 0x80000000
	ErrInvalidCoinType = errors.New("Invalid coin type")

	// ErrInvalidAccount account is >= 0x80000000
	ErrInvalidAccount = errors.New("account index must be < 0x80000000")
)

// CoinType is the coin_type part of the bip44 path
type CoinType uint32

const (
	// CoinTypeBitcoin is the coin_type for Bitcoin
	CoinTypeBitcoin CoinType = 0x80000000
	// CoinTypeBitcoinTestnet is the coin_type for Skycoin
	CoinTypeBitcoinTestnet CoinType = 0x80000001
	// CoinTypeSkycoin is the coin_type for Skycoin
	CoinTypeSkycoin CoinType = 0x88800000
)

// Coin is a bip32 node at the `coin_type` level of a bip44 path
type Coin struct {
	*bip32.PrivateKey
}

// NewCoin creates a bip32 node at the `coin_type` level of a bip44 path
func NewCoin(seed []byte, coinType CoinType) (*Coin, error) {
	if uint32(coinType) < bip32.FirstHardenedChild {
		return nil, ErrInvalidCoinType
	}

	path := fmt.Sprintf("m/44'/%d'", uint32(coinType)-bip32.FirstHardenedChild)
	pk, err := bip32.NewPrivateKeyFromPath(seed, path)
	if err != nil {
		return nil, err
	}

	return &Coin{
		pk,
	}, nil
}

// Account creates a bip32 node at the `account'` level of the bip44 path.
// The account number should be as it would appear in the path string, without
// the apostrophe that indicates hardening
func (c *Coin) Account(account uint32) (*Account, error) {
	if account >= bip32.FirstHardenedChild {
		return nil, ErrInvalidAccount
	}

	pk, err := c.NewPrivateChildKey(account + bip32.FirstHardenedChild)
	if err != nil {
		return nil, err
	}

	return &Account{
		pk,
	}, nil
}

// Account is a bip32 node at the `account` level of a bip44 path
type Account struct {
	*bip32.PrivateKey
}

// External returns the external chain node, to be used for receiving coins
func (a *Account) External() (*bip32.PrivateKey, error) {
	return a.NewPrivateChildKey(0)
}

// Change returns the change chain node, to be used for change addresses
func (a *Account) Change() (*bip32.PrivateKey, error) {
	return a.NewPrivateChildKey(1)
}
