package ibft

import (
	"crypto/ecdsa"
	"fmt"
	"reflect"
	"time"

	"github.com/Renloi/Renloi/consensus"
	"github.com/Renloi/Renloi/consensus/ibft/proto"
	"github.com/Renloi/Renloi/crypto"
	"github.com/Renloi/Renloi/helper/hex"
	"github.com/Renloi/Renloi/network"
	"github.com/Renloi/Renloi/protocol"
	"github.com/Renloi/Renloi/secrets"
	"github.com/Renloi/Renloi/state"
	"github.com/Renloi/Renloi/types"
	"github.com/hashicorp/go-hclog"
	any "google.golang.org/protobuf/types/known/anypb"
)

const (
	DefaultEpochSize = 100000
)

type blockchainInterface interface {
	Header() *types.Header
	GetHeaderByNumber(i uint64) (*types.Header, bool)
	WriteBlocks(blocks []*types.Block) error
	CalculateGasLimit(number uint64) (uint64, error)
}

type transactionPoolInterface interface {
	ResetWithHeaders(headers ...*types.Header)
	Pop() (*types.Transaction, func())
	DecreaseAccountNonce(tx *types.Transaction)
	Length() uint64
}

type syncerInterface interface {
	Start()
	BestPeer() *protocol.SyncPeer
	BulkSyncWithPeer(p *protocol.SyncPeer, newBlocksHandler func(blocks []*types.Block)) error
	WatchSyncWithPeer(p *protocol.SyncPeer, newBlockHandler func(b *types.Block) bool)
	GetSyncProgression() *protocol.Progression
	Broadcast(b *types.Block)
}

// Ibft represents the IBFT consensus mechanism object
type Ibft struct {
	sealing bool // Flag indicating if the node is a sealer

	logger hclog.Logger      // Output logger
	config *consensus.Config // Consensus configuration
	state  *currentState     // Reference to the current state

	blockchain blockchainInterface // Interface exposed by the blockchain layer
	executor   *state.Executor     // Reference to the state executor
	closeCh    chan struct{}       // Channel for closing

	validatorKey     *ecdsa.PrivateKey // Private key for the validator
	validatorKeyAddr types.Address

	txpool transactionPoolInterface // Reference to the transaction pool

	store     *snapshotStore // Snapshot store that keeps track of all snapshots
	epochSize uint64

	msgQueue *msgQueue     // Structure containing different message queues
	updateCh chan struct{} // Update channel

	syncer syncerInterface // Reference to the sync protocol

	network   *network.Server // Reference to the networking layer
	transport transport       // Reference to the transport protocol

	operator *operator

	// aux test methods
	forceTimeoutCh bool

	metrics *consensus.Metrics

	secretsManager secrets.SecretsManager
}

// Factory implements the base consensus Factory method
func Factory(
	params *consensus.ConsensusParams,
) (consensus.Consensus, error) {
	p := &Ibft{
		logger:         params.Logger.Named("ibft"),
		config:         params.Config,
		blockchain:     params.Blockchain,
		executor:       params.Executor,
		closeCh:        make(chan struct{}),
		txpool:         params.Txpool,
		state:          &currentState{},
		network:        params.Network,
		epochSize:      DefaultEpochSize,
		sealing:        params.Seal,
		metrics:        params.Metrics,
		secretsManager: params.SecretsManager,
	}

	// Istanbul requires a different header hash function
	types.HeaderHash = istanbulHeaderHash

	p.syncer = protocol.NewSyncer(params.Logger, params.Network, params.Blockchain)

	// register the grpc operator
	p.operator = &operator{ibft: p}
	proto.RegisterIbftOperatorServer(params.Grpc, p.operator)

	// Set up the node's validator key
	if err := p.createKey(); err != nil {
		return nil, err
	}

	p.logger.Info("validator key", "addr", p.validatorKeyAddr.String())

	// start the transport protocol
	if err := p.setupTransport(); err != nil {
		return nil, err
	}

	return p, nil
}

