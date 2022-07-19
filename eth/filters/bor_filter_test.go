package filters

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	gomock "github.com/golang/mock/gomock"
)

var (
	key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr    = crypto.PubkeyToAddress(key1.PublicKey)

	hash1 = common.BytesToHash([]byte("topic1"))
	hash2 = common.BytesToHash([]byte("topic2"))
	hash3 = common.BytesToHash([]byte("topic3"))
	hash4 = common.BytesToHash([]byte("topic4"))
)

func newTestHeader(blockNumber uint) *types.Header {
	head := types.Header{
		Number: big.NewInt(int64(blockNumber)),
	}
	return &head
}

func newTestReceipt(contractAddr common.Address, topicAddress common.Hash) *types.Receipt {
	receipt := types.NewReceipt(nil, false, 0)
	receipt.Logs = []*types.Log{
		{
			Address: contractAddr,
			Topics:  []common.Hash{topicAddress},
		},
	}

	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	return receipt
}

func (backend *MockBackend) expectBorReceiptsFromMock(hashes []*common.Hash) {
	for i := range hashes {
		if hashes[i] == nil {
			backend.EXPECT().GetBorBlockReceipt(gomock.Any(), gomock.Any()).Return(nil, nil)
			continue
		}
		backend.EXPECT().GetBorBlockReceipt(gomock.Any(), gomock.Any()).Return(newTestReceipt(addr, *hashes[i]), nil)
	}
}

func TestBorFilters(t *testing.T) {
	t.Parallel()

	var (
		db = rawdb.NewMemoryDatabase()

		sprint = params.TestChainConfig.Bor.Sprint
	)

	defer db.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	backend := NewMockBackend(ctrl)

	// should return the following at all times
	backend.EXPECT().ChainDb().Return(db).AnyTimes()
	backend.EXPECT().HeaderByNumber(gomock.Any(), gomock.Any()).Return(newTestHeader(1), nil).AnyTimes()

	// Block 1
	backend.expectBorReceiptsFromMock([]*common.Hash{nil, &hash1, &hash2, &hash3, &hash4})

	filter := NewBorBlockLogsRangeFilter(backend, sprint, 0, 18, []common.Address{addr}, [][]common.Hash{{hash1, hash2, hash3, hash4}})
	logs, err := filter.Logs(context.Background())
	if err != nil {
		t.Error(err)
	}
	if len(logs) != 4 {
		t.Error("expected 4 log, got", len(logs))
	}

	// Block 2
	backend.expectBorReceiptsFromMock([]*common.Hash{&hash1, &hash3})

	filter = NewBorBlockLogsRangeFilter(backend, sprint, 990, 999, []common.Address{addr}, [][]common.Hash{{hash3}})
	logs, _ = filter.Logs(context.Background())

	if len(logs) != 1 {
		t.Error("expected 1 log, got", len(logs))
	}

	if len(logs) > 0 && logs[0].Topics[0] != hash3 {
		t.Errorf("expected log[0].Topics[0] to be %x, got %x", hash3, logs[0].Topics[0])
	}

	// Block 3
	backend.expectBorReceiptsFromMock([]*common.Hash{&hash1, &hash2, &hash3})

	filter = NewBorBlockLogsRangeFilter(backend, sprint, 992, 1000, []common.Address{addr}, [][]common.Hash{{hash3}})
	logs, _ = filter.Logs(context.Background())

	if len(logs) != 1 {
		t.Error("expected 1 log, got", len(logs))
	}

	if len(logs) > 0 && logs[0].Topics[0] != hash3 {
		t.Errorf("expected log[0].Topics[0] to be %x, got %x", hash3, logs[0].Topics[0])
	}

	// Block 4
	backend.expectBorReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3})

	filter = NewBorBlockLogsRangeFilter(backend, sprint, 1, 16, []common.Address{addr}, [][]common.Hash{{hash1, hash2}})

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 2 {
		t.Error("expected 2 log, got", len(logs))
	}

	// Block 5
	backend.expectBorReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3, &hash4, nil})

	failHash := common.BytesToHash([]byte("fail"))
	filter = NewBorBlockLogsRangeFilter(backend, sprint, 0, 20, nil, [][]common.Hash{{failHash}})

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 0 {
		t.Error("expected 0 log, got", len(logs))
	}

	// Block 6
	backend.expectBorReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3, &hash4, nil})

	failAddr := common.BytesToAddress([]byte("failmenow"))
	filter = NewBorBlockLogsRangeFilter(backend, sprint, 0, 20, []common.Address{failAddr}, nil)

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 0 {
		t.Error("expected 0 log, got", len(logs))
	}

	// Block 7
	backend.expectBorReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3, &hash4, nil})

	filter = NewBorBlockLogsRangeFilter(backend, sprint, 0, 20, nil, [][]common.Hash{{failHash}, {hash1}})

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 0 {
		t.Error("expected 0 log, got", len(logs))
	}
}
