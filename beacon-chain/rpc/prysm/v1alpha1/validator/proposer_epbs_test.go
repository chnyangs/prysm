package validator

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	chainMock "github.com/prysmaticlabs/prysm/v5/beacon-chain/blockchain/testing"
	mockChain "github.com/prysmaticlabs/prysm/v5/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/cache"
	powtesting "github.com/prysmaticlabs/prysm/v5/beacon-chain/execution/testing"
	doublylinkedtree "github.com/prysmaticlabs/prysm/v5/beacon-chain/forkchoice/doubly-linked-tree"
	p2ptest "github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p/testing"
	mockSync "github.com/prysmaticlabs/prysm/v5/beacon-chain/sync/initial-sync/testing"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/encoding/ssz"
	enginev1 "github.com/prysmaticlabs/prysm/v5/proto/engine/v1"
	v1 "github.com/prysmaticlabs/prysm/v5/proto/engine/v1"
	eth "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/testing/require"
	"github.com/prysmaticlabs/prysm/v5/testing/util"
)

func TestServer_SubmitSignedExecutionPayloadEnvelope(t *testing.T) {
	env := &enginev1.SignedExecutionPayloadEnvelope{
		Message: &enginev1.ExecutionPayloadEnvelope{
			Payload:            &enginev1.ExecutionPayloadElectra{},
			BeaconBlockRoot:    make([]byte, 32),
			BlobKzgCommitments: [][]byte{},
			StateRoot:          make([]byte, 32),
		},
		Signature: make([]byte, 96),
	}
	t.Run("Happy case", func(t *testing.T) {
		st, _ := util.DeterministicGenesisStateEpbs(t, 1)
		s := &Server{
			P2P:                      p2ptest.NewTestP2P(t),
			ExecutionPayloadReceiver: &mockChain.ChainService{State: st},
		}
		_, err := s.SubmitSignedExecutionPayloadEnvelope(context.Background(), env)
		require.NoError(t, err)
	})

	t.Run("Receive failed", func(t *testing.T) {
		s := &Server{
			P2P:                      p2ptest.NewTestP2P(t),
			ExecutionPayloadReceiver: &mockChain.ChainService{ReceiveBlockMockErr: errors.New("receive failed")},
		}
		_, err := s.SubmitSignedExecutionPayloadEnvelope(context.Background(), env)
		require.ErrorContains(t, "failed to receive execution payload envelope: receive failed", err)
	})
}

func TestServer_GetLocalHeader(t *testing.T) {
	t.Run("Node is syncing", func(t *testing.T) {
		vs := &Server{
			SyncChecker: &mockSync.Sync{IsSyncing: true},
		}
		_, err := vs.GetLocalHeader(context.Background(), &eth.HeaderRequest{
			Slot:          0,
			ProposerIndex: 0,
		})
		require.ErrorContains(t, "Syncing to latest head, not ready to respond", err)
	})
	t.Run("ePBS fork has not occurred", func(t *testing.T) {
		vs := &Server{
			SyncChecker: &mockSync.Sync{IsSyncing: false},
			TimeFetcher: &chainMock.ChainService{},
		}
		_, err := vs.GetLocalHeader(context.Background(), &eth.HeaderRequest{
			Slot:          0,
			ProposerIndex: 0,
		})
		require.ErrorContains(t, "EPBS fork has not occurred yet", err)
	})
	t.Run("Happy case", func(t *testing.T) {
		params.SetupTestConfigCleanup(t)
		cfg := params.BeaconConfig().Copy()
		cfg.EPBSForkEpoch = 1
		params.OverrideBeaconConfig(cfg)

		st, _ := util.DeterministicGenesisStateEpbs(t, 1)
		fc := doublylinkedtree.New()
		chainService := &chainMock.ChainService{
			ForkChoiceStore: fc,
			State:           st,
		}
		payloadIdCache := cache.NewPayloadIDCache()
		payloadId := primitives.PayloadID{1}
		payloadIdCache.Set(params.BeaconConfig().SlotsPerEpoch, [32]byte{}, payloadId)

		payload := &v1.ExecutionPayloadElectra{
			ParentHash: []byte{1},
			BlockHash:  []byte{2},
			GasLimit:   1000000,
		}
		executionData, err := blocks.NewWrappedExecutionData(payload)
		require.NoError(t, err)
		kzgs := [][]byte{make([]byte, 48), make([]byte, 48), make([]byte, 48)}
		vs := &Server{
			SyncChecker:            &mockSync.Sync{IsSyncing: false},
			TimeFetcher:            chainService,
			ForkchoiceFetcher:      chainService,
			HeadFetcher:            chainService,
			TrackedValidatorsCache: cache.NewTrackedValidatorsCache(),
			PayloadIDCache:         payloadIdCache,
			ExecutionEngineCaller: &powtesting.EngineClient{
				GetPayloadResponse: &blocks.GetPayloadResponse{
					ExecutionData: executionData,
					BlobsBundle: &v1.BlobsBundle{
						KzgCommitments: kzgs,
					}},
			},
		}
		validatorIndex := primitives.ValidatorIndex(1)
		vs.TrackedValidatorsCache.Set(cache.TrackedValidator{Active: true, Index: validatorIndex})
		slot := primitives.Slot(params.BeaconConfig().EPBSForkEpoch) * params.BeaconConfig().SlotsPerEpoch
		h, err := vs.GetLocalHeader(context.Background(), &eth.HeaderRequest{
			Slot:          slot,
			ProposerIndex: validatorIndex,
		})
		require.NoError(t, err)
		require.DeepEqual(t, h.ParentBlockHash, payload.ParentHash)
		require.DeepEqual(t, h.BlockHash, payload.BlockHash)
		require.Equal(t, h.GasLimit, payload.GasLimit)
		require.Equal(t, h.BuilderIndex, validatorIndex)
		require.Equal(t, h.Slot, slot)
		require.Equal(t, h.Value, uint64(0))
		kzgRoot, err := ssz.KzgCommitmentsRoot(kzgs)
		require.NoError(t, err)
		require.DeepEqual(t, h.BlobKzgCommitmentsRoot, kzgRoot[:])
	})
}
