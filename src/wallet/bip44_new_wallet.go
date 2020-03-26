// This file is still under refactoring, once it gets done, we will replace the
// old bip44_wallet.go.

package wallet

import (
	"fmt"
	"strconv"
	"time"

	"github.com/SkycoinProject/skycoin/src/cipher"

	"github.com/SkycoinProject/skycoin/src/cipher/bip32"
	"github.com/SkycoinProject/skycoin/src/cipher/bip44"
)

const (
	// Bip44WalletVersion Bip44 wallet version
	Bip44WalletVersion = "0.4"
)

var (
	// defaultBip44WalletDecoder is the default bip44 wallet decoder
	defaultBip44WalletDecoder = &Bip44WalletJSONDecoder{}
)

// Bip44WalletNew manages keys using the original Skycoin deterministic
// keypair generator method.
type Bip44WalletNew struct {
	// Meta wallet meta data
	Meta
	// accounts bip44 wallet accounts
	accounts accountManager
	// decoder is used to encode/decode bip44 wallet to/from []byte
	decoder Bip44WalletDecoder
}

// accountManager is the interface that manages the bip44 wallet accounts.
type accountManager interface {
	// New creates a new account, returns the account index, and error, if any
	New(opts bip44AccountCreateOptions) (uint32, error)
	// NewAddresses generates addresses on selected account
	NewAddresses(index, chain, num uint32) ([]cipher.Addresser, error)
	// Len returns the account number
	Len() uint32
}

// Bip44WalletDecoder is the interface that wraps the Encode and Decode methods.
//
// Encode method encodes the wallet to bytes, Decode method decodes bytes to bip44 wallet.
type Bip44WalletDecoder interface {
	Encode(w *Bip44WalletNew) ([]byte, error)
	Decode(b []byte) (*Bip44WalletNew, error)
}

// Bip44WalletCreateOptions options for creating the bip44 wallet
type Bip44WalletCreateOptions struct {
	Filename       string
	Version        string
	Label          string
	Seed           string
	SeedPassphrase string
	CoinType       CoinType
	WalletDecoder  Bip44WalletDecoder
}

// NewBip44WalletNew create a bip44 wallet with options
func NewBip44WalletNew(opts Bip44WalletCreateOptions) *Bip44WalletNew {
	wlt := &Bip44WalletNew{
		Meta: Meta{
			metaFilename:       opts.Filename,
			metaVersion:        Bip44WalletVersion,
			metaLabel:          opts.Label,
			metaSeed:           opts.Seed,
			metaSeedPassphrase: opts.SeedPassphrase,
			metaCoin:           string(opts.CoinType),
			metaTimestamp:      strconv.FormatInt(time.Now().Unix(), 10),
			metaEncrypted:      "false",
		},
		accounts: &bip44Accounts{},
		decoder:  opts.WalletDecoder,
	}

	if wlt.decoder == nil {
		wlt.decoder = defaultBip44WalletDecoder
	}

	bip44CoinType := resolveCoinAdapter(opts.CoinType).Bip44CoinType()
	wlt.Meta.setBip44Coin(bip44CoinType)

	return wlt
}

// NewAccount create a bip44 wallet account, returns account index and
// error, if any.
func (w *Bip44WalletNew) NewAccount(name string) (uint32, error) {
	opts := bip44AccountCreateOptions{
		name:           name,
		seed:           w.Meta.Seed(),
		seedPassphrase: w.Meta.SeedPassphrase(),
		coinType:       w.Meta.Coin(),
	}

	return w.accounts.New(opts)
}

// NewAddresses creates addresses
func (w *Bip44WalletNew) NewAddresses(account, chain, n uint32) ([]cipher.Addresser, error) {
	return w.accounts.NewAddresses(account, chain, n)
}

func makeChainPubKeys(a *bip44.Account) (*bip32.PublicKey, *bip32.PublicKey, error) {
	external, err := a.NewPublicChildKey(0)
	if err != nil {
		return nil, nil, fmt.Errorf("create external chain public key failed: %v", err)
	}

	change, err := a.NewPublicChildKey(1)
	if err != nil {
		return nil, nil, fmt.Errorf("create change chain public key failed: %v", err)
	}
	return external, change, nil
}

// Serialize encodes the bip44 wallet to []byte
func (w *Bip44WalletNew) Serialize() ([]byte, error) {
	if w.decoder == nil {
		w.decoder = defaultBip44WalletDecoder
	}
	return w.decoder.Encode(w)
}

// Deserialize decodes the []byte to a bip44 wallet
func (w *Bip44WalletNew) Deserialize(b []byte) error {
	if w.decoder == nil {
		w.decoder = defaultBip44WalletDecoder
	}
	toW, err := w.decoder.Decode(b)
	if err != nil {
		return err
	}

	toW.decoder = w.decoder
	*w = *toW
	return nil
}
