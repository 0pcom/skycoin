package wallet

import (
	"errors"
	"fmt"

	"github.com/SkycoinProject/skycoin/src/cipher"
	"github.com/SkycoinProject/skycoin/src/cipher/bip32"
	"github.com/SkycoinProject/skycoin/src/cipher/bip39"
	"github.com/SkycoinProject/skycoin/src/cipher/bip44"
	"github.com/SkycoinProject/skycoin/src/util/mathutil"
)

// bip44Account records the bip44 wallet account info
type bip44Account struct {
	bip44.Account
	Name     string       // Account name
	Index    uint32       // Account index
	CoinType CoinType     // Account coin type, determins the way to generate addresses
	Chains   []bip44Chain // Chains, external chain with index value of 0, and internal(change) chain with index value of 1.
}

type bip44AccountCreateOptions struct {
	name           string
	index          uint32
	seed           string
	seedPassphrase string
	coinType       CoinType
}

func newBip44Account(opts bip44AccountCreateOptions) (*bip44Account, error) {
	// opts.seed must return a valid bip39 mnemonic
	seed, err := bip39.NewSeed(opts.seed, opts.seedPassphrase)
	if err != nil {
		return nil, err
	}

	ca := resolveCoinAdapter(opts.coinType)

	c, err := bip44.NewCoin(seed, ca.Bip44CoinType())
	if err != nil {
		logger.Critical().WithError(err).Error("Failed to derive the bip44 purpose node")
		if bip32.IsImpossibleChildError(err) {
			logger.Critical().Error("ImpossibleChild: this seed cannot be used for bip44")
		}
		return nil, err
	}
	a, err := c.Account(opts.index)
	if err != nil {
		return nil, err
	}

	externalChainKey, changeChainKey, err := makeChainPubKeys(a)
	if err != nil {
		return nil, err
	}

	ba := &bip44Account{
		Account:  *a,
		Name:     opts.name,
		Index:    opts.index,
		CoinType: opts.coinType,
	}

	// init the external chain
	ba.Chains = append(ba.Chains, bip44Chain{
		PubKey:      *externalChainKey,
		makeAddress: ca.AddressFromPubKey,
	})
	// init the change chain
	ba.Chains = append(ba.Chains, bip44Chain{
		PubKey:      *changeChainKey,
		makeAddress: ca.AddressFromPubKey,
	})
	return ba, nil
}

func (a *bip44Account) newAddresses(chainIndex, num uint32) ([]cipher.Addresser, error) {
	if a == nil {
		return nil, errors.New("cannot generate new addresses on nil account")
	}

	// chain index can only be 0 or 1.
	switch chainIndex {
	case bip44.ExternalChainIndex, bip44.ChangeChainIndex:
		return a.Chains[chainIndex].newAddresses(num, a.PrivateKey)
	default:
		return nil, fmt.Errorf("invalid chain index: %d", chainIndex)
	}
}

// bip44Chain bip44 address chain
type bip44Chain struct {
	PubKey      bip32.PublicKey
	Entries     Entries
	makeAddress func(key cipher.PubKey) cipher.Addresser
}

