package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// dockerRouteAgent is a live command session that records what it is asked to
// run and answers the socket probe with the machine's own answer.
type dockerRouteAgent struct {
	mu        sync.Mutex
	commands  []string
	hasDocker bool
}

func (a *dockerRouteAgent) ranCommands() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.commands...)
}

func (a *dockerRouteAgent) serve(conn *websocket.Conn) {
	var writeMu sync.Mutex
	for {
		var message agentexec.Message
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if message.Type != agentexec.MsgTypeExecuteCmd {
			continue
		}
		var command agentexec.ExecuteCommandPayload
		if err := message.DecodePayload(&command); err != nil {
			return
		}
		a.mu.Lock()
		a.commands = append(a.commands, command.Command)
		hasDocker := a.hasDocker
		a.mu.Unlock()

		answer := "no"
		if hasDocker {
			answer = "yes"
		}
		reply, err := agentexec.NewMessage(agentexec.MsgTypeCommandResult, command.RequestID, agentexec.CommandResultPayload{
			RequestID: command.RequestID,
			Success:   true,
			Stdout:    "PULSE_DOCKER_SOCKET\t" + answer + "\n",
		})
		if err != nil {
			return
		}
		writeMu.Lock()
		err = conn.WriteJSON(reply)
		writeMu.Unlock()
		if err != nil {
			return
		}
	}
}

// dockerRouteRig is a Router and Monitor wired as in production: the socket
// probe travels from CheckContainersForDocker through the router's executor to
// a real agentexec.Server and real websocket sessions.
type dockerRouteRig struct {
	t       *testing.T
	state   *models.State
	monitor *monitoring.Monitor
	url     string
}

func newDockerRouteRig(t *testing.T, instances ...config.PVEInstance) *dockerRouteRig {
	t.Helper()
	server := agentexec.NewServerWithAdmissionValidator(
		func(token, agentID, hostname string) (agentexec.AgentAdmission, bool) {
			return agentexec.AgentAdmission{OrganizationID: "default", TokenID: token, AgentID: agentID, Hostname: hostname}, true
		},
		func(agentexec.AgentAdmission) bool { return true },
	)
	websocketServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	t.Cleanup(websocketServer.Close)

	state := models.NewState()
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "config", &config.Config{PVEInstances: instances})

	router := &Router{
		config:          &config.Config{EnableProxmoxGuestDockerInventory: true},
		agentExecServer: server,
	}
	router.configureProxmoxGuestDockerDetection(monitor)

	return &dockerRouteRig{t: t, state: state, monitor: monitor, url: websocketServer.URL}
}

// connectAgent opens a command session under sessionID. The telemetry host
// record is separate: Pulse can hold a host under an ID its session never
// registered with, so each test states both.
func (r *dockerRouteRig) connectAgent(sessionID, hostname, tokenID string, hasDocker bool) *dockerRouteAgent {
	r.t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(r.url, "http"), http.Header{"Origin": []string{r.url}})
	if err != nil {
		r.t.Fatalf("dial agent websocket: %v", err)
	}
	r.t.Cleanup(func() { conn.Close() })
	message, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  sessionID,
		Hostname: hostname,
		Token:    tokenID,
	})
	if err != nil {
		r.t.Fatalf("create registration message: %v", err)
	}
	if err := conn.WriteJSON(message); err != nil {
		r.t.Fatalf("write registration: %v", err)
	}
	var response agentexec.Message
	if err := conn.ReadJSON(&response); err != nil {
		r.t.Fatalf("read registration response: %v", err)
	}
	var registered agentexec.RegisteredPayload
	if err := response.DecodePayload(&registered); err != nil || !registered.Success {
		r.t.Fatalf("registration failed: payload=%+v err=%v", registered, err)
	}
	agent := &dockerRouteAgent{hasDocker: hasDocker}
	go agent.serve(conn)
	return agent
}

func dockerRouteInstance(name, address string) config.PVEInstance {
	return config.PVEInstance{Name: name, Host: "https://" + address + ":8006"}
}

func dockerRouteNode(instance, name, address string) models.Node {
	return models.Node{
		ID: instance + "-" + name, NodeIdentity: instance + "-" + name, Name: name, Instance: instance,
		Host: "https://" + address + ":8006", Status: "online", Type: "node", LastSeen: time.Now(),
	}
}

