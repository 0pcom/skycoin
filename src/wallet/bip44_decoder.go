package wallet

import (
	"bytes"
	"encoding/json"

	"github.com/SkycoinProject/skycoin/src/cipher"
	"github.com/SkycoinProject/skycoin/src/cipher/bip32"
)

// Bip44WalletJSONDecoder implements the WalletDecoder interface,
// which provides methods for encoding and decoding a bip44 wallet in JSON format.
type Bip44WalletJSONDecoder struct{}

// Encode encodes the bip44 wallet to []byte, and error, if any.
func (d Bip44WalletJSONDecoder) Encode(w *Bip44WalletNew) ([]byte, error) {
	rw := newReadableBip44WalletNew(w)
	return json.MarshalIndent(rw, "", "    ")
}

// Decode decodes  the []byte to a bip44 wallet.
func (d Bip44WalletJSONDecoder) Decode(b []byte) (*Bip44WalletNew, error) {
	br := bytes.NewReader(b)
	rw := readableBip44WalletNew{}
	if err := json.NewDecoder(br).Decode(&rw); err != nil {
		return nil, err
	}
	return rw.toWallet()
}

// readableBip44WalletNew readable bip44 wallet
type readableBip44WalletNew struct {
	Meta     `json:"meta"`
	Accounts readableBip44Accounts `json:"accounts"`
}

// newReadableBip44WalletNew creates a readable bip44 wallet
func newReadableBip44WalletNew(w *Bip44WalletNew) *readableBip44WalletNew {
	return &readableBip44WalletNew{
		Meta:     w.Meta.clone(),
		Accounts: *newReadableBip44Accounts(w.accounts.(*bip44Accounts)),
	}
}

// toWallet converts the readable bip44 wallet to a bip44 wallet
func (rw readableBip44WalletNew) toWallet() (*Bip44WalletNew, error) {
	accounts, err := rw.Accounts.toBip44Accounts()
	if err != nil {
		return nil, err
	}

	w := Bip44WalletNew{
		Meta:     rw.Meta.clone(),
		accounts: accounts,
	}
	return &w, nil
}

// readableBip44Accounts is the JSON representation of accounts
type readableBip44Accounts struct {
	Accounts []*readableBip44Account `json:"accounts"`
}

// ToBip44Accounts converts readable bip44 accounts to bip44 accounts
func (ras readableBip44Accounts) toBip44Accounts() (*bip44Accounts, error) {
	as := bip44Accounts{}
	for _, ra := range ras.Accounts {
		a := bip44Account{
			Name:     ra.Name,
			Index:    ra.Index,
			CoinType: CoinType(ra.CoinType),
		}

		// decode private key if not empty
		if ra.PrivateKey != "" {
			key, err := bip32.DeserializeEncodedPrivateKey(ra.PrivateKey)
			if err != nil {
				return nil, err
			}
			a.Account.Identifier()
			a.PrivateKey = key
		}

		for _, rc := range ra.Chains {
			c, err := rc.toBip44Chain(a.CoinType)
			if err != nil {
				return nil, err
			}
			a.Chains = append(a.Chains, *c)
		}

		as.accounts = append(as.accounts, &a)
	}

	return &as, nil
}

// ReadableBip44Account is the JSON representation of account
type readableBip44Account struct {
	PrivateKey string               `json:"private_key,omitempty"`
	Name       string               `json:"name"`      // Account name
	Index      uint32               `json:"index"`     // Account index
	CoinType   string               `json:"coin_type"` // Account coin type, determins the way to generate addresses
	Chains     []readableBip44Chain `json:"chains"`    // Chains, external chain with index value of 0, and internal(change) chain with index value of 1.
}

// ReadableBip44Chain bip44 chain with JSON tags
type readableBip44Chain struct {
	PubKey  string               `json:"public_key"`
	Entries readableBip44Entries `json:"entries"`
}

func (rc readableBip44Chain) toBip44Chain(coinType CoinType) (*bip44Chain, error) {
	pubkey, err := bip32.DeserializeEncodedPublicKey(rc.PubKey)
	if err != nil {
		return nil, err
	}

	c := bip44Chain{
		PubKey: *pubkey,
	}

	for _, re := range rc.Entries.Entries {
		e, err := newBip44EntryFromReadable(re, coinType)
		if err != nil {
			return nil, err
		}
		c.Entries = append(c.Entries, *e)
	}
	return &c, nil
}

func newBip44EntryFromReadable(re readableBip44Entry, coinType CoinType) (*Entry, error) {
	ca := resolveCoinAdapter(coinType)
	addr, err := ca.DecodeBase58Address(re.Address)
	if err != nil {
		return nil, err
	}

	p, err := cipher.PubKeyFromHex(re.Public)
	if err != nil {
		return nil, err
	}

	secKey, err := ca.SecKeyFromHex(re.Secret)
	if err != nil {
		return nil, err
	}

	return &Entry{
		Address:     addr,
		Public:      p,
		Secret:      secKey,
		ChildNumber: re.ChildNumber,
	}, nil
}

// readableBip44Entries wraps the slice of ReadableBip44Entry
type readableBip44Entries struct {
	Entries []readableBip44Entry
}

// ReadableBip44Entry bip44 entry with JSON tags
type readableBip44Entry struct {
	Address     string `json:"address"`
	Public      string `json:"public"`
	Secret      string `json:"secret"`
	ChildNumber uint32 `json:"child_number"` // For bip32/bip44
}

// newReadableBip44Accounts converts bip44Accounts to ReadableBip44Accounts
func newReadableBip44Accounts(as *bip44Accounts) *readableBip44Accounts {
	var ras readableBip44Accounts
	for _, a := range as.accounts {
		ras.Accounts = append(ras.Accounts, &readableBip44Account{
			PrivateKey: a.Account.String(),
			Name:       a.Name,
			Index:      a.Index,
			CoinType:   string(a.CoinType),
			Chains:     newReadableBip44Chains(a.Chains, a.CoinType),
		})
	}

	return &ras
}

func newReadableBip44Chains(cs []bip44Chain, coinType CoinType) []readableBip44Chain {
	ca := resolveCoinAdapter(coinType)
	var rcs []readableBip44Chain
	for _, c := range cs {
		rc := readableBip44Chain{
			PubKey: c.PubKey.String(),
		}
		for _, e := range c.Entries {
			rc.Entries.Entries = append(rc.Entries.Entries, readableBip44Entry{
				Address:     e.Address.String(),
				Public:      e.Public.Hex(),
				Secret:      ca.SecKeyToHex(e.Secret),
				ChildNumber: e.ChildNumber,
			})
		}
		rcs = append(rcs, rc)
	}

	return rcs
}
