package miner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
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

func (bc *BuilderClient) GetBlock(number uint64, parentHash common.Hash) (*types.Block, error) {
	// /eth/v1/builder/block/:parent_hash
	part := fmt.Sprintf("/eth/v1/builder/block/%d/%s", number, parentHash.Hex())
	url := bc.baseURL.JoinPath(part)
	resp, err := bc.hc.Get(url.String())
	if err != nil {
		return nil, fmt.Errorf("endpoint error: %w [req: %s]", err, part)
	}
	if resp.StatusCode >= 400 {
		b, _ := httputil.DumpResponse(resp, true)
		return nil, fmt.Errorf("unsuccessful response from endpoint [req: %s]\n%s\n", part, string(b))
	}
	defer resp.Body.Close()

	response, err := unmarshalResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ExecutionPayloadResponse from builder: %w", err)
	}
	return response.getBlock()
}

func unmarshalResponse(body io.Reader) (*ExecutionPayloadResponse, error) {
	var response engine.BorExecutionPayloadResponse
	decoder := json.NewDecoder(body)
	err := decoder.Decode(&response)
	if err != nil {
		return nil, err
	}
	execData, err := response.Data.ToExecutableData()
	if err != nil {
		return nil, err
	}

	return &ExecutionPayloadResponse{
		Version: response.Version,
		Data:    execData,
	}, nil
}