// Start starts the IBFT consensus
func (i *Ibft) Start() error {
	// Start the syncer
	i.syncer.Start()

	// Set up the snapshots
	if err := i.setupSnapshot(); err != nil {
		return err
	}

	// Start the actual IBFT protocol
	go i.start()

	return nil
}

// GetSyncProgression gets the latest sync progression, if any
func (i *Ibft) GetSyncProgression() *protocol.Progression {
	return i.syncer.GetSyncProgression()
}

type transport interface {
	Gossip(msg *proto.MessageReq) error
}

// Define the IBFT libp2p protocol
var ibftProto = "/ibft/0.1"

type gossipTransport struct {
	topic *network.Topic
}

// Gossip publishes a new message to the topic
func (g *gossipTransport) Gossip(msg *proto.MessageReq) error {
	return g.topic.Publish(msg)
}

// setupTransport sets up the gossip transport protocol
func (i *Ibft) setupTransport() error {
	// Define a new topic
	topic, err := i.network.NewTopic(ibftProto, &proto.MessageReq{})
	if err != nil {
		return err
	}

	// Subscribe to the newly created topic
	err = topic.Subscribe(func(obj interface{}) {
		msg := obj.(*proto.MessageReq)

		if !i.isSealing() {
			// if we are not sealing we do not care about the messages
			// but we need to subscribe to propagate the messages
			return
		}

		// decode sender
		if err := validateMsg(msg); err != nil {
			i.logger.Error("failed to validate msg", "err", err)

			return
		}

		if msg.From == i.validatorKeyAddr.String() {
			// we are the sender, skip this message since we already
			// relay our own messages internally.
			return
		}

		i.pushMessage(msg)
	})

	if err != nil {
		return err
	}

	i.transport = &gossipTransport{topic: topic}

	return nil
}

// createKey sets the validator's private key from the secrets manager
func (i *Ibft) createKey() error {
	i.msgQueue = newMsgQueue()
	i.closeCh = make(chan struct{})
	i.updateCh = make(chan struct{})

	if i.validatorKey == nil {
		// Check if the validator key is initialized
		var key *ecdsa.PrivateKey
		if i.secretsManager.HasSecret(secrets.ValidatorKey) {
			// The validator key is present in the secrets manager, load it
			validatorKey, readErr := crypto.ReadConsensusKey(i.secretsManager)
			if readErr != nil {
				return fmt.Errorf("unable to read validator key from Secrets Manager, %v", readErr)
			}

			key = validatorKey
		} else {
			// The validator key is not present in the secrets manager, generate it
			validatorKey, validatorKeyEncoded, genErr := crypto.GenerateAndEncodePrivateKey()
			if genErr != nil {
				return fmt.Errorf("unable to generate validator key for Secrets Manager, %v", genErr)
			}

			// Save the key to the secrets manager
			saveErr := i.secretsManager.SetSecret(secrets.ValidatorKey, validatorKeyEncoded)
			if saveErr != nil {
				return fmt.Errorf("unable to save validator key to Secrets Manager, %v", saveErr)
			}

			key = validatorKey
		}

		i.validatorKey = key
		i.validatorKeyAddr = crypto.PubKeyToAddress(&key.PublicKey)
	}

	return nil
}

const IbftKeyName = "validator.key"

// start starts the IBFT consensus state machine
func (i *Ibft) start() {
	// consensus always starts in SyncState mode in case it needs
	// to sync with other nodes.
	i.setState(SyncState)

	// Grab the latest header
	header := i.blockchain.Header()
	i.logger.Debug("current sequence", "sequence", header.Number+1)

	for {
		select {
		case <-i.closeCh:
			return
		default: // Default is here because we would block until we receive something in the closeCh
		}

		// Start the state machine loop
		i.runCycle()
	}
}

