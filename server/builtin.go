package server

import (
	consensusDev "github.com/Renloi/Renloi/consensus/dev"
	consensusDummy "github.com/Renloi/Renloi/consensus/dummy"
	consensusIBFT "github.com/Renloi/Renloi/consensus/ibft"
	"github.com/Renloi/Renloi/secrets"
	"github.com/Renloi/Renloi/secrets/hashicorpvault"
	"github.com/Renloi/Renloi/secrets/local"

	"github.com/Renloi/Renloi/consensus"
)

var consensusBackends = map[string]consensus.Factory{
	"dev":   consensusDev.Factory,
	"ibft":  consensusIBFT.Factory,
	"dummy": consensusDummy.Factory,
}

// secretsManagerBackends defines the SecretManager factories for different
// secret management solutions
var secretsManagerBackends = map[secrets.SecretsManagerType]secrets.SecretsManagerFactory{
	secrets.Local:          local.SecretsManagerFactory,
	secrets.HashicorpVault: hashicorpvault.SecretsManagerFactory,
}
