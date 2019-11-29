package admin

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adamwasila/slowjoe/config"
	"github.com/markbates/pkger"
	"github.com/sirupsen/logrus"
	"goji.io"
	"goji.io/pat"
)

func getAsset(filename string) (string, error) {
	f, err := pkger.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseTemplates() (*template.Template, error) {
	var t *template.Template = template.New("")
	err := pkger.Walk("/assets/templates", func(fpath string, info os.FileInfo, ierr error) error {
		if ierr != nil {
			return ierr
		}
		if info.IsDir() {
			return nil
		}
		name := path.Base(fpath)
		content, err := getAsset(fpath)
		if err != nil {
			return err
		}
		t, err = t.New(name).Parse(content)
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
	mux.Handle(pat.Get("/*"), Assets())
}

func Assets() http.HandlerFunc {
	fh := http.FileServer(pkger.Dir("/assets/data"))
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
}

type AdminData struct {
	Version           string
	Config            config.Config
	ConnectionsActive int32
	ConnectionsTotal  int32
	Connections       map[string]ConnData
	lock              sync.RWMutex
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
	a.Connections[id] = ConnData{
		id,
		alias,
		typ,
		0,
		0,
	}
}

func (a *AdminData) ConnectionProgressed(id string, direction string, transferredBytes int) {
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

func (*AdminData) ConnectionClosedUpstream(id string)   {}
func (*AdminData) ConnectionClosedDownstream(id string) {}
func (a *AdminData) ConnectionClosed(id string, d time.Duration) {
	a.lock.Lock()
	defer a.lock.Unlock()
	atomic.AddInt32(&a.ConnectionsActive, -1)
	delete(a.Connections, id)
}

func (a *AdminData) RLock() {
	a.lock.RLock()
}

func (a *AdminData) RUnlock() {
	a.lock.RUnlock()
}
