package event

import (
	"testing"

	"github.com/WuKongIM/WuKongIM/pkg/cluster/node/types"
	"github.com/stretchr/testify/assert"
)

// handleCompare 每个 tick 都会调用 shouldProposeClusterAddr 决定是否回填。
// 这里穷举门控的四种边界，保证：未配 ServerAddr 不提案、地址已一致不提案、
// 空/异地址不同才提案，且 localNode 为 nil 时不 panic。
func TestShouldProposeClusterAddr(t *testing.T) {
	const addr = "127.0.0.1:11110"

	// 未配置 ServerAddr（standalone 未配 cluster.serverAddr）：绝不回填，避免写回空地址
	assert.False(t, shouldProposeClusterAddr("", &types.Node{ClusterAddr: ""}))
	assert.False(t, shouldProposeClusterAddr("   ", &types.Node{ClusterAddr: ""}))

	// 本地还没有该节点记录：不提案，也不能 panic
	assert.False(t, shouldProposeClusterAddr(addr, nil))

	// 存储值与配置一致：已收敛，不再提案（否则每个 tick 都会重复提案）
	assert.False(t, shouldProposeClusterAddr(addr, &types.Node{ClusterAddr: addr}))

	// 存储值为空但配置了地址：需回填
	assert.True(t, shouldProposeClusterAddr(addr, &types.Node{ClusterAddr: ""}))

	// 存储值为旧地址：需回填为新地址
	assert.True(t, shouldProposeClusterAddr(addr, &types.Node{ClusterAddr: "old.addr:1"}))
}
