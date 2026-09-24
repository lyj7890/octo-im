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

// shouldProposeClusterAddr 判定是否需要提案回填本节点的 cluster 通讯地址：
// 只有本地配置了 ServerAddr、且存储记录里的 ClusterAddr 与之不同时才提案。
// ServerAddr 为空（standalone 未配 serverAddr）时不提案，避免把空地址写回。
func shouldProposeClusterAddr(serverAddr string, localNode *types.Node) bool {
	if strings.TrimSpace(serverAddr) == "" {
		return false
	}
	return localNode != nil && localNode.ClusterAddr != serverAddr
}
