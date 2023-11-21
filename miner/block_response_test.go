package miner

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

const RespBody = "{\"version\":\"bellatrix\",\"data\":{\"parent_hash\":\"0xa7994feb8e121c80f46fd477fb01ca828f31b20b0b8616231440e46e7aac93dc\",\"fee_recipient\":\"0x0000000000000000000000000000000000000000\",\"state_root\":\"0x69df903f426582dab091a4788f66952b63d7abd3710186fe20b0c8e14345165c\",\"receipts_root\":\"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421\",\"logs_bloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"prev_randao\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"block_number\":\"6\",\"gas_limit\":\"10048916\",\"gas_used\":\"0\",\"timestamp\":\"1700555813\",\"extra_data\":\"0xd88301000483626f7288676f312e32302e368664617277696e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"base_fee_per_gas\":\"678934158\",\"block_hash\":\"0x0e94b50f191662a7da9b251f55c71b2bcc0fcb89479b3a86366de2810e6b8b74\",\"transactions\":[]}}\n"

func TestBlockDeserialization(t *testing.T) {
	const (
		expectedBlockNumber = uint64(6)
		expectedTimestamp   = uint64(1700555813)
		expectedBaseFee     = uint64(678934158)
	)
	var (
		expectedParentHash = common.HexToHash("0xa7994feb8e121c80f46fd477fb01ca828f31b20b0b8616231440e46e7aac93dc")
	)

	var response engine.BorExecutionPayloadResponse
	decoder := json.NewDecoder(bytes.NewReader([]byte(RespBody)))
	err := decoder.Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "bellatrix", response.Version)
	assert.Equal(t, expectedBlockNumber, response.Data.Number, "block number")
	assert.Equal(t, expectedTimestamp, response.Data.Timestamp, "timestamp")
	assert.Equal(t, expectedBaseFee, response.Data.BaseFeePerGas, "base fee per gas")

	assert.Equal(t, "0xd88301000483626f7288676f312e32302e368664617277696e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", response.Data.ExtraData, "extra data")

	// receiving payload from builder we need to unmarshal it to the intermediate type
	// and then convert it type to beacon.engine.ExecutableData
	ed, err := response.Data.ToExecutableData()
	assert.NoError(t, err)
	assert.Equal(t, ed.Number, expectedBlockNumber)
	assert.Equal(t, ed.ParentHash, expectedParentHash)

	// try to make it block
	block, err := engine.ExecutableDataToBlock(ed)
	assert.NoError(t, err)
	assert.Equal(t, block.Number().Uint64(), expectedBlockNumber)
	assert.Equal(t, block.ParentHash(), expectedParentHash)
}