// newAddresses generates addresses on the chain.
// private key is optional, if not provided, addresses will be generated using the public key.
func (c *bip44Chain) newAddresses(num uint32, seckey *bip32.PrivateKey) ([]cipher.Addresser, error) {
	if c == nil {
		return nil, errors.New("cannot generate new addresses on nil chain")
	}

	var addrs []cipher.Addresser
	initLen := uint32(len(c.Entries))
	_, err := mathutil.AddUint32(initLen, num)
	if err != nil {
		return nil, fmt.Errorf("can not create %d more addresses, current addresses number %d, err: %v", num, initLen, err)
	}

	for i := uint32(0); i < num; i++ {
		index := initLen + i
		pk, err := c.PubKey.NewPublicChildKey(index)
		if err != nil {
			return nil, fmt.Errorf("bip44 chain generate address with index %d failed, err: %v", index, err)
		}
		cpk, err := cipher.NewPubKey(pk.Key)
		if err != nil {
			return nil, err
		}

		addr := c.makeAddress(cpk)
		e := Entry{
			Address:     addr,
			Public:      cpk,
			ChildNumber: index,
		}

		if seckey != nil {
			csk, err := cipher.NewSecKey(seckey.Key)
			if err != nil {
				return nil, err
			}
			e.Secret = csk
		}

		c.Entries = append(c.Entries, e)
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// bip44Accounts implementes the accountManager interface
type bip44Accounts struct {
	accounts []*bip44Account
}

func (a bip44Accounts) Len() uint32 {
	return uint32(len(a.accounts))
}

func (a *bip44Accounts) NewAddresses(index, chain, num uint32) ([]cipher.Addresser, error) {
	accountLen := len(a.accounts)
	if int(index) >= accountLen {
		return nil, fmt.Errorf("account index %d out of range", index)
	}

	account := a.accounts[index]
	if account == nil {
		return nil, fmt.Errorf("account of index %d not found", index)
	}

	return account.newAddresses(chain, num)
}

func (a *bip44Accounts) New(opts bip44AccountCreateOptions) (uint32, error) {
	accountIndex, err := a.nextIndex()
	if err != nil {
		return 0, err
	}

	// assign the account index
	opts.index = accountIndex

	// create a bip44 account
	ba, err := newBip44Account(opts)
	if err != nil {
		return 0, err
	}

	a.accounts = append(a.accounts, ba)
	return accountIndex, nil
}

func (a *bip44Accounts) nextIndex() (uint32, error) {
	// Try to get next account index, return error if the
	// account is full.
	if _, err := mathutil.AddUint32(uint32(len(a.accounts)), 1); err != nil {
		return 0, errors.New("Maximum bip44 account number reached")
	}

	return uint32(len(a.accounts)), nil
}

func (a *bip44Accounts) ToReadable() ReadableBip44Accounts {
	return *NewReadableBip44Accounts(a)
}

// ReadableBip44Accounts is the JSON representation of accounts
type ReadableBip44Accounts struct {
	Accounts []*ReadableBip44Account `json:"accounts"`
}

// ToBip44Accounts converts readable bip44 accounts to bip44 accounts
func (ras ReadableBip44Accounts) toBip44Accounts() (*bip44Accounts, error) {
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
type ReadableBip44Account struct {
	PrivateKey string               `json:"private_key,omitempty"`
	Name       string               `json:"name"`      // Account name
	Index      uint32               `json:"index"`     // Account index
	CoinType   string               `json:"coin_type"` // Account coin type, determins the way to generate addresses
	Chains     []ReadableBip44Chain `json:"chains"`    // Chains, external chain with index value of 0, and internal(change) chain with index value of 1.
}

// ReadableBip44Chain bip44 chain with JSON tags
type ReadableBip44Chain struct {
	PubKey  string               `json:"public_key"`
	Entries ReadableBip44Entries `json:"entries"`
}

func (rc ReadableBip44Chain) toBip44Chain(coinType CoinType) (*bip44Chain, error) {
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

func newBip44EntryFromReadable(re ReadableBip44Entry, coinType CoinType) (*Entry, error) {
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

// ReadableBip44Entries wraps the slice of ReadableBip44Entry
type ReadableBip44Entries struct {
	Entries []ReadableBip44Entry
}

// ReadableBip44Entry bip44 entry with JSON tags
type ReadableBip44Entry struct {
	Address     string `json:"address"`
	Public      string `json:"public"`
	Secret      string `json:"secret"`
	ChildNumber uint32 `json:"child_number"` // For bip32/bip44
}

// NewReadableBip44Accounts converts bip44Accounts to ReadableBip44Accounts
func NewReadableBip44Accounts(as *bip44Accounts) *ReadableBip44Accounts {
	var ras ReadableBip44Accounts
	for _, a := range as.accounts {
		ras.Accounts = append(ras.Accounts, &ReadableBip44Account{
			PrivateKey: a.Account.String(),
			Name:       a.Name,
			Index:      a.Index,
			CoinType:   string(a.CoinType),
			Chains:     newReadableBip44Chains(a.Chains, a.CoinType),
		})
	}

	return &ras
}

func newReadableBip44Chains(cs []bip44Chain, coinType CoinType) []ReadableBip44Chain {
	ca := resolveCoinAdapter(coinType)
	var rcs []ReadableBip44Chain
	for _, c := range cs {
		rc := ReadableBip44Chain{
			PubKey: c.PubKey.String(),
		}
		for _, e := range c.Entries {
			rc.Entries.Entries = append(rc.Entries.Entries, ReadableBip44Entry{
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
