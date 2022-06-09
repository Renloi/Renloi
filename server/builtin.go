package server

import (
	"github.com/Renloi/Renloi/consensus"
	consensusDev "github.com/Renloi/Renloi/consensus/dev"
	consensusDummy "github.com/Renloi/Renloi/consensus/dummy"
	consensusIBFT "github.com/Renloi/Renloi/consensus/ibft"
	"github.com/Renloi/Renloi/secrets"
	"github.com/Renloi/Renloi/secrets/awsssm"
	"github.com/Renloi/Renloi/secrets/gcpssm"
	"github.com/Renloi/Renloi/secrets/hashicorpvault"
	"github.com/Renloi/Renloi/secrets/local"
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
	secrets.GCPSSM:         gcpssm.SecretsManagerFactory,
}

func ConsensusSupported(value string) bool {
	_, ok := consensusBackends[ConsensusType(value)]

	return ok
}
