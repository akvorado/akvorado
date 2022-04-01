package clickhouse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"strings"
	"text/template"
	"time"
)

// templatedFS is a wrapper around fs.FS to automatically expand templates
type templatedFS struct {
	data interface{}
	base fs.FS
}

// templatedFile is a wrapper around fs.File to automatically expand templates
type templatedFile struct {
	base     fs.File
	offset   int
	rendered []byte
}

// templatedDirEntry is a wrapper around fs.DirEntry to automatically expand templates
type templatedDirEntry struct {
	base fs.DirEntry
}

// templatedFileInfo is a wrapper around fs.FileInfo to automatically expand templates
type templatedFileInfo struct {
	base fs.FileInfo
}

func (tf *templatedFile) Stat() (fs.FileInfo, error) {
	info, err := tf.base.Stat()
	if err != nil {
		return nil, err
	}
	return &templatedFileInfo{info}, nil
}
func (tf *templatedFile) Read(buf []byte) (int, error) {
	if tf.offset >= len(tf.rendered) {
		return 0, io.EOF
	}
	n := copy(buf, tf.rendered[tf.offset:])
	tf.offset += n
	return n, nil
}
func (tf *templatedFile) Close() error {
	return tf.base.Close()
}

func (tfs *templatedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(tfs.base, name)
	if err != nil {
		return nil, err
	}
	results := []fs.DirEntry{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmpl") {
			results = append(results, &templatedDirEntry{entry})
		} else {
			results = append(results, entry)
		}
	}
	return results, nil
}

func (tde *templatedDirEntry) Name() string {
	return strings.TrimSuffix(tde.base.Name(), ".tmpl")
}
func (tde *templatedDirEntry) IsDir() bool {
	return tde.base.IsDir()
}
func (tde *templatedDirEntry) Type() fs.FileMode {
	return tde.base.Type()
}
func (tde *templatedDirEntry) Info() (fs.FileInfo, error) {
	info, err := tde.base.Info()
	if err != nil {
		return nil, err
	}
	return &templatedFileInfo{info}, nil
}

func (tfi *templatedFileInfo) Name() string {
	return strings.TrimSuffix(tfi.base.Name(), ".tmpl")
}
func (tfi *templatedFileInfo) Size() int64 {
	return 0 // Can't be sure
}
func (tfi *templatedFileInfo) Mode() fs.FileMode {
	return tfi.base.Mode()
}
func (tfi *templatedFileInfo) ModTime() time.Time {
	return tfi.base.ModTime()
}
func (tfi *templatedFileInfo) IsDir() bool {
	return tfi.base.IsDir()
}
func (tfi *templatedFileInfo) Sys() interface{} {
	return nil
}

func (tfs *templatedFS) Open(name string) (fs.File, error) {
	candidates := []string{fmt.Sprintf("%s.tmpl", name), name}
	var f fs.File
	var err error
	var candidate string
	for _, candidate = range candidates {
		f, err = tfs.base.Open(candidate)
		if err != nil && errors.Is(err, fs.ErrNotExist) {
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("unable to stat file: %w", err)
	}
	if info.IsDir() {
		panic("assumed that Open() won't be called for a directory")
	}
	if !strings.HasSuffix(candidate, ".tmpl") {
		return f, nil
	}

	// Render template
	tmpl, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	t, err := template.New("anything").Option("missingkey=error").Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("cannot parse template: %w", err)
	}
	b := bytes.NewBuffer([]byte{})
	if err := t.Execute(b, tfs.data); err != nil {
		return nil, fmt.Errorf("cannot execute template: %w", err)
	}
	return &templatedFile{base: f, rendered: b.Bytes()}, nil
}
