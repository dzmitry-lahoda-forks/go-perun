// Copyright 2020 - See NOTICE file for copyright holders.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client_test

import (
	"context"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/apps/payment"
	chtest "perun.network/go-perun/channel/test"
	"perun.network/go-perun/client"
	ctest "perun.network/go-perun/client/test"
	"perun.network/go-perun/wallet"
	wtest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"
	wiretest "perun.network/go-perun/wire/test"
	"polycry.pt/poly-go/test"
)

const (
	roleOperationTimeout = 1 * time.Second
	twoPartyTestTimeout  = 10 * time.Second
)

func NewSetups(rng *rand.Rand, names []string) []ctest.RoleSetup {
	var (
		bus     = wiretest.NewSerializingLocalBus()
		n       = len(names)
		setup   = make([]ctest.RoleSetup, n)
		backend = ctest.NewMockBackend(rng, "1337")
	)

	for i := 0; i < n; i++ {
		watcher, err := local.NewWatcher(backend)
		if err != nil {
			panic("Error initializing watcher: " + err.Error())
		}
		setup[i] = ctest.RoleSetup{
			Name:              names[i],
			Identity:          wiretest.NewRandomAccount(rng),
			Bus:               bus,
			Funder:            backend,
			Adjudicator:       backend,
			Watcher:           watcher,
			Wallet:            wtest.NewWallet(),
			Timeout:           roleOperationTimeout,
			BalanceReader:     backend,
			ChallengeDuration: 60,
			Errors:            make(chan error),
		}
	}

	return setup
}

type Client struct {
	*client.Client
	ctest.RoleSetup
	WalletAddress wallet.Address
}

func NewClients(t *testing.T, rng *rand.Rand, setups []ctest.RoleSetup) []*Client {
	t.Helper()
	clients := make([]*Client, len(setups))
	for i, setup := range setups {
		setup.Identity = wiretest.NewRandomAccount(rng)
		cl, err := client.New(setup.Identity.Address(), setup.Bus, setup.Funder, setup.Adjudicator, setup.Wallet, setup.Watcher)
		assert.NoError(t, err)
		clients[i] = &Client{
			Client:        cl,
			RoleSetup:     setup,
			WalletAddress: setup.Wallet.NewRandomAccount(rng).Address(),
		}
	}
	return clients
}

func runAliceBobTest(ctx context.Context, t *testing.T, setup func(*rand.Rand) ([]ctest.RoleSetup, [2]ctest.Executer)) {
	t.Helper()
	rng := test.Prng(t)
	for i := 0; i < 2; i++ {
		setups, roles := setup(rng)
		app := client.WithoutApp()
		if i == 1 {
			app = client.WithApp(
				chtest.NewRandomAppAndData(rng, chtest.WithAppRandomizer(new(payment.Randomizer))),
			)
		}

		cfg := &ctest.AliceBobExecConfig{
			BaseExecConfig: ctest.MakeBaseExecConfig(
				[2]wire.Address{setups[0].Identity.Address(), setups[1].Identity.Address()},
				chtest.NewRandomAsset(rng),
				[2]*big.Int{big.NewInt(100), big.NewInt(100)},
				app,
			),
			NumPayments: [2]int{2, 2},
			TxAmounts:   [2]*big.Int{big.NewInt(5), big.NewInt(3)},
		}

		ctest.ExecuteTwoPartyTest(ctx, t, roles, cfg)
	}
}
