package xpubwallet

import (
	"errors"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/SkycoinProject/skycoin/src/cipher"
	"github.com/SkycoinProject/skycoin/src/wallet"
	"github.com/stretchr/testify/require"
)

var testXPub = "xpub6EMRsT95ntbCFRR2Z6WppnGss1SijAkarfKoRM8tft66tuJh2nt4aJi13S21hUCLZL4cbFBXgHuxipmsS7dj1DW1s4NRup3hzxWfqUdGYv7"

type fakeWalletDecoder struct{}

func (d fakeWalletDecoder) Encode(w wallet.Wallet) ([]byte, error) {
	return nil, nil
}

func (d fakeWalletDecoder) Decode(b []byte) (wallet.Wallet, error) {
	return nil, nil
}

var (
	testSkycoinAddresses = stringsToAddresses([]string{
		"2JBfeo6y6FQn2rCiuhdQ8F1E6bj6rpnHo5U",
		"28Wn9scn3wb5nkScHiTHgNmLjSUS3F2SqAj",
		"qHVbkuuzzxGE6p6CnLY1JxY9ifK1RxjoNS",
		"2WNKEdCvoR8Mv5a7J5bLeE9syq7vHSzACmk",
		"2Z1ZcRWwsyiRqTYLm6VJF914FAE8uhfgmkX",
	})
)

func stringsToAddresses(addrsStr []string) []cipher.Addresser {
	var addrs []cipher.Addresser
	for _, addr := range addrsStr {
		a := cipher.MustDecodeBase58Address(addr)
		addrs = append(addrs, a)
	}

	return addrs
}

func TestNewWallet(t *testing.T) {
	type expect struct {
		meta map[string]string
		err  error
	}

	tt := []struct {
		name    string
		wltName string
		label   string
		xpub    string
		opts    []wallet.Option
		expect  expect
	}{
		{
			name:    "ok all defaults",
			wltName: "test.wlt",
			label:   "test",
			xpub:    testXPub,
			expect: expect{
				meta: map[string]string{
					"label":    "test",
					"filename": "test.wlt",
					"coin":     string(wallet.CoinTypeSkycoin),
					"type":     wallet.WalletTypeXPub,
					"version":  wallet.Version,
				},
				err: nil,
			},
		},
		{
			name:    "ok with label, coin set, XPub",
			wltName: "test.wlt",
			label:   "test",
			xpub:    testXPub,
			opts: []wallet.Option{
				wallet.OptionCoinType(wallet.CoinTypeBitcoin),
			},
			expect: expect{
				meta: map[string]string{
					"label":    "test",
					"filename": "test.wlt",
					"coin":     string(wallet.CoinTypeBitcoin),
					"type":     wallet.WalletTypeXPub,
				},
				err: nil,
			},
		},
		{
			name:    "set decoder",
			wltName: "test.wlt",
			label:   "test",
			xpub:    testXPub,
			opts: []wallet.Option{
				wallet.OptionDecoder(&fakeWalletDecoder{}),
			},
			expect: expect{
				meta: map[string]string{
					"label":    "test",
					"filename": "test.wlt",
					"coin":     string(wallet.CoinTypeSkycoin),
					"type":     wallet.WalletTypeXPub,
				},
				err: nil,
			},
		},
		{
			name:  "missing filename",
			label: "test",
			xpub:  testXPub,
			expect: expect{
				err: fmt.Errorf("filename not set"),
			},
		},
		{
			name:    "invalid xpub",
			wltName: "test.wlt",
			label:   "test",
			xpub:    "invalid xpub string",
			expect: expect{
				err: errors.New("invalid xpub key: Invalid base58 character"),
			},
		},
	}

	for _, tc := range tt {
		// test all supported crypto types
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewWallet(tc.wltName, tc.label, tc.xpub, tc.opts...)
			require.Equal(t, tc.expect.err, err, fmt.Sprintf("expect: %v, got: %v", tc.expect.err, err))
			if err != nil {
				return
			}

			require.NotEmpty(t, w.Timestamp())
			require.NotNil(t, w.decoder)

			// confirms the meta data
			for k, v := range tc.expect.meta {
				require.Equal(t, v, w.Meta[k])
			}
		})
	}
}