// runCycle represents the IBFT state machine loop
func (i *Ibft) runCycle() {
	// Log to the console
	if i.state.view != nil {
		i.logger.Debug("cycle", "state", i.getState(), "sequence", i.state.view.Sequence, "round", i.state.view.Round+1)
	}

	// Based on the current state, execute the corresponding section
	switch i.getState() {
	case AcceptState:
		i.runAcceptState()

	case ValidateState:
		i.runValidateState()

	case RoundChangeState:
		i.runRoundChangeState()

	case SyncState:
		i.runSyncState()
	}
}

// isValidSnapshot checks if the current node is in the validator set for the latest snapshot
func (i *Ibft) isValidSnapshot() bool {
	if !i.isSealing() {
		return false
	}

	// check if we are a validator and enabled
	header := i.blockchain.Header()
	snap, err := i.getSnapshot(header.Number)
	if err != nil {
		return false
	}

	if snap.Set.Includes(i.validatorKeyAddr) {
		i.state.view = &proto.View{
			Sequence: header.Number + 1,
			Round:    0,
		}

		return true
	}
	return false
}

// runSyncState implements the Sync state loop.
//
// It fetches fresh data from the blockchain. Checks if the current node is a validator and resolves any pending blocks
func (i *Ibft) runSyncState() {
	for i.isState(SyncState) {
		// try to sync with some target peer
		p := i.syncer.BestPeer()
		if p == nil {
			// if we do not have any peers and we have been a validator
			// we can start now. In case we start on another fork this will be
			// reverted later
			if i.isValidSnapshot() {
				// initialize the round and sequence
				header := i.blockchain.Header()
				i.state.view = &proto.View{
					Round:    0,
					Sequence: header.Number + 1,
				}
				//Set the round metric
				i.metrics.Rounds.Set(float64(i.state.view.Round))

				i.setState(AcceptState)
			} else {
				time.Sleep(1 * time.Second)
			}
			continue
		}

		if err := i.syncer.BulkSyncWithPeer(p, func(newBlocks []*types.Block) {
			newHeaders := []*types.Header{}
			for _, block := range newBlocks {
				newHeaders = append(newHeaders, block.Header)
			}

			i.txpool.ResetWithHeaders(newHeaders...)
		}); err != nil {
			i.logger.Error("failed to bulk sync", "err", err)
			continue
		}

		// if we are a validator we do not even want to wait here
		// we can just move ahead
		if i.isValidSnapshot() {
			i.setState(AcceptState)
			continue
		}

		// start watch mode
		var isValidator bool
		i.syncer.WatchSyncWithPeer(p, func(b *types.Block) bool {
			i.syncer.Broadcast(b)
			i.txpool.ResetWithHeaders(b.Header)
			isValidator = i.isValidSnapshot()

			return isValidator
		})

		if isValidator {
			// at this point, we are in sync with the latest chain we know of
			// and we are a validator of that chain so we need to change to AcceptState
			// so that we can start to do some stuff there
			i.setState(AcceptState)
		}
	}
}

var defaultBlockPeriod = 3 * time.Second

