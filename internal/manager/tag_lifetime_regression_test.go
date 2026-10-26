package manager

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/WuKongIM/WuKongIM/internal/options"
	"github.com/WuKongIM/WuKongIM/internal/service"
	"github.com/stretchr/testify/require"
)

func lifetimeManager(t *testing.T) *TagManager {
	old := options.G
	options.G = options.New()
	t.Cleanup(func() { options.G = old })
	return newSnapshotTestTagManager(t, []string{"fixed", "removed"})
}

// Exercise the new production operation rather than the obsolete three-call API.
func TestTagLifetime_AtomicUpdateKeepsInflightSnapshot(t *testing.T) {
	tm := lifetimeManager(t)
	old := tm.Get("tag")
	require.NoError(t, tm.UpdateChannelTagUsers("g1", 2, []string{"removed"}, true))
	key := tm.GetChannelTag("g1", 2)
	require.NotEqual(t, "tag", key)
	require.Equal(t, []string{"fixed", "removed"}, tm.Get("tag").GetUsers())
	require.Equal(t, old, tm.Get("tag"))
	require.Equal(t, []string{"fixed"}, tm.Get(key).GetUsers())
	require.NoError(t, tm.UpdateChannelTagUsers("g1", 2, []string{"new"}, false))
	require.Equal(t, []string{"fixed"}, tm.Get(key).GetUsers())
	require.Equal(t, []string{"fixed", "new"}, tm.Get(tm.GetChannelTag("g1", 2)).GetUsers())
}

func TestTagLifetime_ConcurrentUpdatesDoNotLoseMembers(t *testing.T) {
	tm := lifetimeManager(t)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- tm.UpdateChannelTagUsers("g1", 2, []string{fmt.Sprint(i)}, false)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Len(t, tm.Get(tm.GetChannelTag("g1", 2)).GetUsers(), 42)
	require.Equal(t, []string{"fixed", "removed"}, tm.Get("tag").GetUsers())
}

func TestTagLifetime_RetiredVersionExpiresDespiteHotReads(t *testing.T) {
	tm := lifetimeManager(t)
	require.NoError(t, tm.UpdateChannelTagUsers("g1", 2, []string{"new"}, false))
	b := tm.getBlucketByTagKey("tag")
	b.tag.Lock()
	b.tag.retired["tag"] = time.Now().Add(-2 * b.expire)
	b.tag.Unlock()
	// Both versions are hot. Retirement must not be extended by shared access state.
	require.NotNil(t, tm.Get("tag"))
	current := tm.GetChannelTag("g1", 2)
	require.NotNil(t, tm.Get(current))
	b.checkExpireTags()
	require.Nil(t, tm.Get("tag"))
	require.NotNil(t, tm.Get(current))
}

func TestTagLifetime_NoOpDoesNotCreateVersion(t *testing.T) {
	tm := lifetimeManager(t)
	require.NoError(t, tm.UpdateChannelTagUsers("g1", 2, []string{"fixed", "fixed"}, false))
	require.Equal(t, "tag", tm.GetChannelTag("g1", 2))
	require.Len(t, tm.GetAllTags(), 1)
}

// Routing failure must leave both published membership and mapping unchanged.
type unavailableTagCluster struct{ snapshotTestCluster }

func (unavailableTagCluster) SlotLeaderId(uint32) uint64 { return 0 }
func TestTagLifetime_FailedUpdateKeepsPublishedVersion(t *testing.T) {
	tm := lifetimeManager(t)
	oldCluster := service.Cluster
	service.Cluster = unavailableTagCluster{}
	t.Cleanup(func() { service.Cluster = oldCluster })
	require.Error(t, tm.UpdateChannelTagUsers("g1", 2, []string{"new"}, false))
	require.Equal(t, "tag", tm.GetChannelTag("g1", 2))
	require.Equal(t, []string{"fixed", "removed"}, tm.Get("tag").GetUsers())
	require.Len(t, tm.GetAllTags(), 1)
}
