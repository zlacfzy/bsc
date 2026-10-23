package blobpool

import (
	"bytes"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/billy"
	"github.com/holiman/uint256"
)

func TestBSCBlobV0PoolRoundTrip(t *testing.T) {
	config := *params.BSCChainConfig
	key, _ := crypto.GenerateKey()
	blob := makeUnsignedTxWithTestBlob(0, 1, 1000, 100, 0)
	blob.ChainID = uint256.MustFromBig(config.ChainID)
	proof, err := kzg4844.ComputeBlobProof(testBlobs[0], testBlobCommits[0])
	if err != nil {
		t.Fatal(err)
	}
	blob.Sidecar = types.NewBlobTxSidecar(types.BlobSidecarVersion0, []kzg4844.Blob{*testBlobs[0]}, []kzg4844.Commitment{testBlobCommits[0]}, []kzg4844.Proof{proof})
	tx := types.MustSignNewTx(key, types.LatestSigner(&config), blob)
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	statedb.SetBalance(crypto.PubkeyToAddress(key.PublicKey), uint256.MustFromBig(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)), tracing.BalanceChangeUnspecified)
	chain := &testBlockChain{config: &config, basefee: uint256.NewInt(1), blobfee: uint256.NewInt(1), statedb: statedb}
	pool := New(Config{Datadir: "", Datacap: 10_000_000}, chain, nil)
	if err := pool.Init(1, chain.CurrentBlock(), newReserver()); err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Add([]*types.Transaction{tx}, true)[0]; err != nil {
		t.Fatalf("v0 admission: %v", err)
	}
	ptx, err := newBlobTxForPool(tx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ptx.toTx()
	if err != nil {
		t.Fatal(err)
	}
	wantRLP, _ := rlp.EncodeToBytes(tx)
	gotRLP, _ := rlp.EncodeToBytes(got)
	if !bytes.Equal(gotRLP, wantRLP) {
		t.Fatal("storage round trip changed v0 sidecar")
	}
	stored, _ := rlp.EncodeToBytes(ptx)
	for _, version := range []uint{69, 70, 71} {
		encoded, err := encodeForNetwork(stored, version)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, wantRLP) {
			t.Fatalf("eth/%d changed v0 encoding", version)
		}
	}
	if ptx.txSize() != tx.Size() {
		t.Fatalf("announcement size: got %d want %d", ptx.txSize(), tx.Size())
	}
	bad := *ptx.CellSidecar
	bad.Proofs = append([]kzg4844.Proof(nil), bad.Proofs...)
	bad.Proofs[0][0] ^= 1
	if err := pool.AddPooledTx(&BlobTxForPool{Tx: ptx.Tx, CellSidecar: &bad}); err == nil {
		t.Fatal("cell ingress accepted invalid v0 proof")
	}
	bad = *ptx.CellSidecar
	bad.Version = types.BlobSidecarVersion1
	if err := pool.AddPooledTx(&BlobTxForPool{Tx: ptx.Tx, CellSidecar: &bad}); err == nil {
		t.Fatal("cell ingress accepted v1 on BSC")
	}
	t.Run("legacy disk migration and restart", func(t *testing.T) {
		storage := t.TempDir()
		queuedir := filepath.Join(storage, pendingTransactionStore)
		if err := os.MkdirAll(queuedir, 0700); err != nil {
			t.Fatal(err)
		}
		store, err := billy.Open(billy.Options{Path: queuedir}, newSlotterEIP7594(params.BlobTxMaxBlobs), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Put(wantRLP); err != nil {
			store.Close()
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			diskPool := New(Config{Datadir: storage, Datacap: 10_000_000}, chain, nil)
			if err := diskPool.Init(1, chain.CurrentBlock(), newReserver()); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(10 * time.Second)
			for diskPool.Get(tx.Hash()) == nil && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			got := diskPool.Get(tx.Hash())
			if got == nil {
				diskPool.Close()
				t.Fatalf("attempt %d: lost legacy BSC blob transaction", attempt)
			}
			encoded, err := rlp.EncodeToBytes(got)
			if err != nil || !bytes.Equal(encoded, wantRLP) {
				diskPool.Close()
				t.Fatalf("attempt %d: v0 sidecar changed: %v", attempt, err)
			}
			if err := diskPool.Close(); err != nil {
				t.Fatal(err)
			}
		}
	})

}