// buildBlock builds the block, based on the passed in snapshot and parent header
func (i *Ibft) buildBlock(snap *Snapshot, parent *types.Header) (*types.Block, error) {
	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      types.Address{},
		Nonce:      types.Nonce{},
		MixHash:    IstanbulDigest,
		Difficulty: parent.Number + 1,   // we need to do this because blockchain needs difficulty to organize blocks and forks
		StateRoot:  types.EmptyRootHash, // this avoids needing state for now
		Sha3Uncles: types.EmptyUncleHash,
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
	}

	// calculate gas limit based on parent header
	gasLimit, err := i.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return nil, err
	}
	header.GasLimit = gasLimit

	// try to pick a candidate
	if candidate := i.operator.getNextCandidate(snap); candidate != nil {
		header.Miner = types.StringToAddress(candidate.Address)
		if candidate.Auth {
			header.Nonce = nonceAuthVote
		} else {
			header.Nonce = nonceDropVote
		}
	}

	// set the timestamp
	parentTime := time.Unix(int64(parent.Timestamp), 0)
	headerTime := parentTime.Add(defaultBlockPeriod)

	if headerTime.Before(time.Now()) {
		headerTime = time.Now()
	}
	header.Timestamp = uint64(headerTime.Unix())

	// we need to include in the extra field the current set of validators
	putIbftExtraValidators(header, snap.Set)

	transition, err := i.executor.BeginTxn(parent.StateRoot, header, i.validatorKeyAddr)
	if err != nil {
		return nil, err
	}
	txns := i.writeTransactions(gasLimit, transition)

	_, root := transition.Commit()
	header.StateRoot = root
	header.GasUsed = transition.TotalGas()

	// build the block
	block := consensus.BuildBlock(consensus.BuildBlockParams{
		Header:   header,
		Txns:     txns,
		Receipts: transition.Receipts(),
	})

	// write the seal of the block after all the fields are completed
	header, err = writeSeal(i.validatorKey, block.Header)
	if err != nil {
		return nil, err
	}
	block.Header = header

	// compute the hash, this is only a provisional hash since the final one
	// is sealed after all the committed seals
	block.Header.ComputeHash()

	i.logger.Info("build block", "number", header.Number, "txns", len(txns))
	return block, nil
}

type transitionInterface interface {
	Write(txn *types.Transaction) error
}

// writeTransactions writes transactions from the txpool to the transition object
// and returns transactions that were included in the transition (new block)
func (i *Ibft) writeTransactions(gasLimit uint64, transition transitionInterface) []*types.Transaction {
	txns := []*types.Transaction{}
	returnTxnFuncs := []func(){}
	for {
		txn, retTxnFn := i.txpool.Pop()
		if txn == nil {
			break
		}

		if txn.ExceedsBlockGasLimit(gasLimit) {
			i.logger.Error(fmt.Sprintf("failed to write transaction: %v", state.ErrBlockLimitExceeded))
			i.txpool.DecreaseAccountNonce(txn)
			continue
		}

		if err := transition.Write(txn); err != nil {
			if _, ok := err.(*state.GasLimitReachedTransitionApplicationError); ok {
				returnTxnFuncs = append(returnTxnFuncs, retTxnFn)
				break
			} else if appErr, ok := err.(*state.TransitionApplicationError); ok && appErr.IsRecoverable {
				returnTxnFuncs = append(returnTxnFuncs, retTxnFn)
			} else {
				i.txpool.DecreaseAccountNonce(txn)
			}
			continue
		}

		txns = append(txns, txn)
	}

	// we return recoverable txns that were popped from the txpool after the above for loop breaks,
	// since we don't want to return the tx to the pool just to pop the same txn in the next loop iteration
	for _, retFunc := range returnTxnFuncs {
		retFunc()
	}

	i.logger.Info("picked out txns from pool", "num", len(txns), "remaining", i.txpool.Length())
	return txns
}

