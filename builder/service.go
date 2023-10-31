package builder

import (
	"errors"
	"fmt"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/eth"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	blockvalidation "github.com/ethereum/go-ethereum/eth/block-validation"
	"github.com/ethereum/go-ethereum/flashbotsextra"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/flashbots/go-boost-utils/bls"
	"github.com/flashbots/go-utils/httplogger"
	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
)

const (
	_PathStatus            = "/eth/v1/builder/status"
	_PathRegisterValidator = "/eth/v1/builder/validators"
	_PathGetHeader         = "/eth/v1/builder/header/{slot:[0-9]+}/{parent_hash:0x[a-fA-F0-9]+}/{pubkey:0x[a-fA-F0-9]+}"
	_PathGetPayload        = "/eth/v1/builder/blinded_blocks"
)

type Service struct {
	srv     *http.Server
	builder IBuilder
}

func (s *Service) Start() error {
	if s.srv != nil {
		log.Info("Service started")
		go s.srv.ListenAndServe()
	}

	s.builder.Start()

	return nil
}

func (s *Service) Stop() error {
	if s.srv != nil {
		s.srv.Close()
	}
	s.builder.Stop()
	return nil
}

func (s *Service) PayloadAttributes(payloadAttributes *types.BuilderPayloadAttributes) error {
	return s.builder.OnPayloadAttribute(payloadAttributes)
}

func getRouter(localRelay *LocalRelay) http.Handler {
	router := mux.NewRouter()

	// Add routes
	router.HandleFunc("/", localRelay.handleIndex).Methods(http.MethodGet)
	router.HandleFunc(_PathStatus, localRelay.handleStatus).Methods(http.MethodGet)
	router.HandleFunc(_PathRegisterValidator, localRelay.handleRegisterValidator).Methods(http.MethodPost)
	router.HandleFunc(_PathGetHeader, localRelay.handleGetHeader).Methods(http.MethodGet)
	router.HandleFunc(_PathGetPayload, localRelay.handleGetPayload).Methods(http.MethodPost)

	// Add logging and return router
	loggedRouter := httplogger.LoggingMiddleware(router)
	return loggedRouter
}

func getRelayConfig(endpoint string) (RelayConfig, error) {
	configs := strings.Split(endpoint, ";")
	if len(configs) == 0 {
		return RelayConfig{}, fmt.Errorf("empty relay endpoint %s", endpoint)
	}
	relayUrl := configs[0]
	// relay endpoint is configurated in the format URL;ssz=<value>;gzip=<value>
	// if any of them are missing, we default the config value to false
	var sszEnabled, gzipEnabled bool
	var err error

	for _, config := range configs {
		if strings.HasPrefix(config, "ssz=") {
			sszEnabled, err = strconv.ParseBool(config[4:])
			if err != nil {
				log.Info("invalid ssz config for relay", "endpoint", endpoint, "err", err)
			}
		} else if strings.HasPrefix(config, "gzip=") {
			gzipEnabled, err = strconv.ParseBool(config[5:])
			if err != nil {
				log.Info("invalid gzip config for relay", "endpoint", endpoint, "err", err)
			}
		}
	}
	return RelayConfig{
		Endpoint:    relayUrl,
		SszEnabled:  sszEnabled,
		GzipEnabled: gzipEnabled,
	}, nil
}

func NewService(listenAddr string, localRelay *LocalRelay, builder IBuilder) *Service {
	var srv *http.Server
	if localRelay != nil {
		srv = &http.Server{
			Addr:    listenAddr,
			Handler: getRouter(localRelay),
			/*
			   ReadTimeout:
			   ReadHeaderTimeout:
			   WriteTimeout:
			   IdleTimeout:
			*/
		}
	}

	return &Service{
		srv:     srv,
		builder: builder,
	}
}

