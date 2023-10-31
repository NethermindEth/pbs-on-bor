// Copyright 2014 The go-ethereum Authors
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

// Package miner implements Ethereum block creation and mining.
package miner

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// Backend wraps all methods required for mining. Only full node is capable
// to offer all the functions here.
type Backend interface {
	BlockChain() *core.BlockChain
	TxPool() *txpool.TxPool
	PeerCount() int
}

type AlgoType int

const (
	ALGO_MEV_GETH AlgoType = iota
	ALGO_GREEDY
	ALGO_GREEDY_BUCKETS
	ALGO_GREEDY_MULTISNAP
	ALGO_GREEDY_BUCKETS_MULTISNAP
)

func (a AlgoType) String() string {
	switch a {
	case ALGO_GREEDY:
		return "greedy"
	case ALGO_GREEDY_MULTISNAP:
		return "greedy-multi-snap"
	case ALGO_MEV_GETH:
		return "mev-geth"
	case ALGO_GREEDY_BUCKETS:
		return "greedy-buckets"
	case ALGO_GREEDY_BUCKETS_MULTISNAP:
		return "greedy-buckets-multi-snap"
	default:
		return "unsupported"
	}
}

func AlgoTypeFlagToEnum(algoString string) (AlgoType, error) {
	switch strings.ToLower(algoString) {
	case ALGO_MEV_GETH.String():
		return ALGO_MEV_GETH, nil
	case ALGO_GREEDY_BUCKETS.String():
		return ALGO_GREEDY_BUCKETS, nil
	case ALGO_GREEDY.String():
		return ALGO_GREEDY, nil
	case ALGO_GREEDY_MULTISNAP.String():
		return ALGO_GREEDY_MULTISNAP, nil
	case ALGO_GREEDY_BUCKETS_MULTISNAP.String():
		return ALGO_GREEDY_BUCKETS_MULTISNAP, nil
	default:
		return ALGO_MEV_GETH, errors.New("algo not recognized")
	}
}

// Config is the configuration parameters of mining.
type Config struct {
	Etherbase                common.Address    `toml:",omitempty"` // Public address for block mining rewards
	Notify                   []string          `toml:",omitempty"` // HTTP URL list to be notified of new work packages (only useful in ethash).
	NotifyFull               bool              `toml:",omitempty"` // Notify with pending block headers instead of work packages
	ExtraData                hexutil.Bytes     `toml:",omitempty"` // Block extra data set by the miner
	GasFloor                 uint64            // Target gas floor for mined blocks.
	GasCeil                  uint64            // Target gas ceiling for mined blocks.
	GasPrice                 *big.Int          // Minimum gas price for mining a transaction
	AlgoType                 AlgoType          // Algorithm to use for block building
	Recommit                 time.Duration     // The time interval for miner to re-create mining work.
	Noverify                 bool              // Disable remote mining solution verification(only useful in ethash).
	CommitInterruptFlag      bool              // Interrupt commit when time is up ( default = true)
	BuilderTxSigningKey      *ecdsa.PrivateKey `toml:",omitempty"` // Signing key of builder coinbase to make transaction to validator
	MaxMergedBundles         int
	Blocklist                []common.Address `toml:",omitempty"`
	NewPayloadTimeout        time.Duration    // The maximum time allowance for creating a new payload
	PriceCutoffPercent       int              // Effective gas price cutoff % used for bucketing transactions by price (only useful in greedy-buckets AlgoType)
	DiscardRevertibleTxOnErr bool             // When enabled, if bundle revertible transaction has error on commit, builder will discard the transaction
}

// DefaultConfig contains default settings for miner.
var DefaultConfig = Config{
	GasCeil:  30000000,
	GasPrice: big.NewInt(params.GWei),

	// The default recommit time is chosen as two seconds since
	// consensus-layer usually will wait a half slot of time(6s)
	// for payload generation. It should be enough for Geth to
	// run 3 rounds.
	Recommit:           2 * time.Second,
	NewPayloadTimeout:  2 * time.Second,
	PriceCutoffPercent: defaultPriceCutoffPercent,
}

// Miner creates blocks and searches for proof-of-work values.
// nolint:staticcheck
type Miner struct {
	mux     *event.TypeMux
	eth     Backend
	engine  consensus.Engine
	exitCh  chan struct{}
	startCh chan struct{}
	stopCh  chan chan struct{}
	worker  *multiWorker

	wg sync.WaitGroup
}

func New(eth Backend, config *Config, chainConfig *params.ChainConfig, mux *event.TypeMux, engine consensus.Engine, isLocalBlock func(header *types.Header) bool) *Miner {
	if config.BuilderTxSigningKey == nil {
		key := os.Getenv("BUILDER_TX_SIGNING_KEY")
		if key, err := crypto.HexToECDSA(strings.TrimPrefix(key, "0x")); err != nil {
			log.Error("Error parsing builder signing key from env", "err", err)
		} else {
			config.BuilderTxSigningKey = key
		}
	}

	miner := &Miner{
		mux:     mux,
		eth:     eth,
		engine:  engine,
		exitCh:  make(chan struct{}),
		startCh: make(chan struct{}),
		stopCh:  make(chan chan struct{}),
		worker:  newMultiWorker(config, chainConfig, engine, eth, mux, isLocalBlock, true),
	}
	miner.wg.Add(1)

	go miner.update()

	return miner
}