func TestWalletGenerateAddresses(t *testing.T) {
	tt := []struct {
		name               string
		xpub               string
		opts               []wallet.Option
		num                uint64
		oneAddressEachTime bool
		expectAddrs        []cipher.Addresser
		err                error
	}{
		{
			name:        "ok with none address",
			xpub:        testXPub,
			num:         0,
			expectAddrs: []cipher.Addresser{},
		},
		{
			name:        "ok with one address",
			xpub:        testXPub,
			num:         1,
			expectAddrs: testSkycoinAddresses[:1],
		},
		{
			name:               "ok with three address and generate one address each time deterministic",
			xpub:               testXPub,
			num:                2,
			oneAddressEachTime: true,
			expectAddrs:        testSkycoinAddresses[:2],
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// create wallet
			w, err := NewWallet("test.wlt", "test", tc.xpub, tc.opts...)
			require.NoError(t, err)

			// generate address
			var addrs []cipher.Addresser
			if !tc.oneAddressEachTime {
				addrs, err = w.GenerateAddresses(tc.num)
				require.NoError(t, err)
				if err != nil {
					return
				}

			} else {
				for i := uint64(0); i < tc.num; i++ {
					addr, err := w.GenerateAddresses(1)
					require.Equal(t, tc.err, err)
					if err != nil {
						return
					}

					addrs = append(addrs, addr[0])
				}
			}

			require.Equal(t, len(tc.expectAddrs), len(addrs))
			for i, addr := range addrs {
				require.Equal(t, tc.expectAddrs[i], addr)
			}
		})
	}
}

func TestWalletGetEntry(t *testing.T) {
	tt := []struct {
		name    string
		address cipher.Addresser
		err     error
	}{
		{
			"ok",
			testSkycoinAddresses[0],
			nil,
		},
		{
			"entry not exist",
			testSkycoinAddresses[3],
			wallet.ErrEntryNotFound,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewWallet("test.wlt", "test", testXPub)
			require.NoError(t, err)
			_, err = w.GenerateAddresses(3)
			require.NoError(t, err)

			e, err := w.GetEntry(tc.address)
			require.Equal(t, tc.err, err, fmt.Sprintf("expect: %v, got: %v", tc.err, err))
			if err != nil {
				return
			}

			require.Equal(t, tc.address, e.Address)
		})
	}

}

func TestWalletSerialize(t *testing.T) {
	w, err := NewWallet("test.wlt", "test", testXPub)
	require.NoError(t, err)

	_, err = w.GenerateAddresses(5)
	require.NoError(t, err)

	w.SetTimestamp(0)
	b, err := w.Serialize()
	require.NoError(t, err)

	// load wallet file and compare
	fb, err := ioutil.ReadFile("./testdata/wallet_serialize.wlt")
	require.NoError(t, err)
	require.Equal(t, fb, b)

	wlt := Wallet{}
	err = wlt.Deserialize(b)
	require.NoError(t, err)
}

func TestWalletDeserialize(t *testing.T) {
	b, err := ioutil.ReadFile("./testdata/wallet_serialize.wlt")
	require.NoError(t, err)

	w := Wallet{}
	err = w.Deserialize(b)
	require.NoError(t, err)

	require.Equal(t, w.Filename(), "test.wlt")
	require.Equal(t, w.Label(), "test")
	entries, err := w.GetEntries()
	require.NoError(t, err)
	require.Equal(t, 5, len(entries))
	for i, e := range entries {
		require.Equal(t, testSkycoinAddresses[i], e.Address)
	}
	require.Equal(t, testXPub, w.XPub())
}
