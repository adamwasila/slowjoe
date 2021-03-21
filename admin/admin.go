package admin

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adamwasila/slowjoe/config"
	"github.com/sirupsen/logrus"
	"goji.io"
	"goji.io/pat"

	humanize "github.com/dustin/go-humanize"
)

//go:embed assets/templates/*
var templates embed.FS

func parseTemplates() (*template.Template, error) {
	var t *template.Template = template.New("")

	err := fs.WalkDir(templates, "assets/templates", func(fpath string, d fs.DirEntry, ierr error) error {
		if ierr != nil {
			return ierr
		}
		if d.IsDir() {
			return nil
		}
		name := path.Base(fpath)
		content, err := fs.ReadFile(templates, fpath)
		if err != nil {
			return err
		}
		t, err = t.New(name).Parse(string(content))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return t, err
}

func AddRoutes(mux *goji.Mux, data *AdminData) {
	mux.Handle(pat.Get("/"), Redirect())
	mux.Handle(pat.Get("/favicon.ico"), Assets())
	mux.Handle(pat.Get("/admin/connections.html"), ForTemplate("connections.html", data))
	mux.Handle(pat.Get("/admin/settings.html"), ForTemplate("settings.html", data))
	mux.Handle(pat.Get("/admin/connections/total"), ForTemplate("total.html", data))
	mux.Handle(pat.Get("/admin/connections/active"), ForTemplate("active.html", data))
	mux.Handle(pat.Get("/admin/connections"), ForTemplate("cards.html", data))
	mux.Handle(pat.Get("/*"), Assets())
}

//go:embed assets/data/*
var assets embed.FS

func Assets() http.HandlerFunc {
	dataFs, _ := fs.Sub(assets, "assets/data")
	fh := http.FileServer(http.FS(dataFs))
	return func(w http.ResponseWriter, r *http.Request) {
		fh.ServeHTTP(w, r)
	}
}

func Redirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/connections.html", http.StatusMovedPermanently)
	}
}

func ForTemplate(page string, adminData *AdminData) http.HandlerFunc {
	t, err := parseTemplates()
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		data := make(map[string]interface{})

		data["admin"] = adminData
		ui := Ui{
			Menu: []MenuItem{
				MenuItem{
					Name:    "connections.html",
					Label:   "Connections",
					Enabled: false,
				},
				MenuItem{
					Name:    "settings.html",
					Label:   "Settings",
					Enabled: false,
				},
			},
		}
		for i, item := range ui.Menu {
			if page == item.Name {
				ui.Menu[i].Enabled = true
			}
		}
		data["ui"] = ui

		adminData.RLock()
		defer adminData.RUnlock()

		err = t.ExecuteTemplate(w, page, data)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to generate admin page")
			//TODO: buffer body output so proper ISE error can be returned here
		}
	}
}

type Ui struct {
	Menu []MenuItem
}

type MenuItem struct {
	Name    string
	Label   string
	Enabled bool
}

type ConnData struct {
	Name                string
	Alias               string
	Type                string
	BytesSentUpstream   int
	BytesSentDownstream int
	Started             *time.Time
	Finished            *time.Time
}

type AdminData struct {
	Version           string
	Config            config.Config
	ConnectionsActive int32
	ConnectionsTotal  int32
	Connections       map[string]ConnData
	lock              sync.RWMutex
}

func (c ConnData) Since() string {
	if c.Started == nil {
		return "-"
	}
	return humanize.Time(*c.Started)
}

func (c ConnData) Until() string {
	if c.Finished == nil {
		return "-"
	}
	return humanize.Time(*c.Finished)
}

func NewAdminData() *AdminData {
	return &AdminData{
		Connections: make(map[string]ConnData),
	}
}

func (a *AdminData) ConnectionOpened(id, alias, typ string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	atomic.AddInt32(&a.ConnectionsActive, 1)
	atomic.AddInt32(&a.ConnectionsTotal, 1)
	t := time.Now()
	a.Connections[id] = ConnData{
		Name:    id,
		Alias:   alias,
		Type:    typ,
		Started: &t,
	}
}

func (a *AdminData) ConnectionProgressed(id, alias string, direction string, transferredBytes int) {
	a.lock.Lock()
	defer a.lock.Unlock()
	conn := a.Connections[id]
	if direction == config.DirUpstream {
		conn.BytesSentUpstream += transferredBytes
	}
	if direction == config.DirDownstream {
		conn.BytesSentDownstream += transferredBytes
	}
	a.Connections[id] = conn
}

func (a *AdminData) ConnectionDelayed(id, alias string, direction string, delay time.Duration) {
}

func (a *AdminData) ConnectionCompleted(id, alias, direction string, transferredBytes int, duration time.Duration) {
}

func (a *AdminData) ConnectionScheduledClose(id, alias string, delay time.Duration) {
}

func (*AdminData) ConnectionClosedUpstream(id, alias string) {
}

func (*AdminData) ConnectionClosedDownstream(id, alias string) {
}

func (a *AdminData) ConnectionClosed(id, alias string, d time.Duration) {
	t1 := time.Now()
	a.lock.Lock()
	defer a.lock.Unlock()
	atomic.AddInt32(&a.ConnectionsActive, -1)
	conn := a.Connections[id]
	conn.Finished = &t1
	a.Connections[id] = conn
	time.AfterFunc(1*time.Minute, func() {
		a.lock.Lock()
		defer a.lock.Unlock()
		delete(a.Connections, id)
	})
}

func (a *AdminData) RLock() {
	a.lock.RLock()
}

func (a *AdminData) RUnlock() {
	a.lock.RUnlock()
}
