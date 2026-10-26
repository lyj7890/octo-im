package handler

import (
	"github.com/WuKongIM/WuKongIM/internal/eventbus"
	"github.com/WuKongIM/WuKongIM/internal/options"
	"github.com/WuKongIM/WuKongIM/internal/service"
	"github.com/WuKongIM/WuKongIM/internal/types"
	wkproto "github.com/WuKongIM/WuKongIMGoProto"
	"github.com/stretchr/testify/require"
	"testing"
)

type batchTagStore struct {
	service.ITagManager
	tags    map[string]*types.Tag
	current string
}

func (m *batchTagStore) Get(key string) *types.Tag          { return m.tags[key] }
func (m *batchTagStore) GetChannelTag(string, uint8) string { return m.current }
func (m *batchTagStore) RemoveTag(key string)               { delete(m.tags, key) }

// A target node may batch consecutive same-channel distribution events
// stamped with different membership versions. The normal distributor must
// not replace every event's key with the first event's key.
func TestTagBatch_EachMessageUsesOwnTag(t *testing.T) {
	oldOptions, oldTags, oldPresence, oldUser, oldPusher := options.G, service.TagManager, service.Presence, eventbus.User, eventbus.Pusher
	t.Cleanup(func() {
		options.G, service.TagManager, service.Presence, eventbus.User, eventbus.Pusher = oldOptions, oldTags, oldPresence, oldUser, oldPusher
	})
	options.G = options.New()
	options.G.Cluster.NodeId = 2
	service.Presence = nil
	service.TagManager = &batchTagStore{current: "tagA", tags: map[string]*types.Tag{
		"tagA": {Key: "tagA", Nodes: []*types.Node{{LeaderId: 2, Uids: []string{"fixed", "removed"}}}},
		"tagB": {Key: "tagB", Nodes: []*types.Node{{LeaderId: 2, Uids: []string{"fixed", "new"}}}},
	}}
	users := &distributionUsers{conns: map[string][]*eventbus.Conn{}}
	for _, uid := range []string{"fixed", "removed", "new"} {
		users.conns[uid] = []*eventbus.Conn{{Uid: uid, NodeId: 2, ConnId: 7, Auth: true, DeviceLevel: wkproto.DeviceLevelMaster}}
	}
	eventbus.RegisterUser(users)
	pusher := &distributionPusher{}
	eventbus.RegisterPusher(pusher)
	makeEvent := func(mid int64, key string) *eventbus.Event {
		return &eventbus.Event{
			Type: eventbus.EventChannelDistribute, MessageId: mid, TagKey: key,
			Conn: &eventbus.Conn{Uid: "sender", NodeId: 1, ConnId: 9}, Frame: &wkproto.SendPacket{},
		}
	}
	ctx := &eventbus.ChannelContext{ChannelId: "room", ChannelType: wkproto.ChannelTypeData, SlotLeaderId: 1, Events: []*eventbus.Event{makeEvent(101, "tagA"), makeEvent(102, "tagB"), makeEvent(103, "tagA")}}
	NewHandler().distributeCommon(ctx)
	got := map[int64][]string{}
	for _, e := range pusher.events {
		if e.Type == eventbus.EventPushOnline {
			got[e.MessageId] = append(got[e.MessageId], e.ToUid)
		}
	}
	require.ElementsMatch(t, []string{"fixed", "removed"}, got[101])
	require.ElementsMatch(t, []string{"fixed", "new"}, got[102], "second message must use tagB, rather than tagA from the first event")
	require.Equal(t, "tagB", ctx.Events[1].TagKey)
	require.ElementsMatch(t, []string{"fixed", "removed"}, got[103])
	var sequence []int64
	for _, e := range pusher.events {
		if e.Type == eventbus.EventPushOnline && e.ToUid == "fixed" {
			sequence = append(sequence, e.MessageId)
		}
	}
	require.Equal(t, []int64{101, 102, 103}, sequence)
}