func (miner *Miner) GetWorker() *worker {
	// TODO [pnowosie] where is it used?
	return miner.worker.workers[0] //??
}

// update keeps track of the downloader events. Please be aware that this is a one shot type of update loop.
// It's entered once and as soon as `Done` or `Failed` has been broadcasted the events are unregistered and
// the loop is exited. This to prevent a major security vuln where external parties can DOS you with blocks
// and halt your mining operation for as long as the DOS continues.
func (miner *Miner) update() {
	defer miner.wg.Done()

	events := miner.mux.Subscribe(downloader.StartEvent{}, downloader.DoneEvent{}, downloader.FailedEvent{})
	defer func() {
		if !events.Closed() {
			events.Unsubscribe()
		}
	}()

	shouldStart := false
	canStart := true
	dlEventCh := events.Chan()

	for {
		select {
		case ev := <-dlEventCh:
			if ev == nil {
				// Unsubscription done, stop listening
				dlEventCh = nil
				continue
			}

			switch ev.Data.(type) {
			case downloader.StartEvent:
				wasMining := miner.Mining()
				miner.worker.stop()

				canStart = false

				if wasMining {
					// Resume mining after sync was finished
					shouldStart = true

					log.Info("Mining aborted due to sync")
				}
			case downloader.FailedEvent:
				canStart = true

				if shouldStart {
					miner.worker.start()
				}
			case downloader.DoneEvent:
				canStart = true

				if shouldStart {
					miner.worker.start()
				}
				// Stop reacting to downloader events
				events.Unsubscribe()
			}
		case <-miner.startCh:
			if canStart {
				miner.worker.start()
			}

			shouldStart = true
		case ch := <-miner.stopCh:
			shouldStart = false

			miner.worker.stop()
			close(ch)
		case <-miner.exitCh:
			miner.worker.close()
			return
		}
	}
}

func (miner *Miner) Start() {
	miner.startCh <- struct{}{}
}

func (miner *Miner) Stop(ch chan struct{}) {
	miner.stopCh <- ch
}

func (miner *Miner) Close() {
	close(miner.exitCh)
	miner.wg.Wait()
}

func (miner *Miner) Mining() bool {
	return miner.worker.isRunning()
}

func (miner *Miner) Hashrate() uint64 {
	if pow, ok := miner.engine.(consensus.PoW); ok {
		return uint64(pow.Hashrate())
	}

	return 0
}

func (miner *Miner) SetExtra(extra []byte) error {
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra exceeds max length. %d > %v", len(extra), params.MaximumExtraDataSize)
	}

	miner.worker.setExtra(extra)

	return nil
}

// SetRecommitInterval sets the interval for sealing work resubmitting.
func (miner *Miner) SetRecommitInterval(interval time.Duration) {
	miner.worker.setRecommitInterval(interval)
}

// Pending returns the currently pending block and associated state.
func (miner *Miner) Pending() (*types.Block, *state.StateDB) {
	return miner.worker.regularWorker.pending()
}

// PendingBlock returns the currently pending block.
//
// Note, to access both the pending block and the pending state
// simultaneously, please use Pending(), as the pending state can
// change between multiple method calls
func (miner *Miner) PendingBlock() *types.Block {
	return miner.worker.regularWorker.pendingBlock()
}

// PendingBlockAndReceipts returns the currently pending block and corresponding receipts.
func (miner *Miner) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	return miner.worker.pendingBlockAndReceipts()
}

func (miner *Miner) SetEtherbase(addr common.Address) {
	miner.worker.setEtherbase(addr)
}

// SetGasCeil sets the gaslimit to strive for when mining blocks post 1559.
// For pre-1559 blocks, it sets the ceiling.
func (miner *Miner) SetGasCeil(ceil uint64) {
	miner.worker.setGasCeil(ceil)
}

// EnablePreseal turns on the preseal mining feature. It's enabled by default.
// Note this function shouldn't be exposed to API, it's unnecessary for users
// (miners) to actually know the underlying detail. It's only for outside project
// which uses this library.
func (miner *Miner) EnablePreseal() {
	miner.worker.enablePreseal()
}

// DisablePreseal turns off the preseal mining feature. It's necessary for some
// fake consensus engine which can seal blocks instantaneously.
// Note this function shouldn't be exposed to API, it's unnecessary for users
// (miners) to actually know the underlying detail. It's only for outside project
// which uses this library.
func (miner *Miner) DisablePreseal() {
	miner.worker.disablePreseal()
}

// SubscribePendingLogs starts delivering logs from pending transactions
// to the given channel.
func (miner *Miner) SubscribePendingLogs(ch chan<- []*types.Log) event.Subscription {
	return miner.worker.regularWorker.pendingLogsFeed.Subscribe(ch)
}

// Accepts the block, time at which orders were taken, bundles which were used to build the block and all bundles that were considered for the block
type BlockHookFn = func(*types.Block, *big.Int, time.Time, []types.SimulatedBundle, []types.SimulatedBundle, []types.UsedSBundle)

// BuildPayload builds the payload according to the provided parameters.
func (miner *Miner) BuildPayload(args *BuildPayloadArgs) (*Payload, error) {
	return miner.worker.buildPayload(args)
}
