package builder

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type IBeaconClient interface {
	isValidator(pubkey PubkeyHex) bool
	getProposerForNextSlot(requestedSlot uint64) (PubkeyHex, error)
	SubscribeToPayloadAttributesEvents(payloadAttrC chan types.BuilderPayloadAttributes)
	Start() error
	Stop()
}

type testBeaconClient struct {
	validator *ValidatorPrivateData
	slot      uint64
}

func (b *testBeaconClient) Stop() {}

func (b *testBeaconClient) isValidator(pubkey PubkeyHex) bool {
	return true
}

func (b *testBeaconClient) getProposerForNextSlot(requestedSlot uint64) (PubkeyHex, error) {
	return PubkeyHex(hexutil.Encode(b.validator.Pk)), nil
}

func (b *testBeaconClient) SubscribeToPayloadAttributesEvents(payloadAttrC chan types.BuilderPayloadAttributes) {
}

func (b *testBeaconClient) Start() error { return nil }

type NilBeaconClient struct{}

func (b *NilBeaconClient) isValidator(pubkey PubkeyHex) bool {
	return false
}

func (b *NilBeaconClient) getProposerForNextSlot(requestedSlot uint64) (PubkeyHex, error) {
	return PubkeyHex(""), nil
}

func (b *NilBeaconClient) SubscribeToPayloadAttributesEvents(payloadAttrC chan types.BuilderPayloadAttributes) {
}

func (b *NilBeaconClient) Start() error { return nil }

func (b *NilBeaconClient) Stop() {}
