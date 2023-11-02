package miner

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BuilderClient struct {
	hc      *http.Client
	baseURL *url.URL
	//key     *keystore.Key
}

func urlForHost(h string) (*url.URL, error) {
	// try to parse as url (being permissive)
	u, err := url.Parse(h)
	if err == nil && u.Host != "" {
		return u, nil
	}
	// try to parse as host:port
	host, port, err := net.SplitHostPort(h)
	if err != nil {
		return nil, errors.New("hostname must include port, separated by one colon, like example.com:3500")
	}
	return &url.URL{Host: net.JoinHostPort(host, port), Scheme: "http"}, nil
}

func NewBuilderClient(host string, timeout time.Duration) (*BuilderClient, error) {
	//key, err := keystore.Keystore{}.GetKey(keyfile, password)
	//if err != nil {
	//	return nil, err
	//}

	u, err := urlForHost(host)
	if err != nil {
		return nil, err
	}

	hc := &http.Client{Timeout: timeout}
	return &BuilderClient{
		hc:      hc,
		baseURL: u,
		//key:     key,
	}, nil
}

type ExecutionPayloadResponse struct {
	Version string                `json:"version"`
	Data    engine.ExecutableData `json:"data"`
}

type GetHeaderResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (res *ExecutionPayloadResponse) getBlock() (*types.Block, error) {
	return engine.ExecutableDataToBlock(res.Data)
}

//func createRegistration(feeRecipient string, gasLimit uint64, key *keystore.Key) (*builderApiV1.SignedValidatorRegistration, error) {
//	decoded, err := hexutil.Decode(feeRecipient)
//	if err != nil {
//		return nil, err
//	}
//	address := bellatrix.ExecutionAddress{}
//	n := copy(address[:], decoded)
//	if n != 20 {
//		return nil, errors.New("invalid fee recipient")
//	}
//
//	pubkey := phase0.BLSPubKey{}
//	copy(pubkey[:], key.PublicKey.Marshal())
//
//	message := &builderApiV1.ValidatorRegistration{
//		FeeRecipient: address,
//		GasLimit:     gasLimit,
//		Timestamp:    time.Now(),
//		Pubkey:       pubkey,
//	}
//
//	sk, err := bls.SecretKeyFromBytes(key.SecretKey.Marshal())
//	if err != nil {
//		return nil, err
//	}
//
//	// https://github.com/ethereum/builder-specs/blob/main/specs/bellatrix/builder.md#domain-types
//	domain := ssz.ComputeDomain(phase0.DomainType{0x00, 0x00, 0x00, 0x01}, phase0.Version{}, phase0.Root{})
//	signature, err := ssz.SignMessage(message, domain, sk)
//	if err != nil {
//		return nil, err
//	}
//
//	signed := &builderApiV1.SignedValidatorRegistration{
//		Message:   message,
//		Signature: signature,
//	}
//
//	return signed, nil
//}

//func (bc *BuilderClient) RegisterValidator(feeRecipient string, gasLimit uint64) error {
//	signedReg, err := createRegistration(feeRecipient, gasLimit, bc.key)
//	if err != nil {
//		return err
//	}
//
//	payload := []*builderApiV1.SignedValidatorRegistration{signedReg}
//	body, err := json.Marshal(payload)
//	if err != nil {
//		return err
//	}
//
//	url := bc.baseURL.JoinPath("/eth/v1/builder/validators")
//	_, err = bc.hc.Post(url.String(), "application/json", bytes.NewBuffer(body))
//	if err != nil {
//		return err
//	}
//	return nil
//}

//func (bc *BuilderClient) GetHeader(slot uint64, parentHash common.Hash) error {
//	part := fmt.Sprintf("/eth/v1/builder/header/%d/%s/%s", slot, parentHash.Hex(), hexutil.Encode(bc.key.PublicKey.Marshal()))
//	url := bc.baseURL.JoinPath(part)
//	resp, err := bc.hc.Get(url.String())
//	if err != nil {
//		return err
//	}
//	defer resp.Body.Close()
//	var response GetHeaderResponse
//	decoder := json.NewDecoder(resp.Body)
//	err = decoder.Decode(&response)
//	if err != nil {
//		return err
//	}
//	if response.Code != 200 {
//		return errors.New(response.Message)
//	}
//	return nil
//}

func (bc *BuilderClient) GetBlock(slot uint64, parentHash common.Hash) (*types.Block, error) {
	// /eth/v1/builder/block/:parent_hash
	part := fmt.Sprintf("/eth/v1/builder/block/%s", parentHash.Hex())
	url := bc.baseURL.JoinPath(part)
	resp, err := bc.hc.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var response ExecutionPayloadResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&response)
	if err != nil {
		return nil, err
	}
	return response.getBlock()
}
