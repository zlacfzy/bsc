package vm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// Even an accidental cache opt-in must not bypass stateful dispatch.
type cacheableStatefulProbe struct {
	cas20StatefulBase
	calls byte
}

func (*cacheableStatefulProbe) Name() string { return "CacheableStatefulProbe" }

func (p *cacheableStatefulProbe) Cacheable() bool { return true }
func (p *cacheableStatefulProbe) RunStateful(_ *PrecompileContext, _ []byte) ([]byte, error) {
	p.calls++
	return []byte{p.calls}, nil
}

type cacheableTimestampProbe struct{ milliTimestamp }

func (*cacheableTimestampProbe) Cacheable() bool { return true }

func TestPrecompileCacheBypassesBSCContext(t *testing.T) {
	cache := NewPrecompileCache()
	evm := &EVM{precompileCache: cache}
	p := &cacheableStatefulProbe{}
	for want := byte(1); want <= 2; want++ {
		got, _, err := evm.runPrecompile(p, common.Address{}, common.Address{1}, nil, NewGasBudget(100000, 0), false, true, uint256.NewInt(0))
		if err != nil || len(got) != 1 || got[0] != want {
			t.Fatalf("stateful invocation %d: got %x, err %v", want, got, err)
		}
	}
	for _, timestamp := range []uint64{123001, 123002} {
		got, _, err := RunPrecompiledContract(nil, &cacheableTimestampProbe{}, common.Address{2}, nil, NewGasBudget(100000, 0), nil, params.Rules{}, cache, BlockContext{MilliTimestamp: timestamp})
		if err != nil || new(uint256.Int).SetBytes(got).Uint64() != timestamp {
			t.Fatalf("timestamp %d: got %x, err %v", timestamp, got, err)
		}
	}
}
