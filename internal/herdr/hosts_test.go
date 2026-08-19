package herdr

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandLocalHostCallsHerdrDirectly: the empty host is this
// machine, so the call is the herdr binary with the bare args.
func TestCommandLocalHostCallsHerdrDirectly(t *testing.T) {
	name, argv := command("", []string{"agent", "list"})
	assert.Equal(t, "herdr", name)
	assert.Equal(t, []string{"agent", "list"}, argv)
}

// TestCommandRemoteHostWrapsInSSH: a named host is reached as
// `ssh <host> herdr agent list`, the read Phase 1 fans out.
func TestCommandRemoteHostWrapsInSSH(t *testing.T) {
	name, argv := command("box", []string{"agent", "list"})
	assert.Equal(t, "ssh", name)
	assert.Equal(t, []string{"box", "herdr", "agent", "list"}, argv)
}

// cannedAgents is one host's `herdr agent list` reply, naming its pane
// after the host so a test can tell whose panes came back.
func cannedAgents(host string) []byte {
	return []byte(`{"result":{"agents":[
		{"agent":"claude","agent_status":"working",
		 "cwd":"/w","pane_id":"` + host + `:p1"}]}}`)
}

// TestListHostsProbesConcurrently is the phase gate: a serial walk
// blocks on the first host forever, so every host must be in-flight at
// once for the read to make progress. Each fake exec announces itself
// and then waits on a shared gate; the test releases it only after all
// hosts have announced. A serial ListHosts would send one entry and
// deadlock, tripping the timeout.
func TestListHostsProbesConcurrently(t *testing.T) {
	hosts := []Host{"a", "b", "c"}

	entered := make(chan struct{}, len(hosts))
	release := make(chan struct{})
	exec := func(name string, args ...string) ([]byte, error) {
		entered <- struct{}{}
		<-release

		return cannedAgents("h"), nil
	}

	done := make(chan []HostResult, 1)
	go func() { done <- ListHosts(hosts, exec) }()

	for range hosts {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("hosts were probed serially, not concurrently")
		}
	}
	close(release)

	results := <-done
	assert.Len(t, results, len(hosts))
}

// TestListHostsCollectsPanesFromEveryHost keeps the host dimension:
// one result per host, tagged with the host and carrying its panes.
func TestListHostsCollectsPanesFromEveryHost(t *testing.T) {
	hosts := []Host{"", "box"}
	exec := func(name string, args ...string) ([]byte, error) {
		if name == "ssh" {
			return cannedAgents("box"), nil
		}

		return cannedAgents("local"), nil
	}

	results := ListHosts(hosts, exec)
	require.Len(t, results, 2)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Host < results[j].Host
	})
	assert.Equal(t, Host(""), results[0].Host)
	require.Len(t, results[0].Panes, 1)
	assert.Equal(t, "local:p1", results[0].Panes[0].PaneID)

	assert.Equal(t, Host("box"), results[1].Host)
	require.Len(t, results[1].Panes, 1)
	assert.Equal(t, "box:p1", results[1].Panes[0].PaneID)
}

// TestListHostsKeepsPerHostErrors: an unreachable host carries its own
// error in its own result rather than failing the whole read, so a
// reachable host beside it still reports its panes.
func TestListHostsKeepsPerHostErrors(t *testing.T) {
	hosts := []Host{"dead", "live"}
	boom := errors.New("ssh: connect: no route to host")
	exec := func(name string, args ...string) ([]byte, error) {
		if args[0] == "dead" {
			return nil, boom
		}

		return cannedAgents("live"), nil
	}

	results := ListHosts(hosts, exec)
	require.Len(t, results, 2)

	byHost := map[Host]HostResult{}
	for _, r := range results {
		byHost[r.Host] = r
	}

	assert.ErrorIs(t, byHost["dead"].Err, boom)
	assert.Empty(t, byHost["dead"].Panes)

	require.NoError(t, byHost["live"].Err)
	require.Len(t, byHost["live"].Panes, 1)
}
