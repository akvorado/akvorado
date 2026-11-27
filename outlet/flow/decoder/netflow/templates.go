// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"strconv"
	"sync"

	"github.com/netsampler/goflow2/v3/decoders/netflow"
)

// templateAndOptionCollection map exporters to the set of templates and options we
// received from them.
type templateAndOptionCollection struct {
	nd   *Decoder
	lock sync.Mutex

	Collection map[string]*templatesAndOptions
}

// templatesAndOptions contains templates and options associated to an exporter.
type templatesAndOptions struct {
	nd               *Decoder
	templateLock     sync.RWMutex
	samplingRateLock sync.RWMutex
	applicationLock  sync.RWMutex

	Key           string
	Templates     templates
	SamplingRates map[samplingRateKey]uint32
	Applications  map[applicationKey]application
}

// templates is a mapping to one of netflow.TemplateRecord,
// netflow.IPFIXOptionsTemplateRecord, netflow.NFv9OptionsTemplateRecord.
type templates map[templateKey]any

// templateKey is the key structure to access a template.
type templateKey struct {
	version     uint16
	obsDomainID uint32
	templateID  uint16
}

// applicationKey is the key structure to access an application. The
// application ID is unique per exporter (RFC 6759, section 4.3), so the
// observation domain is not part of the key: Cisco IOS XE exports the
// application tables and the flows with different source IDs.
type applicationKey struct {
	id      string
	version uint16
}

// application contains the name and attributes of an application, as exported
// in option records (Cisco NBAR2 application table and application
// attributes).
type application [applicationAttributeCount]string

// samplingRateKey is the key structure to access a sampling rate.
type samplingRateKey struct {
	version     uint16
	obsDomainID uint32
	samplerID   uint64
}

var (
	_ netflow.TemplateStore = &templatesAndOptions{}
)

// Get returns templates and options for the provided key. If it did not exist,
// it will create a new one.
func (c *templateAndOptionCollection) Get(key string) *templatesAndOptions {
	c.lock.Lock()
	defer c.lock.Unlock()
	t, ok := c.Collection[key]
	if ok {
		return t
	}
	t = &templatesAndOptions{
		nd:            c.nd,
		Key:           key,
		Templates:     make(map[templateKey]any),
		SamplingRates: make(map[samplingRateKey]uint32),
		Applications:  make(map[applicationKey]application),
	}
	c.Collection[key] = t
	return t
}

// GetTemplate returns the requested template.
func (t *templatesAndOptions) GetTemplate(_ netflow.FlowContext, version uint16, obsDomainID uint32, templateID uint16) (any, error) {
	t.templateLock.RLock()
	defer t.templateLock.RUnlock()
	template, ok := t.Templates[templateKey{version: version, obsDomainID: obsDomainID, templateID: templateID}]
	if !ok {
		return nil, netflow.ErrorTemplateNotFound
	}
	return template, nil
}

// AddTemplate stores a template.
func (t *templatesAndOptions) AddTemplate(_ netflow.FlowContext, version uint16, obsDomainID uint32, templateID uint16, template any) (netflow.TemplateStatus, error) {
	var typeStr string
	switch template.(type) {
	case netflow.IPFIXOptionsTemplateRecord:
		typeStr = "options_template"
	case netflow.NFv9OptionsTemplateRecord:
		typeStr = "options_template"
	case netflow.TemplateRecord:
		typeStr = "template"
	}

	t.nd.metrics.templates.WithLabelValues(
		t.Key,
		strconv.Itoa(int(version)),
		strconv.Itoa(int(obsDomainID)),
		strconv.Itoa(int(templateID)),
		typeStr,
	).Inc()

	t.templateLock.Lock()
	defer t.templateLock.Unlock()
	t.Templates[templateKey{version: version, obsDomainID: obsDomainID, templateID: templateID}] = template
	return netflow.TemplateAdded, nil
}

// GetSamplingRate returns the requested sampling rate.
func (t *templatesAndOptions) GetSamplingRate(version uint16, obsDomainID uint32, samplerID uint64) uint32 {
	t.samplingRateLock.RLock()
	defer t.samplingRateLock.RUnlock()
	rate := t.SamplingRates[samplingRateKey{
		version:     version,
		obsDomainID: obsDomainID,
		samplerID:   samplerID,
	}]
	return rate
}

// SetSamplingRate sets the sampling rate.
func (t *templatesAndOptions) SetSamplingRate(version uint16, obsDomainID uint32, samplerID uint64, samplingRate uint32) {
	t.samplingRateLock.Lock()
	defer t.samplingRateLock.Unlock()
	t.SamplingRates[samplingRateKey{
		version:     version,
		obsDomainID: obsDomainID,
		samplerID:   samplerID,
	}] = samplingRate
}

// GetApplication returns the application matching the provided ID.
func (t *templatesAndOptions) GetApplication(version uint16, id []byte) (application, bool) {
	t.applicationLock.RLock()
	defer t.applicationLock.RUnlock()
	app, ok := t.Applications[applicationKey{version: version, id: string(id)}]
	return app, ok
}

// UpdateApplication merges the non-empty attributes of an application.
func (t *templatesAndOptions) UpdateApplication(version uint16, id []byte, update *application) {
	t.applicationLock.Lock()
	defer t.applicationLock.Unlock()
	if t.Applications == nil {
		t.Applications = make(map[applicationKey]application)
	}
	key := applicationKey{version: version, id: string(id)}
	app := t.Applications[key]
	for i, value := range update {
		if value != "" {
			app[i] = value
		}
	}
	t.Applications[key] = app
}
