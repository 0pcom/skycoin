package collection

import (
	"fmt"
	"testing"

	"github.com/SkycoinProject/skycoin/src/cipher"
	"github.com/SkycoinProject/skycoin/src/wallet"
	"github.com/SkycoinProject/skycoin/src/wallet/crypto"
	"github.com/stretchr/testify/require"
)

var (
	testSeed           = "test123"
	testSkycoinEntries = skycoinEntries([]readableEntry{
		{
			Address: "B4B6Hx1a3WPUHP323Bhqydifeu8TS4Zfan",
			Public:  "0251a81011b0b766242fb3d6777ae1f62e490e73e0c66ed25cbbb45421fa476356",
			Secret:  "91570790c29faa5ecfea981fdcb4bbb81280309f3f17dec4bad6e7697e126410",
		},
		{
			Address: "2FgDYaVqoR3DusaUmuin6xYnSW2FKpCcRrX",
			Public:  "028fcc354cb75dc2041ad2f938f5cd8453f6b799c6550a6f78aef214fa9b13721e",
			Secret:  "df18db782378605e9e5cbff9c845af6139294a114ba91ae530ad1bd20738c9e2",
		},
		{
			Address: "2L8awjtwfe1pMbkHKB9zZdeHYmSNAB1Krug",
			Public:  "021d175a6e13e58d5223d6d0d517eb66e6f0802674b762a4430b84b0349d79cbfb",
			Secret:  "1ad93991934205910301a773cf4f747a9a1fb2f86ec0478d5bb7b8296da9df8e",
		},
		{
			Address: "2VgCCNKj3TXFzddZSrSvbgHREdBB51Em4pN",
			Public:  "024290c4f4c6c2b975af998345564e069d45da9bbd814502600dab23fb38517110",
			Secret:  "cde8f59cf5d052e0c9594493caef8779179ee902870af28ef0e3fcd311774d99",
		},
		{
			Address: "7fqATQGPa6x3Qb7uakFWJM5xqSv3y8wtA1",
			Public:  "0399558a5cfd5b439175776252ec4f01287eaf9a93a791d3f0f12cb098453059a9",
			Secret:  "9238164770d2d7bd9dc1b9f303522b46f08c2b5db175ef7b1f754be9f4439725",
		},
	})
)

func TestNewWallet(t *testing.T) {
	type expect struct {
		meta map[string]string
		err  error
	}

	tt := []struct {
		name    string
		wltName string
		label   string
		opts    []wallet.Option
		expect  expect
	}{
		{
			name:    "ok all defaults",
			wltName: "test.wlt",
			label:   "",
			expect: expect{
				meta: map[string]string{
					"label":    "",
					"filename": "test.wlt",
					"coin":     string(wallet.CoinTypeSkycoin),
					"type":     wallet.WalletTypeCollection,
					"version":  wallet.Version,
				},
				err: nil,
			},
		},
		{
			name:    "ok with label,and coin set, collection",
			wltName: "test.wlt",
			label:   "test",
			opts: []wallet.Option{
				wallet.OptionCoinType(wallet.CoinTypeBitcoin),
			},
			expect: expect{
				meta: map[string]string{
					"label":    "test",
					"filename": "test.wlt",
					"coin":     string(wallet.CoinTypeBitcoin),
					"type":     wallet.WalletTypeCollection,
				},
				err: nil,
			},
		},
		{
			name:    "ok default crypto type, collection",
			wltName: "test.wlt",
			label:   "test",
			opts: []wallet.Option{
				wallet.OptionEncrypt(true),
				wallet.OptionPassword([]byte("pwd")),
			},
			expect: expect{
				meta: map[string]string{
					"label":     "test",
					"coin":      string(wallet.CoinTypeSkycoin),
					"type":      wallet.WalletTypeCollection,
					"encrypted": "true",
				},
				err: nil,
			},
		},
		{
			name:    "encrypt without password, collection",
			wltName: "test.wlt",
			label:   "wallet1",
			opts: []wallet.Option{
				wallet.OptionEncrypt(true),
			},
			expect: expect{
				meta: map[string]string{
					"label":     "wallet1",
					"coin":      string(wallet.CoinTypeSkycoin),
					"type":      wallet.WalletTypeCollection,
					"encrypted": "true",
				},
				err: wallet.ErrMissingPassword,
			},
		},
		{
			name:    "password=pwd encrypt=false",
			wltName: "test.wlt",
			label:   "test",
			opts: []wallet.Option{
				wallet.OptionEncrypt(false),
				wallet.OptionPassword([]byte("pwd")),
			},
			expect: expect{
				err: wallet.ErrMissingEncrypt,
			},
		},
	}

	for _, tc := range tt {
		// test all supported crypto types
		for _, ct := range crypto.TypesInsecure() {
			name := fmt.Sprintf("%v crypto=%v", tc.name, ct)
			opts := tc.opts
			opts = append(opts, wallet.OptionCryptoType(ct))

			t.Run(name, func(t *testing.T) {
				w, err := NewWallet(tc.wltName, tc.label, opts...)
				require.Equal(t, tc.expect.err, err, fmt.Sprintf("want:%v get:%v", tc.expect.err, err))
				if err != nil {
					return
				}

				// require.Equal(t, tc.opts.Encrypt, w.IsEncrypted())
				// confirms the meta data
				for k, v := range tc.expect.meta {
					require.Equal(t, v, w.Meta[k])
				}

				if w.IsEncrypted() {
					// Confirms the seeds and entry secrets are all empty
					require.Equal(t, "", w.Seed())
					require.Equal(t, "", w.LastSeed())
					entries, err := w.GetEntries()
					require.NoError(t, err)

					for _, e := range entries {
						require.True(t, e.Secret.Null())
					}

					// Confirms that secrets field is not empty
					require.NotEmpty(t, w.Secrets())
				}
			})
		}
	}

}

