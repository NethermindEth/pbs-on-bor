package engine

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

type BorExecutionPayloadResponse struct {
	Version string            `json:"version"`
	Data    BorExecutableData `json:"data"`
}

// BorExecutableData is the intermediate data type for the builder's payload unmarshalling
type BorExecutableData struct {
	ParentHash    common.Hash         `json:"parent_hash"`
	FeeRecipient  common.Address      `json:"fee_recipient"`
	StateRoot     common.Hash         `json:"state_root"`
	ReceiptsRoot  common.Hash         `json:"receipts_root"`
	LogsBloom     string              `json:"logs_bloom"`
	Random        common.Hash         `json:"prev_randao"`
	Number        uint64              `json:"block_number,string"`
	GasLimit      uint64              `json:"gas_limit,string"`
	GasUsed       uint64              `json:"gas_used,string"`
	Timestamp     uint64              `json:"timestamp,string"`
	ExtraData     string              `json:"extra_data"`
	BaseFeePerGas uint64              `json:"base_fee_per_gas,string"`
	BlockHash     common.Hash         `json:"block_hash"`
	Transactions  [][]byte            `json:"transactions"`
	Withdrawals   []*types.Withdrawal `json:"withdrawals"`
}

func (bed BorExecutableData) ToExecutableData() (ExecutableData, error) {
	ed := ExecutableData{
		ParentHash:   bed.ParentHash,
		FeeRecipient: bed.FeeRecipient,
		StateRoot:    bed.StateRoot,
		ReceiptsRoot: bed.ReceiptsRoot,
		Random:       bed.Random,
		Number:       bed.Number,
		GasLimit:     bed.GasLimit,
		GasUsed:      bed.GasUsed,
		Timestamp:    bed.Timestamp,
		BlockHash:    bed.BlockHash,
		Transactions: bed.Transactions,
		Withdrawals:  bed.Withdrawals,
	}

	if len(bed.LogsBloom) > 0 {
		lb, err := hexutil.Decode(bed.LogsBloom)
		if err != nil {
			return ExecutableData{}, err
		}
		if len(lb) != 256 {
			return ExecutableData{}, fmt.Errorf("invalid logs bloom length: %d, expected 256", len(lb))
		}
		ed.LogsBloom = lb
	}

	if len(bed.ExtraData) > 0 {
		data, err := hexutil.Decode(bed.ExtraData)
		if err != nil {
			return ExecutableData{}, err
		}
		ed.ExtraData = data
	}

	ed.BaseFeePerGas = common.Big0
	if bed.BaseFeePerGas > 0 {
		ed.BaseFeePerGas = big.NewInt(int64(bed.BaseFeePerGas))
	}

	return ed, nil
}
