package consensus

import (
	"context"
	"github.com/Renloi/Renloi/protocol"
	"log"

	"github.com/Renloi/Renloi/blockchain"
	"github.com/Renloi/Renloi/chain"
	"github.com/Renloi/Renloi/network"
	"github.com/Renloi/Renloi/secrets"
	"github.com/Renloi/Renloi/state"
	"github.com/Renloi/Renloi/txpool"
	"github.com/Renloi/Renloi/types"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
)

// Consensus is the public interface for consensus mechanism
// Each consensus mechanism must implement this interface in order to be valid
type Consensus interface {
	// VerifyHeader verifies the header is correct
	VerifyHeader(parent, header *types.Header) error

	// GetBlockCreator retrieves the block creator (or signer) given the block header
	GetBlockCreator(header *types.Header) (types.Address, error)

	// GetSyncProgression retrieves the current sync progression, if any
	GetSyncProgression() *protocol.Progression

	// Start starts the consensus
	Start() error

	// Close closes the connection
	Close() error
}

// Config is the configuration for the consensus
type Config struct {
	// Logger to be used by the backend
	Logger *log.Logger

	// Params are the params of the chain and the consensus
	Params *chain.Params

	// Config defines specific configuration parameters for the backend
	Config map[string]interface{}

	// Path is the directory path for the consensus protocol tos tore information
	Path string
}

type ConsensusParams struct {
	Context        context.Context
	Seal           bool
	Config         *Config
	Txpool         *txpool.TxPool
	Network        *network.Server
	Blockchain     *blockchain.Blockchain
	Executor       *state.Executor
	Grpc           *grpc.Server
	Logger         hclog.Logger
	Metrics        *Metrics
	SecretsManager secrets.SecretsManager
}

// Factory is the factory function to create a discovery backend
type Factory func(
	*ConsensusParams,
) (Consensus, error)