// runAcceptState runs the Accept state loop
//
// The Accept state always checks the snapshot, and the validator set. If the current node is not in the validators set,
// it moves back to the Sync state. On the other hand, if the node is a validator, it calculates the proposer.
// If it turns out that the current node is the proposer, it builds a block, and sends preprepare and then prepare messages.
func (i *Ibft) runAcceptState() { // start new round
	logger := i.logger.Named("acceptState")
	logger.Info("Accept state", "sequence", i.state.view.Sequence, "round", i.state.view.Round+1)

	// This is the state in which we either propose a block or wait for the pre-prepare message
	parent := i.blockchain.Header()
	number := parent.Number + 1
	if number != i.state.view.Sequence {
		i.logger.Error("sequence not correct", "parent", parent.Number, "sequence", i.state.view.Sequence)
		i.setState(SyncState)
		return
	}
	snap, err := i.getSnapshot(parent.Number)
	if err != nil {
		i.logger.Error("cannot find snapshot", "num", parent.Number)
		i.setState(SyncState)
		return
	}

	if !snap.Set.Includes(i.validatorKeyAddr) {
		// we are not a validator anymore, move back to sync state
		i.logger.Info("we are not a validator anymore")
		i.setState(SyncState)
		return
	}

	i.logger.Info("current snapshot", "validators", len(snap.Set), "votes", len(snap.Votes))

	i.state.validators = snap.Set

	//Update the No.of validator metric
	i.metrics.Validators.Set(float64(len(snap.Set)))
	// reset round messages
	i.state.resetRoundMsgs()

	// select the proposer of the block
	var lastProposer types.Address
	if parent.Number != 0 {
		lastProposer, _ = ecrecoverFromHeader(parent)
	}

	i.state.CalcProposer(lastProposer)

	if i.state.proposer == i.validatorKeyAddr {
		logger.Info("we are the proposer", "block", number)

		if !i.state.locked {
			// since the state is not locked, we need to build a new block
			i.state.block, err = i.buildBlock(snap, parent)
			if err != nil {
				i.logger.Error("failed to build block", "err", err)
				i.setState(RoundChangeState)
				return
			}

			// calculate how much time do we have to wait to mine the block
			delay := time.Until(time.Unix(int64(i.state.block.Header.Timestamp), 0))

			select {
			case <-time.After(delay):
			case <-i.closeCh:
				return
			}
		}

		// send the preprepare message as an RLP encoded block
		i.sendPreprepareMsg()

		// send the prepare message since we are ready to move the state
		i.sendPrepareMsg()

		// move to validation state for new prepare messages
		i.setState(ValidateState)
		return
	}

	i.logger.Info("proposer calculated", "proposer", i.state.proposer, "block", number)

	// we are NOT a proposer for the block. Then, we have to wait
	// for a pre-prepare message from the proposer

	timeout := exponentialTimeout(i.state.view.Round)
	for i.getState() == AcceptState {
		msg, ok := i.getNextMessage(timeout)
		if !ok {
			return
		}
		if msg == nil {
			i.setState(RoundChangeState)
			continue
		}

		if msg.From != i.state.proposer.String() {
			i.logger.Error("msg received from wrong proposer")
			continue
		}

		// retrieve the block proposal
		block := &types.Block{}
		if err := block.UnmarshalRLP(msg.Proposal.Value); err != nil {
			i.logger.Error("failed to unmarshal block", "err", err)
			i.setState(RoundChangeState)
			return
		}
		if i.state.locked {
			// the state is locked, we need to receive the same block
			if block.Hash() == i.state.block.Hash() {
				// fast-track and send a commit message and wait for validations
				i.sendCommitMsg()
				i.setState(ValidateState)
			} else {
				i.handleStateErr(errIncorrectBlockLocked)
			}
		} else {
			// since its a new block, we have to verify it first
			if err := i.verifyHeaderImpl(snap, parent, block.Header); err != nil {
				i.logger.Error("block verification failed", "err", err)
				i.handleStateErr(errBlockVerificationFailed)
			} else {
				i.state.block = block

				// send prepare message and wait for validations
				i.sendPrepareMsg()
				i.setState(ValidateState)
			}
		}
	}
}