func Register(stack *node.Node, backend *eth.Ethereum, cfg *Config) error {
	envBuilderSkBytes, err := hexutil.Decode(cfg.BuilderSecretKey)
	if err != nil {
		return errors.New("incorrect builder API secret key provided")
	}

	var beaconClient IBeaconClient = &NilBeaconClient{}

	var localRelay *LocalRelay
	builderSigningDomain, proposerSigningDomain := phase0.Domain{}, phase0.Domain{}

	if cfg.EnableLocalRelay {
		envRelaySkBytes, err := hexutil.Decode(cfg.RelaySecretKey)
		if err != nil {
			return errors.New("incorrect builder API secret key provided")
		}

		relaySk, err := bls.SecretKeyFromBytes(envRelaySkBytes[:])
		if err != nil {
			return errors.New("incorrect builder API secret key provided")
		}

		localRelay, err = NewLocalRelay(relaySk, beaconClient, builderSigningDomain, proposerSigningDomain, ForkData{cfg.GenesisForkVersion, cfg.BellatrixForkVersion, cfg.GenesisValidatorsRoot}, cfg.EnableValidatorChecks)
		if err != nil {
			return fmt.Errorf("failed to create local relay: %w", err)
		}
	}

	var relay IRelay
	if cfg.RemoteRelayEndpoint != "" {
		relayConfig, err := getRelayConfig(cfg.RemoteRelayEndpoint)
		if err != nil {
			return fmt.Errorf("invalid remote relay endpoint: %w", err)
		}
		relay = NewRemoteRelay(relayConfig, localRelay, cfg.EnableCancellations)
	} else if localRelay != nil {
		relay = localRelay
	} else {
		return errors.New("neither local nor remote relay specified")
	}

	if len(cfg.SecondaryRemoteRelayEndpoints) > 0 && !(len(cfg.SecondaryRemoteRelayEndpoints) == 1 && cfg.SecondaryRemoteRelayEndpoints[0] == "") {
		secondaryRelays := make([]IRelay, len(cfg.SecondaryRemoteRelayEndpoints))
		for i, endpoint := range cfg.SecondaryRemoteRelayEndpoints {
			relayConfig, err := getRelayConfig(endpoint)
			if err != nil {
				return fmt.Errorf("invalid secondary remote relay endpoint: %w", err)
			}
			secondaryRelays[i] = NewRemoteRelay(relayConfig, nil, cfg.EnableCancellations)
		}
		relay = NewRemoteRelayAggregator(relay, secondaryRelays)
	}

	var validator *blockvalidation.BlockValidationAPI
	if cfg.DryRun {
		var accessVerifier *blockvalidation.AccessVerifier
		if cfg.ValidationBlocklist != "" {
			accessVerifier, err = blockvalidation.NewAccessVerifierFromFile(cfg.ValidationBlocklist)
			if err != nil {
				return fmt.Errorf("failed to load validation blocklist %w", err)
			}
		}
		validator = blockvalidation.NewBlockValidationAPI(backend, accessVerifier, cfg.ValidationUseCoinbaseDiff)
	}

	// Set up builder rate limiter based on environment variables or CLI flags.
	// Builder rate limit parameters are flags.BuilderRateLimitDuration and flags.BuilderRateLimitMaxBurst
	duration, err := time.ParseDuration(cfg.BuilderRateLimitDuration)
	if err != nil {
		return fmt.Errorf("error parsing builder rate limit duration - %w", err)
	}

	// BuilderRateLimitMaxBurst is set to builder.RateLimitBurstDefault by default if not specified
	limiter := rate.NewLimiter(rate.Every(duration), cfg.BuilderRateLimitMaxBurst)

	var submissionOffset time.Duration
	if offset := cfg.BuilderSubmissionOffset; offset != 0 {
		if offset < 0 {
			return fmt.Errorf("builder submission offset must be positive")
		} else if uint64(offset.Seconds()) > cfg.SecondsInSlot {
			return fmt.Errorf("builder submission offset must be less than seconds in slot")
		}
		submissionOffset = offset
	} else {
		submissionOffset = SubmissionOffsetFromEndOfSlotSecondsDefault
	}

	// TODO: move to proper flags
	var ds flashbotsextra.IDatabaseService = flashbotsextra.NilDbService{}

	// Bundle fetcher
	// TODO: can I ignore BundleFetcher?
	//if !cfg.DisableBundleFetcher {
	//	mevBundleCh := make(chan []types.MevBundle)
	//	blockNumCh := make(chan int64)
	//	bundleFetcher := flashbotsextra.NewBundleFetcher(backend, ds, blockNumCh, mevBundleCh, true)
	//	backend.RegisterBundleFetcher(bundleFetcher)
	//	go bundleFetcher.Run()
	//}

	ethereumService := NewEthereumService(backend)

	builderSk, err := bls.SecretKeyFromBytes(envBuilderSkBytes[:])
	if err != nil {
		return errors.New("incorrect builder API secret key provided")
	}
	//proposerPubKey, err := bls.PublicKeyFromBytes(cfg.ProposerPubkey)

	builderArgs := CliqueBuilderArgs{
		builderSecretKey:              builderSk,
		ds:                            ds,
		eth:                           ethereumService,
		relay:                         relay,
		submissionOffsetFromEndOfSlot: submissionOffset,
		limiter:                       limiter,
		validator:                     validator,
		//proposerPubkey:                proposerPubKey,
		//builderSigningDomain:          builderSigningDomain,
		//builderBlockResubmitInterval:  builderRateLimitInterval,
	}

	builderBackend, err := NewCliqueBuilder(builderArgs)
	if err != nil {
		return fmt.Errorf("failed to create builder backend: %w", err)
	}
	builderService := NewService(cfg.ListenAddr, localRelay, builderBackend)

	stack.RegisterAPIs([]rpc.API{
		{
			Namespace:     "builder",
			Version:       "1.0",
			Service:       builderService,
			Public:        true,
			Authenticated: true,
		},
	})

	stack.RegisterLifecycle(builderService)

	return nil
}
