package web

import (
	"akvorado/reporter"
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/fsnotify/fsnotify"
)

//go:embed data/templates
var embeddedTemplates embed.FS

// baseData is the data to pass for all templates.
type templateBaseData struct {
	RootPath    string
	CurrentPath string
}

// loadTemplates reload the templates.
func (c *Component) loadTemplates() error {
	mainTemplate, err := template.
		New("main").
		Option("missingkey=error").
		Funcs(sprig.FuncMap()).
		Parse(`{{define "main" }}{{ template "base" . }}{{ end }}`)
	if err != nil {
		c.r.Err(err).Msg("unable to create main template")
		return fmt.Errorf("unable to create main template: %w", err)
	}

	layoutFiles := c.embedOrLiveFS(embeddedTemplates, "data/templates/layout")
	templateFiles := c.embedOrLiveFS(embeddedTemplates, "data/templates")
	compiled := make(map[string]*template.Template)
	entries, err := fs.ReadDir(templateFiles, ".")
	if err != nil {
		c.r.Err(err).Msg("unable to list template files")
		return fmt.Errorf("unable to list template files: %w", err)
	}
	for _, tpl := range entries {
		if tpl.IsDir() {
			continue
		}
		template, err := mainTemplate.Clone()
		if err != nil {
			c.r.Err(err).Msg("unable to clone main template")
			return fmt.Errorf("unable to clone main template: %w", err)
		}
		f, err := templateFiles.Open(tpl.Name())
		if err != nil {
			c.r.Err(err).Str("template", tpl.Name()).Msg("unable to open template")
			return fmt.Errorf("unable to open template %q: %w", tpl.Name(), err)
		}
		content, err := io.ReadAll(f)
		if err != nil {
			f.Close()
			c.r.Err(err).Str("template", tpl.Name()).Msg("unable to read template")
			return fmt.Errorf("unable to read template %q: %w", tpl.Name(), err)
		}
		f.Close()
		template, err = template.Parse(string(content))
		if err != nil {
			c.r.Err(err).Str("template", tpl.Name()).Msg("unable to parse template")
			return fmt.Errorf("unable to parse template %q: %w", tpl.Name(), err)
		}
		template, err = template.ParseFS(layoutFiles, "*.html")
		if err != nil {
			c.r.Err(err).Msg("unable to parse layout templates")
			return fmt.Errorf("unable to parse layout templates: %w", err)
		}
		compiled[tpl.Name()] = template
	}
	c.templatesLock.Lock()
	c.templates = compiled
	c.templatesLock.Unlock()
	return nil
}

// renderTemplate render the specified template
func (c *Component) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	c.templatesLock.RLock()
	tmpl, ok := c.templates[name]
	c.templatesLock.RUnlock()
	if !ok {
		c.r.Error().Str("template", name).Msg("template not found")
		http.Error(w, fmt.Sprintf("No template %q found.", name), http.StatusNotFound)
		return
	}

	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		c.r.Err(err).Str("template", name).Msg("error while rendering template")
		http.Error(w, fmt.Sprintf("Error while rendering %q.", name), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// watchTemplates monitor changes in template directories and reload them
func (c *Component) watchTemplates() error {
	if !c.config.ServeLiveFS {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.r.Err(err).Msg("cannot setup watcher for templates")
		return fmt.Errorf("cannot setup watcher: %w", err)
	}
	for _, dir := range []string{"templates", "templates/layout"} {
		_, base, _, _ := runtime.Caller(0)
		dir = filepath.Join(path.Dir(base), "data", dir)
		if err := watcher.Add(dir); err != nil {
			c.r.Err(err).Str("directory", dir).Msg("cannot watch template directory")
			return fmt.Errorf("cannot watch template directory %q: %w", dir, err)
		}
	}
	c.t.Go(func() error {
		defer watcher.Close()
		errLogger := c.r.Sample(reporter.BurstSampler(10*time.Second, 1))
		timer := time.NewTimer(100 * time.Millisecond)

		for {
			select {
			case <-c.t.Dying():
				return nil
			case err := <-watcher.Errors:
				errLogger.Err(err).Msg("error from watcher")
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				timer.Stop()
				timer.Reset(500 * time.Millisecond)
			case <-timer.C:
				c.r.Info().Msg("reload templates")
				c.loadTemplates() // errors are ignored
			}
		}
	})
	return nil
}
