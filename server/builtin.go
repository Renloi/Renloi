package server

import (
	"github.com/renloi/Renloi/consensus"
	consensusDev "github.com/renloi/Renloi/consensus/dev"
	consensusDummy "github.com/renloi/Renloi/consensus/dummy"
	consensusIBFT "github.com/renloi/Renloi/consensus/ibft"
	"github.com/renloi/Renloi/secrets"
	"github.com/renloi/Renloi/secrets/awsssm"
	"github.com/renloi/Renloi/secrets/hashicorpvault"
	"github.com/renloi/Renloi/secrets/local"
)

type ConsensusType string

const (
	DevConsensus   ConsensusType = "dev"
	IBFTConsensus  ConsensusType = "ibft"
	DummyConsensus ConsensusType = "dummy"
)

var consensusBackends = map[ConsensusType]consensus.Factory{
	DevConsensus:   consensusDev.Factory,
	IBFTConsensus:  consensusIBFT.Factory,
	DummyConsensus: consensusDummy.Factory,
}

// secretsManagerBackends defines the SecretManager factories for different
// secret management solutions
var secretsManagerBackends = map[secrets.SecretsManagerType]secrets.SecretsManagerFactory{
	secrets.Local:          local.SecretsManagerFactory,
	secrets.HashicorpVault: hashicorpvault.SecretsManagerFactory,
	secrets.AWSSSM:         awsssm.SecretsManagerFactory,
}

func ConsensusSupported(value string) bool {
	_, ok := consensusBackends[ConsensusType(value)]

	return ok
}