// runValidateState implements the Validate state loop.
//
// The Validate state is rather simple - all nodes do in this state is read messages and add them to their local snapshot state
func (i *Ibft) runValidateState() {
	hasCommitted := false
	sendCommit := func() {
		// at this point either we have enough prepare messages
		// or commit messages so we can lock the block
		i.state.lock()

		if !hasCommitted {
			// send the commit message
			i.sendCommitMsg()
			hasCommitted = true
		}
	}

	timeout := exponentialTimeout(i.state.view.Round)
	for i.getState() == ValidateState {
		msg, ok := i.getNextMessage(timeout)
		if !ok {
			// closing
			return
		}
		if msg == nil {
			i.setState(RoundChangeState)
			continue
		}

		switch msg.Type {
		case proto.MessageReq_Prepare:
			i.state.addPrepared(msg)

		case proto.MessageReq_Commit:
			i.state.addCommitted(msg)

		default:
			panic(fmt.Sprintf("BUG: %s", reflect.TypeOf(msg.Type)))
		}

		if i.state.numPrepared() > i.state.NumValid() {
			// we have received enough pre-prepare messages
			sendCommit()
		}

		if i.state.numCommitted() > i.state.NumValid() {
			// we have received enough commit messages
			sendCommit()

			// try to commit the block (TODO: just to get out of the loop)
			i.setState(CommitState)
		}
	}

	if i.getState() == CommitState {
		// at this point either if it works or not we need to unlock
		block := i.state.block
		i.state.unlock()

		if err := i.insertBlock(block); err != nil {
			// start a new round with the state unlocked since we need to
			// be able to propose/validate a different block
			i.logger.Error("failed to insert block", "err", err)
			i.handleStateErr(errFailedToInsertBlock)
		} else {
			// update metrics
			i.updateMetrics(block)

			// move ahead to the next block
			i.setState(AcceptState)
		}
	}
}

// updateMetrics will update various metrics based on the given block
// currently we capture No.of Txs and block interval metrics using this function
func (i *Ibft) updateMetrics(block *types.Block) {
	prvHeader, _ := i.blockchain.GetHeaderByNumber(block.Number() - 1)
	parentTime := time.Unix(int64(prvHeader.Timestamp), 0)
	headerTime := time.Unix(int64(block.Header.Timestamp), 0)
	//Update the block interval metric
	if block.Number() > 1 {
		i.metrics.BlockInterval.Observe(
			headerTime.Sub(parentTime).Seconds(),
		)
	}
	//Update the Number of transactions in the block metric
	i.metrics.NumTxs.Set(float64(len(block.Body().Transactions)))

}
func (i *Ibft) insertBlock(block *types.Block) error {
	committedSeals := [][]byte{}
	for _, commit := range i.state.committed {
		// no need to check the format of seal here because writeCommittedSeals will check
		committedSeals = append(committedSeals, hex.MustDecodeHex(commit.Seal))
	}

	header, err := writeCommittedSeals(block.Header, committedSeals)
	if err != nil {
		return err
	}

	// we need to recompute the hash since we have change extra-data
	block.Header = header
	block.Header.ComputeHash()

	if err := i.blockchain.WriteBlocks([]*types.Block{block}); err != nil {
		return err
	}

	i.logger.Info(
		"block committed",
		"sequence", i.state.view.Sequence,
		"hash", block.Hash(),
		"validators", len(i.state.validators),
		"rounds", i.state.view.Round+1,
		"committed", i.state.numCommitted(),
	)

	// increase the sequence number and reset the round if any
	i.state.view = &proto.View{
		Sequence: header.Number + 1,
		Round:    0,
	}

	// broadcast the new block
	i.syncer.Broadcast(block)

	// after the block has been written we reset the txpool so that
	// the old transactions are removed
	i.txpool.ResetWithHeaders(block.Header)

	return nil
}

var (
	errIncorrectBlockLocked    = fmt.Errorf("block locked is incorrect")
	errBlockVerificationFailed = fmt.Errorf("block verification failed")
	errFailedToInsertBlock     = fmt.Errorf("failed to insert block")
)

func (i *Ibft) handleStateErr(err error) {
	i.state.err = err
	i.setState(RoundChangeState)
}

