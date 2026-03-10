package idgen

import (
	"fmt"
	"hash/fnv"
	"os"

	"github.com/bwmarrin/snowflake"
)

const (
	// AutoNodeID asks generator to derive node id from hostname.
	AutoNodeID int64 = -1
	maxNodeID  int64 = 1023
)

type Generator interface {
	NextID() string
	NodeID() int64
}

type SnowflakeGenerator struct {
	node   *snowflake.Node
	nodeID int64
}

func NewSnowflakeGenerator(nodeID int64) (*SnowflakeGenerator, error) {
	if nodeID == AutoNodeID {
		nodeID = deriveNodeID()
	}
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("invalid snowflake node id: %d", nodeID)
	}

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, err
	}

	return &SnowflakeGenerator{
		node:   node,
		nodeID: nodeID,
	}, nil
}

func (g *SnowflakeGenerator) NextID() string {
	return g.node.Generate().String()
}

func (g *SnowflakeGenerator) NodeID() int64 {
	return g.nodeID
}

func deriveNodeID() int64 {
	host, _ := os.Hostname()
	if host == "" {
		host = "order-api"
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(host))
	return int64(h.Sum32() % uint32(maxNodeID+1))
}
