package init

import (
	"bytes"
	"fmt"
	"github.com/renloi/Renloi/command/helper"
	"github.com/renloi/Renloi/types"
)

type SecretsInitResult struct {
	Address types.Address `json:"address"`
	NodeID  string        `json:"node_id"`
}

func (r *SecretsInitResult) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[SECRETS INIT]\n")
	buffer.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Public key (address)|%s", r.Address),
		fmt.Sprintf("Node ID|%s", r.NodeID),
	}))
	buffer.WriteString("\n")

	return buffer.String()
}