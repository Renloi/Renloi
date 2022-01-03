package server

import (
	"github.com/Renloi/Renloi/consensus"
	"github.com/Renloi/Renloi/txpool"
)

// serverMetrics holds the metric instances of all sub systems
type serverMetrics struct {
	consensus *consensus.Metrics
	txpool    *txpool.Metrics
}

// metricProvider serverMetric instance for the given ChainID and nameSpace
func metricProvider(nameSpace string, chainID string, metricsRequired bool) *serverMetrics {
	if metricsRequired {
		return &serverMetrics{
			consensus: consensus.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
			txpool:    txpool.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
		}
	}
	return &serverMetrics{
		consensus: consensus.NilMetrics(),
		txpool:    txpool.NilMetrics(),
	}

}