func TestWalletLock(t *testing.T) {
	tt := []struct {
		name    string
		wltName string
		opts    []wallet.Option
		lockPwd []byte
		err     error
	}{
		{
			name:    "ok",
			lockPwd: []byte("pwd"),
		},
		{
			name:    "password is nil",
			lockPwd: nil,
			err:     wallet.ErrMissingPassword,
		},
		{
			name: "wallet already encrypted",
			opts: []wallet.Option{
				wallet.OptionEncrypt(true),
				wallet.OptionPassword([]byte("pwd")),
			},
			lockPwd: []byte("pwd"),
			err:     wallet.ErrWalletEncrypted,
		},
	}

	for _, tc := range tt {
		for _, ct := range crypto.TypesInsecure() {
			name := fmt.Sprintf("%v crypto=%v", tc.name, ct)
			opts := tc.opts
			opts = append(opts, wallet.OptionCryptoType(ct))

			t.Run(name, func(t *testing.T) {
				wltName := wallet.NewWalletFilename()
				w, err := NewWallet(wltName, "test", opts...)
				require.NoError(t, err)

				// add entries
				for _, e := range testSkycoinEntries {
					w.AddEntry(e)
				}

				err = w.Lock(tc.lockPwd)
				require.Equal(t, tc.err, err)
				if err != nil {
					return
				}

				require.True(t, w.IsEncrypted())

				// Checks if the entries are encrypted
				entries, err := w.GetEntries()
				require.NoError(t, err)

				for _, e := range entries {
					require.Equal(t, cipher.SecKey{}, e.Secret)
				}
			})

		}
	}

}

// func TestWalletUnlock(t *testing.T) {
// 	tt := []struct {
// 		name      string
// 		opts      []wallet.Option
// 		unlockPwd []byte
// 		err       error
// 	}{
// 		{
// 			name: "ok",
// 			opts: []wallet.Option{
// 				wallet.OptionEncrypt(true),
// 				wallet.OptionPassword([]byte("pwd")),
// 			},
// 			unlockPwd: []byte("pwd"),
// 		},
// 		{
// 			name: "unlock with nil password",
// 			opts: []wallet.Option{
// 				wallet.OptionEncrypt(true),
// 				wallet.OptionPassword([]byte("pwd")),
// 			},
// 			unlockPwd: nil,
// 			err:       wallet.ErrMissingPassword,
// 		},
// 		{
// 			name: "unlock with wrong password",
// 			opts: []wallet.Option{
// 				wallet.OptionEncrypt(true),
// 				wallet.OptionPassword([]byte("pwd")),
// 			},
// 			unlockPwd: []byte("wrong_pwd"),
// 			err:       wallet.ErrInvalidPassword,
// 		},
// 		{
// 			name:      "unlock undecrypted wallet",
// 			unlockPwd: []byte("pwd"),
// 			err:       wallet.ErrWalletNotEncrypted,
// 		},
// 	}

// 	for _, tc := range tt {
// 		for _, ct := range crypto.TypesInsecure() {
// 			name := fmt.Sprintf("%v crypto=%v", tc.name, ct)

// 			opts := tc.opts
// 			opts = append(opts, wallet.OptionCryptoType(ct))

// 			t.Run(name, func(t *testing.T) {
// 				w, err := NewWallet("test.wlt", "test", opts...)
// 				require.NoError(t, err)
// 				// Tests the unlock method
// 				wlt, err := w.Unlock(tc.unlockPwd)
// 				require.Equal(t, tc.err, err)
// 				if err != nil {
// 					return
// 				}

// 				require.False(t, wlt.IsEncrypted())

// 				// Checks the seeds
// 				require.Equal(t, "testseed123", wlt.Seed())

// 				// Checks the generated addresses
// 				el, err := wlt.EntriesLen()
// 				require.NoError(t, err)
// 				require.Equal(t, 1, el)

// 				sd, sks := cipher.MustGenerateDeterministicKeyPairsSeed([]byte(wlt.Seed()), 1)
// 				require.Equal(t, hex.EncodeToString(sd), wlt.LastSeed())
// 				entries, err := wlt.GetEntries()
// 				require.NoError(t, err)
// 				for i, e := range entries {
// 					addr := cipher.MustAddressFromSecKey(sks[i])
// 					require.Equal(t, addr, e.Address)
// 				}

// 				// Checks the original seeds
// 				require.NotEqual(t, "testseed123", w.Seed())

// 				// Checks if the seckeys in entries of original wallet are empty
// 				entries, err = w.GetEntries()
// 				require.NoError(t, err)
// 				for _, e := range entries {
// 					require.True(t, e.Secret.Null())
// 				}

// 				// Checks if the seed and lastSeed in original wallet are still empty
// 				require.Empty(t, w.Seed())
// 				require.Empty(t, w.LastSeed())
// 			})
// 		}
// 	}
// }

func skycoinEntries(es []readableEntry) []wallet.Entry {
	entries := make([]wallet.Entry, len(es))
	for i, e := range es {
		pk, err := cipher.PubKeyFromHex(e.Public)
		if err != nil {
			panic(err)
		}
		sk, err := cipher.SecKeyFromHex(e.Secret)
		if err != nil {
			panic(err)
		}

		entries[i] = wallet.Entry{
			Address: cipher.MustDecodeBase58Address(e.Address),
			Public:  pk,
			Secret:  sk,
		}
	}

	return entries
}