func dockerRouteHost(id, hostname, tokenID, address string) models.Host {
	return models.Host{ID: id, Hostname: hostname, TokenID: tokenID, ReportIP: address, Status: "online", LastSeen: time.Now()}
}

func dockerRouteGuest(instance, node string, vmid int) models.Container {
	return models.Container{
		ID:       fmt.Sprintf("%s:%s:%d", instance, node, vmid),
		VMID:     vmid,
		Name:     fmt.Sprintf("ct%d", vmid),
		Node:     node,
		Instance: instance,
		Status:   "running",
		Type:     "lxc",
	}
}

func probeDocker(rig *dockerRouteRig, guest models.Container) models.Container {
	rig.t.Helper()
	return rig.monitor.CheckContainersForDocker(context.Background(), []models.Container{guest})[0]
}

func TestProxmoxGuestDockerProbesStayOnTheGuestsOwnNode(t *testing.T) {
	siteA := dockerRouteInstance("site-a", "10.0.0.5")
	siteB := dockerRouteInstance("site-b", "10.1.0.5")

	t.Run("a same-named node with no agent of its own is not probed through the other site's", func(t *testing.T) {
		rig := newDockerRouteRig(t, siteA, siteB)
		rig.state.UpsertHost(dockerRouteHost("agent-a", "pve", "token-a", "10.0.0.5"))
		rig.state.UpdateNodesForInstance("site-a", []models.Node{dockerRouteNode("site-a", "pve", "10.0.0.5")})
		rig.state.UpdateNodesForInstance("site-b", []models.Node{dockerRouteNode("site-b", "pve", "10.1.0.5")})
		agentA := rig.connectAgent("agent-a", "pve", "token-a", true)

		guest := probeDocker(rig, dockerRouteGuest("site-b", "pve", 101))
		if got := agentA.ranCommands(); len(got) != 0 {
			t.Fatalf("site-b's guest was probed through site-a's agent: %q", got)
		}
		if guest.HasDocker {
			t.Fatalf("site-b's guest took site-a's answer: %+v", guest)
		}

		own := probeDocker(rig, dockerRouteGuest("site-a", "pve", 101))
		if got := agentA.ranCommands(); len(got) != 1 || !strings.Contains(got[0], "pct exec 101 ") {
			t.Fatalf("site-a's own guest was not probed through its agent: %q", got)
		}
		if !own.HasDocker {
			t.Fatalf("site-a's guest lost its agent's answer: %+v", own)
		}
	})

	t.Run("inventory stays on the guest's own node too", func(t *testing.T) {
		rig := newDockerRouteRig(t, siteA, siteB)
		rig.state.UpsertHost(dockerRouteHost("agent-a", "pve", "token-a", "10.0.0.5"))
		rig.state.UpdateNodesForInstance("site-a", []models.Node{dockerRouteNode("site-a", "pve", "10.0.0.5")})
		rig.state.UpdateNodesForInstance("site-b", []models.Node{dockerRouteNode("site-b", "pve", "10.1.0.5")})
		agentA := rig.connectAgent("agent-a", "pve", "token-a", true)

		guest := dockerRouteGuest("site-b", "pve", 101)
		guest.HasDocker = true
		rig.monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{guest})
		if got := agentA.ranCommands(); len(got) != 0 {
			t.Fatalf("site-b's inventory ran through site-a's agent: %q", got)
		}

		guest = dockerRouteGuest("site-a", "pve", 101)
		guest.HasDocker = true
		rig.monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{guest})
		if got := agentA.ranCommands(); len(got) != 1 || !strings.Contains(got[0], "pct exec 101 ") {
			t.Fatalf("site-a's own inventory did not run through its agent: %q", got)
		}
	})

	t.Run("two same-named nodes each answer through their own agent", func(t *testing.T) {
		rig := newDockerRouteRig(t, siteA, siteB)
		rig.state.UpsertHost(dockerRouteHost("agent-a", "pve.site-a.lan", "token-a", "10.0.0.5"))
		rig.state.UpsertHost(dockerRouteHost("agent-b", "pve.site-b.lan", "token-b", "10.1.0.5"))
		rig.state.UpdateNodesForInstance("site-a", []models.Node{dockerRouteNode("site-a", "pve", "10.0.0.5")})
		rig.state.UpdateNodesForInstance("site-b", []models.Node{dockerRouteNode("site-b", "pve", "10.1.0.5")})
		agentA := rig.connectAgent("agent-a", "pve.site-a.lan", "token-a", true)
		agentB := rig.connectAgent("agent-b", "pve.site-b.lan", "token-b", false)

		onA := probeDocker(rig, dockerRouteGuest("site-a", "pve", 101))
		onB := probeDocker(rig, dockerRouteGuest("site-b", "pve", 101))

		if got := agentA.ranCommands(); len(got) != 1 {
			t.Fatalf("site-a's agent ran %q, want only its own guest's probe", got)
		}
		if got := agentB.ranCommands(); len(got) != 1 {
			t.Fatalf("site-b's agent ran %q, want only its own guest's probe", got)
		}
		if !onA.HasDocker || onB.HasDocker {
			t.Fatalf("answers crossed the sites: site-a=%v site-b=%v, want true and false", onA.HasDocker, onB.HasDocker)
		}
	})

	t.Run("an agent refused a session never borrows another machine's through a shared token", func(t *testing.T) {
		// The command server admits one session per hostname, so site-b's agent
		// has none while site-a's holds it, and both enrolled with one token.
		rig := newDockerRouteRig(t, siteA, siteB)
		rig.state.UpsertHost(dockerRouteHost("agent-a", "pve", "shared-token", "10.0.0.5"))
		rig.state.UpsertHost(dockerRouteHost("agent-b", "pve", "shared-token", "10.1.0.5"))
		rig.state.UpdateNodesForInstance("site-a", []models.Node{dockerRouteNode("site-a", "pve", "10.0.0.5")})
		rig.state.UpdateNodesForInstance("site-b", []models.Node{dockerRouteNode("site-b", "pve", "10.1.0.5")})
		agentA := rig.connectAgent("agent-a", "pve", "shared-token", true)

		guest := probeDocker(rig, dockerRouteGuest("site-b", "pve", 101))
		if got := agentA.ranCommands(); len(got) != 0 {
			t.Fatalf("site-b's guest was probed through site-a's session: %q", got)
		}
		if guest.HasDocker || !guest.DockerCheckedAt.IsZero() {
			t.Fatalf("site-b's guest took an answer: %+v", guest)
		}
	})

	t.Run("a session bound to an earlier ID is found through its token", func(t *testing.T) {
		rig := newDockerRouteRig(t, dockerRouteInstance("home", "192.168.1.5"))
		rig.state.UpsertHost(dockerRouteHost("agent-home-renamed", "pve", "token-home", "192.168.1.5"))
		rig.state.UpdateNodesForInstance("home", []models.Node{dockerRouteNode("home", "pve", "192.168.1.5")})
		agent := rig.connectAgent("agent-home", "pve", "token-home", true)

		guest := probeDocker(rig, dockerRouteGuest("home", "pve", 200))
		if got := agent.ranCommands(); len(got) != 1 {
			t.Fatalf("session under an earlier ID ran %q", got)
		}
		if !guest.HasDocker {
			t.Fatalf("lost the session's answer: %+v", guest)
		}
	})

	t.Run("a token session presenting another hostname is not this agent", func(t *testing.T) {
		rig := newDockerRouteRig(t, dockerRouteInstance("home", "192.168.1.5"))
		rig.state.UpsertHost(dockerRouteHost("agent-home", "pve", "token-home", "192.168.1.5"))
		rig.state.UpdateNodesForInstance("home", []models.Node{dockerRouteNode("home", "pve", "192.168.1.5")})
		other := rig.connectAgent("agent-other", "backup", "token-home", true)

		probeDocker(rig, dockerRouteGuest("home", "pve", 200))
		if got := other.ranCommands(); len(got) != 0 {
			t.Fatalf("another machine's session ran the probe: %q", got)
		}
	})

	t.Run("a manual link routes an agent whose hostname differs from the node", func(t *testing.T) {
		rig := newDockerRouteRig(t, dockerRouteInstance("home", "192.168.1.5"))
		rig.state.UpdateNodesForInstance("home", []models.Node{dockerRouteNode("home", "pve", "192.168.1.5")})
		rig.state.UpsertHost(dockerRouteHost("agent-box", "proxmox-box", "token-box", "192.168.9.9"))
		if err := rig.state.LinkHostAgentToNode("agent-box", "home-pve"); err != nil {
			t.Fatalf("LinkHostAgentToNode: %v", err)
		}
		agent := rig.connectAgent("agent-box", "proxmox-box", "token-box", true)

		guest := probeDocker(rig, dockerRouteGuest("home", "pve", 200))
		if got := agent.ranCommands(); len(got) != 1 || !guest.HasDocker {
			t.Fatalf("manually linked agent: ran %q, guest %+v", got, guest)
		}
	})

	t.Run("an agent that does not report the address Pulse connects through needs a manual link", func(t *testing.T) {
		// Reached through NAT, the node's endpoint is not one of the agent's
		// addresses. That is the same evidence as a different machine of the same
		// name, so the node poll drops the automatic link and nothing is probed
		// until the operator says they are one machine.
		rig := newDockerRouteRig(t, dockerRouteInstance("home", "203.0.113.10"))
		node := dockerRouteNode("home", "pve", "203.0.113.10")
		rig.state.UpdateNodesForInstance("home", []models.Node{node})
		host := dockerRouteHost("agent-home", "pve", "token-home", "192.168.1.5")
		host.LinkedNodeID = node.ID
		host.NodeLinkSource = "automatic"
		rig.state.UpsertHost(host)
		rig.state.LinkNodeToHostAgent(node.ID, host.ID)
		rig.state.UpdateNodesForInstance("home", []models.Node{node})
		agent := rig.connectAgent("agent-home", "pve", "token-home", true)

		probeDocker(rig, dockerRouteGuest("home", "pve", 200))
		if got := agent.ranCommands(); len(got) != 0 {
			t.Fatalf("probed without a proven link: %q", got)
		}

		if err := rig.state.SetHostNodeLinkIntent(host.ID, node.ID, nil); err != nil {
			t.Fatalf("SetHostNodeLinkIntent: %v", err)
		}
		rig.state.UpdateNodesForInstance("home", []models.Node{node})
		guest := probeDocker(rig, dockerRouteGuest("home", "pve", 200))
		if got := agent.ranCommands(); len(got) != 1 || !guest.HasDocker {
			t.Fatalf("manual link across a node poll: ran %q, guest %+v", got, guest)
		}
	})

	t.Run("a session under another agent's ID is never borrowed through its rotated token", func(t *testing.T) {
		rig := newDockerRouteRig(t, siteA, siteB)
		rig.state.UpsertHost(dockerRouteHost("agent-a", "pve", "token-a-rotated", "10.0.0.5"))
		rig.state.UpsertHost(dockerRouteHost("agent-b", "pve", "shared-token", "10.1.0.5"))
		rig.state.UpdateNodesForInstance("site-a", []models.Node{dockerRouteNode("site-a", "pve", "10.0.0.5")})
		rig.state.UpdateNodesForInstance("site-b", []models.Node{dockerRouteNode("site-b", "pve", "10.1.0.5")})
		agentA := rig.connectAgent("agent-a", "pve", "shared-token", true)

		probeDocker(rig, dockerRouteGuest("site-b", "pve", 101))
		if got := agentA.ranCommands(); len(got) != 0 {
			t.Fatalf("site-b's guest was probed through site-a's session: %q", got)
		}
	})

	t.Run("one installation probes through its node's agent as before", func(t *testing.T) {
		rig := newDockerRouteRig(t, dockerRouteInstance("home", "192.168.1.5"))
		rig.state.UpsertHost(dockerRouteHost("agent-home", "pve", "token-home", "192.168.1.5"))
		rig.state.UpdateNodesForInstance("home", []models.Node{dockerRouteNode("home", "pve", "192.168.1.5")})
		agent := rig.connectAgent("agent-home", "pve", "token-home", true)

		guest := probeDocker(rig, dockerRouteGuest("home", "pve", 200))
		if got := agent.ranCommands(); len(got) != 1 || !strings.Contains(got[0], "pct exec 200 ") {
			t.Fatalf("single installation's probe = %q", got)
		}
		if !guest.HasDocker {
			t.Fatalf("single installation lost its agent's answer: %+v", guest)
		}
	})
}
