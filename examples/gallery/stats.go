package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// The gallery counts visits in Umami under the website's site, tagged as
// the gallery client and carrying a random ID kept in the storage, so
// that one install is one visitor. Builds of nsgo dev (statsBuild) and
// test binaries report nothing, and the settings turn it off.
const (
	statsURL     = "https://umami.oneo.me/api/send"
	statsWebsite = "4c29c1e7-28ea-4d0c-8297-b22d29bb1c41"
	statsHost    = "oneo-me.github.io"
	statsTag     = "gallery"
)

// Storage keys of the statistics: the install's ID, and statsKey "off"
// when the settings turned them off.
const (
	installKey = "installID"
	statsKey   = "stats"
)

// Umami measures a visit from its first event to its last, so the gallery
// sends a heartbeat each statsBeat, for statsIdle after the last thing
// done in it: a window left open does not count as use.
const (
	statsBeat = time.Minute
	statsIdle = 5 * time.Minute
)

// statsOn reports whether the settings leave the statistics on.
func statsOn(cx *core.Scope) bool {
	v, _ := cx.Storage().GetItem(statsKey)
	return v != "off"
}

// setStatsOn turns the statistics on or off.
func setStatsOn(cx *core.Scope, on bool) {
	if on {
		cx.Storage().RemoveItem(statsKey)
	} else {
		cx.Storage().SetItem(statsKey, "off")
	}
}

// statsPayload is an event of Umami's /api/send: a pageview without a
// name.
type statsPayload struct {
	Website  string `json:"website"`
	Hostname string `json:"hostname"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Language string `json:"language,omitempty"`
	Screen   string `json:"screen,omitempty"`
	Tag      string `json:"tag"`
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
}

// stats sends the gallery's events; a nil *stats sends nothing.
type stats struct {
	cx     *core.Scope
	id     string
	queue  chan statsPayload
	active time.Time
}

// reportUsage starts the statistics and counts this install as a visit,
// or returns nil in builds and tests that report nothing. Events are
// made on the graph's goroutine and sent from another, oldest first;
// one that fails is dropped.
func reportUsage(cx *core.Scope, s *state) *stats {
	if !statsBuild || testing.Testing() {
		return nil
	}
	store := cx.Storage()
	id, ok := store.GetItem(installKey)
	if !ok {
		b := make([]byte, 16)
		rand.Read(b)
		id = hex.EncodeToString(b)
		store.SetItem(installKey, id)
	}
	st := &stats{cx: cx, id: id, queue: make(chan statsPayload, 32), active: time.Now()}
	go sendStats(st.queue, statsAgent(cx.Env().OS()))
	st.send("")
	cx.Effect(func() {
		s.selected()
		s.query.Get()
		core.Untracked(cx, func() bool {
			st.active = time.Now()
			return true
		})
	})
	var beat func()
	beat = func() {
		if time.Since(st.active) < statsIdle {
			st.send("heartbeat")
		}
		cx.AfterFunc(statsBeat, beat)
	}
	cx.AfterFunc(statsBeat, beat)
	return st
}

// send queues an event, a pageview when name is empty, unless the
// settings turned the statistics off.
func (st *stats) send(name string) {
	if !statsOn(st.cx) {
		return
	}
	env := st.cx.Env()
	w := env.Window()
	p := statsPayload{Website: statsWebsite, Hostname: statsHost, URL: "/", Title: "Kubai Icons",
		Language: env.Locale(), Screen: fmt.Sprintf("%dx%d", int(w.Width), int(w.Height)),
		Tag: statsTag, ID: st.id, Name: name}
	select {
	case st.queue <- p:
	default:
	}
}

// sendStats posts the queue's events to Umami until it closes.
func sendStats(queue <-chan statsPayload, agent string) {
	client := &http.Client{Timeout: 10 * time.Second}
	for p := range queue {
		body, _ := json.Marshal(map[string]any{"type": "event", "payload": p})
		req, err := http.NewRequest(http.MethodPost, statsURL, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if agent != "" {
			req.Header.Set("User-Agent", agent)
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
	}
}

// statsAgent is the User-Agent a native gallery sends, as a browser on
// the platform would say it, for Umami to tell the system and not take it
// for a bot as it does Go's own. The web's browser sends its own.
func statsAgent(os core.OS) string {
	platforms := map[core.OS]string{
		core.OSMacOS:   "Macintosh; Intel Mac OS X 10_15_7",
		core.OSWindows: "Windows NT 10.0; Win64; x64",
		core.OSLinux:   "X11; Linux x86_64",
		core.OSIOS:     "iPhone; CPU iPhone OS 18_0 like Mac OS X",
		core.OSAndroid: "Linux; Android 15",
	}
	p, ok := platforms[os]
	if !ok {
		return ""
	}
	return "Mozilla/5.0 (" + p + ") KubaiIconsGallery/1.0"
}
