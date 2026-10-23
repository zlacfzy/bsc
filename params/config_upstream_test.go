// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params/forks"
)

func TestBlobConfigChainSpecificSchedule(t *testing.T) {
	prague := &BlobConfig{Target: 6, Max: 9, UpdateFraction: 5007716}
	osaka := &BlobConfig{Target: 3, Max: 6, UpdateFraction: 3338477}
	bpo := &BlobConfig{Target: 10, Max: 15, UpdateFraction: 8346193}
	cfg := ChainConfig{BlobScheduleConfig: &BlobScheduleConfig{
		Prague: prague, Osaka: osaka, BPO1: bpo,
	}}
	if got := cfg.BlobConfig(forks.Osaka); got != prague {
		t.Fatalf("Ethereum Osaka: got %v, want inherited Prague %v", got, prague)
	}
	if got := cfg.BlobConfig(forks.Amsterdam); got != bpo {
		t.Fatalf("Ethereum Amsterdam: got %v, want inherited BPO1 %v", got, bpo)
	}
	cfg.Parlia = &ParliaConfig{}
	for _, fork := range []forks.Fork{forks.Osaka, forks.Mendel, forks.Pasteur, forks.Jenner} {
		if got := cfg.BlobConfig(fork); got != osaka {
			t.Errorf("BSC %v: got %v, want BSC Osaka %v", fork, got, osaka)
		}
	}
	if got := cfg.BlobConfig(forks.BPO1); got != bpo {
		t.Fatalf("BSC BPO1: got %v, want %v", got, bpo)
	}
	cfg.BlobScheduleConfig = nil
	if got := cfg.BlobConfig(forks.Osaka); got != nil {
		t.Fatalf("missing schedule: got %v, want nil", got)
	}
}

func TestActiveSystemContractsChainSpecificAmsterdam(t *testing.T) {
	cfg := ChainConfig{LondonBlock: big.NewInt(0), AmsterdamTime: newUint64(0)}
	ethContracts := cfg.ActiveSystemContracts(0)
	if ethContracts["BUILDER_DEPOSIT_CONTRACT_ADDRESS"] != BuilderDepositAddress ||
		ethContracts["BUILDER_EXIT_CONTRACT_ADDRESS"] != BuilderExitAddress {
		t.Fatal("Ethereum Amsterdam builder contracts are missing")
	}
	cfg.Parlia = &ParliaConfig{}
	bscContracts := cfg.ActiveSystemContracts(0)
	if len(bscContracts) != 1 || bscContracts["HISTORY_STORAGE_ADDRESS"] != HistoryStorageAddress {
		t.Fatalf("unexpected BSC system contracts: %v", bscContracts)
	}
}

func TestBSCBuiltinUpstreamForksRemainDisabled(t *testing.T) {
	for _, tc := range bscConfigs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cfg.AmsterdamTime != nil || tc.cfg.BogotaTime != nil {
				t.Fatal("upstream-only forks must remain unscheduled in built-in BSC configs")
			}
			rules := tc.cfg.Rules(new(big.Int).SetUint64(^uint64(0)), false, ^uint64(0))
			if !rules.IsInBSC || rules.IsAmsterdam || rules.IsBogota {
				t.Fatalf("unexpected built-in BSC fork activation: %+v", rules)
			}
		})
	}
}