func (i *Ibft) runRoundChangeState() {
	sendRoundChange := func(round uint64) {
		i.logger.Debug("local round change", "round", round+1)
		// set the new round and update the round metric
		i.state.view.Round = round
		i.metrics.Rounds.Set(float64(round))
		// clean the round
		i.state.cleanRound(round)
		// send the round change message
		i.sendRoundChange()
	}
	sendNextRoundChange := func() {
		sendRoundChange(i.state.view.Round + 1)
	}

	checkTimeout := func() {
		// check if there is any peer that is really advanced and we might need to sync with it first
		if i.syncer != nil {
			bestPeer := i.syncer.BestPeer()
			if bestPeer != nil {
				lastProposal := i.blockchain.Header()
				if bestPeer.Number() > lastProposal.Number {
					i.logger.Debug("it has found a better peer to connect", "local", lastProposal.Number, "remote", bestPeer.Number())
					// we need to catch up with the last sequence
					i.setState(SyncState)
					return
				}
			}
		}

		// otherwise, it seems that we are in sync
		// and we should start a new round
		sendNextRoundChange()
	}

	// if the round was triggered due to an error, we send our own
	// next round change
	if err := i.state.getErr(); err != nil {
		i.logger.Debug("round change handle err", "err", err)
		sendNextRoundChange()
	} else {
		// otherwise, it is due to a timeout in any stage
		// First, we try to sync up with any max round already available
		if maxRound, ok := i.state.maxRound(); ok {
			i.logger.Debug("round change set max round", "round", maxRound)
			sendRoundChange(maxRound)
		} else {
			// otherwise, do your best to sync up
			checkTimeout()
		}
	}

	// create a timer for the round change
	timeout := exponentialTimeout(i.state.view.Round)
	for i.getState() == RoundChangeState {
		msg, ok := i.getNextMessage(timeout)
		if !ok {
			// closing
			return
		}
		if msg == nil {
			i.logger.Debug("round change timeout")
			checkTimeout()
			// update the timeout duration
			timeout = exponentialTimeout(i.state.view.Round)
			continue
		}

		// we only expect RoundChange messages right now
		num := i.state.AddRoundMessage(msg)

		if num == i.state.NumValid() {
			// start a new round immediately
			i.state.view.Round = msg.View.Round
			i.setState(AcceptState)
		} else if num == i.state.validators.MaxFaultyNodes()+1 {
			// weak certificate, try to catch up if our round number is smaller
			if i.state.view.Round < msg.View.Round {
				// update timer
				timeout = exponentialTimeout(i.state.view.Round)
				sendRoundChange(msg.View.Round)
			}
		}
	}
}

// --- com wrappers ---

func (i *Ibft) sendRoundChange() {
	i.gossip(proto.MessageReq_RoundChange)
}

func (i *Ibft) sendPreprepareMsg() {
	i.gossip(proto.MessageReq_Preprepare)
}

func (i *Ibft) sendPrepareMsg() {
	i.gossip(proto.MessageReq_Prepare)
}

func (i *Ibft) sendCommitMsg() {
	i.gossip(proto.MessageReq_Commit)
}

func (i *Ibft) gossip(typ proto.MessageReq_Type) {
	msg := &proto.MessageReq{
		Type: typ,
	}

	// add View
	msg.View = i.state.view.Copy()

	// if we are sending a preprepare message we need to include the proposed block
	if msg.Type == proto.MessageReq_Preprepare {
		msg.Proposal = &any.Any{
			Value: i.state.block.MarshalRLP(),
		}
	}

	// if the message is commit, we need to add the committed seal
	if msg.Type == proto.MessageReq_Commit {
		seal, err := writeCommittedSeal(i.validatorKey, i.state.block.Header)
		if err != nil {
			i.logger.Error("failed to commit seal", "err", err)
			return
		}
		msg.Seal = hex.EncodeToHex(seal)
	}

	if msg.Type != proto.MessageReq_Preprepare {
		// send a copy to ourselves so that we can process this message as well
		msg2 := msg.Copy()
		msg2.From = i.validatorKeyAddr.String()
		i.pushMessage(msg2)
	}
	if err := signMsg(i.validatorKey, msg); err != nil {
		i.logger.Error("failed to sign message", "err", err)
		return
	}
	if err := i.transport.Gossip(msg); err != nil {
		i.logger.Error("failed to gossip", "err", err)
	}
}

