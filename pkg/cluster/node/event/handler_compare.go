package event

import (
	"strings"

	"github.com/WuKongIM/WuKongIM/pkg/cluster/node/types"
	"go.uber.org/zap"
)

func (h *handler) handleCompare() {
	if h.cfgServer.LeaderId() == 0 {
		return
	}
	// 如果配置里自己节点的apiServerAddr配置不存在或不同，则提案配置
	if strings.TrimSpace(h.cfgOptions.ApiServerAddr) != "" {
		localNode := h.cfgServer.Node(h.cfgOptions.NodeId)
		if localNode != nil && localNode.ApiServerAddr != h.cfgOptions.ApiServerAddr {
			err := h.cfgServer.ProposeApiServerAddr(h.cfgOptions.NodeId, h.cfgOptions.ApiServerAddr)
			if err != nil {
				h.Error("ProposeApiServerAddr failed", zap.Error(err))
				return
			}
		}
	}
	// 如果配置里自己节点的clusterAddr配置不存在或不同，则提案配置。
	// standalone 首启时 ServerAddr 为空，记录会带着空 cluster_addr 出生；
	// 之后补配 serverAddr 时必须回填，否则 join 响应会把空地址发给新节点
	if shouldProposeClusterAddr(h.cfgOptions.ServerAddr, h.cfgServer.Node(h.cfgOptions.NodeId)) {
		err := h.cfgServer.ProposeClusterAddr(h.cfgOptions.NodeId, h.cfgOptions.ServerAddr)
		if err != nil {
			h.Error("ProposeClusterAddr failed", zap.Error(err))
			return
		}
	}
}

// shouldProposeClusterAddr 判定是否需要回填本节点的 cluster 通讯地址。
//
// 只在“存储里的 ClusterAddr 为空/空白”时才提案，其它情况一律保留存储值：
//   - ClusterAddr 由两条独立配置项播种：cluster 模式来自 InitNodes[self]（在
//     internal/server/server.go 里剥过 "tcp://" 前缀），standalone 模式来自
//     cfgOptions.ServerAddr（原样透传）。二者天然可以文本不等，例如文档示例的
//     initNodes[self]="127.0.0.1:11110" + serverAddr="0.0.0.0:11110"。
//   - 如果按“文本差异即覆盖”触发，就会把源 A 播下去的正确值覆盖成源 B 的绑定
//     地址（如 0.0.0.0），然后全集群 addOrUpdateNodes 重连到这个不能用的地址。
//
// 本次修复的原始场景（standalone 首启后再补配 serverAddr）里，节点的 ClusterAddr
// 是空的，被此判定覆盖；而其它场景下值非空，此判定拒绝覆盖，从而不会掉进那个陷阱。
// ServerAddr 未配置时同样不提案，避免把空地址写回。
func shouldProposeClusterAddr(serverAddr string, localNode *types.Node) bool {
	if strings.TrimSpace(serverAddr) == "" {
		return false
	}
	if localNode == nil {
		return false
	}
	return strings.TrimSpace(localNode.ClusterAddr) == ""
}