// getState returns the current IBFT state
func (i *Ibft) getState() IbftState {
	return i.state.getState()
}

// isState checks if the node is in the passed in state
func (i *Ibft) isState(s IbftState) bool {
	return i.state.getState() == s
}

// setState sets the IBFT state
func (i *Ibft) setState(s IbftState) {
	i.logger.Info("state change", "new", s)
	i.state.setState(s)
}

// forceTimeout sets the forceTimeoutCh flag to true
func (i *Ibft) forceTimeout() {
	i.forceTimeoutCh = true
}

// isSealing checks if the current node is sealing blocks
func (i *Ibft) isSealing() bool {
	return i.sealing
}

// verifyHeaderImpl implements the actual header verification logic
func (i *Ibft) verifyHeaderImpl(snap *Snapshot, parent, header *types.Header) error {
	// ensure the extra data is correctly formatted
	if _, err := getIbftExtra(header); err != nil {
		return err
	}

	// Because you must specify either AUTH or DROP vote, it is confusing how to have a block without any votes.
	// 		This is achieved by specifying the miner field to zeroes,
	// 		because then the value in the Nonce will not be taken into consideration.
	if header.Nonce != nonceDropVote && header.Nonce != nonceAuthVote {
		return fmt.Errorf("invalid nonce")
	}

	if header.MixHash != IstanbulDigest {
		return fmt.Errorf("invalid mixhash")
	}

	if header.Sha3Uncles != types.EmptyUncleHash {
		return fmt.Errorf("invalid sha3 uncles")
	}

	// difficulty has to match number
	if header.Difficulty != header.Number {
		return fmt.Errorf("wrong difficulty")
	}

	// verify the sealer
	if err := verifySigner(snap, header); err != nil {
		return err
	}

	return nil
}

// VerifyHeader wrapper for verifying headers
func (i *Ibft) VerifyHeader(parent, header *types.Header) error {
	snap, err := i.getSnapshot(parent.Number)
	if err != nil {
		return err
	}

	// verify all the header fields + seal
	if err := i.verifyHeaderImpl(snap, parent, header); err != nil {
		return err
	}

	// verify the commited seals
	if err := verifyCommitedFields(snap, header); err != nil {
		return err
	}

	// process the new block in order to update the snapshot
	if err := i.processHeaders([]*types.Header{header}); err != nil {
		return err
	}

	return nil
}

// GetBlockCreator retrieves the block signer from the extra data field
func (i *Ibft) GetBlockCreator(header *types.Header) (types.Address, error) {
	return ecrecoverFromHeader(header)
}

// Close closes the IBFT consensus mechanism, and does write back to disk
func (i *Ibft) Close() error {
	close(i.closeCh)

	if i.config.Path != "" {
		err := i.store.saveToPath(i.config.Path)

		if err != nil {
			return err
		}
	}
	return nil
}

// getNextMessage reads a new message from the message queue
func (i *Ibft) getNextMessage(timeout time.Duration) (*proto.MessageReq, bool) {
	timeoutCh := time.After(timeout)
	for {
		msg := i.msgQueue.readMessage(i.getState(), i.state.view)
		if msg != nil {
			return msg.obj, true
		}

		if i.forceTimeoutCh {
			i.forceTimeoutCh = false
			return nil, true
		}

		// wait until there is a new message or
		// someone closes the stopCh (i.e. timeout for round change)
		select {
		case <-timeoutCh:
			i.logger.Info("unable to read new message from the message queue", "timeout expired", timeout)
			return nil, true
		case <-i.closeCh:
			return nil, false
		case <-i.updateCh:
		}
	}
}

// pushMessage pushes a new message to the message queue
func (i *Ibft) pushMessage(msg *proto.MessageReq) {
	task := &msgTask{
		view: msg.View,
		msg:  protoTypeToMsg(msg.Type),
		obj:  msg,
	}
	i.msgQueue.pushMessage(task)

	select {
	case i.updateCh <- struct{}{}:
	default:
	}
}
