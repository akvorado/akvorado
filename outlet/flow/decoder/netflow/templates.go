// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"strconv"
	"sync"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
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

	Key           string
	Templates     templates
	SamplingRates map[samplingRateKey]uint32
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

// samplingRateKey is the key structure to access a sampling rate.
type samplingRateKey struct {
	version     uint16
	obsDomainID uint32
	samplerID   uint64
}

var (
	_ netflow.NetFlowTemplateSystem = &templatesAndOptions{}
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
	}
	c.Collection[key] = t
	return t
}

// RemoveTemplate removes an existing template. This is a noop as it is not
// really needed.
func (t *templatesAndOptions) RemoveTemplate(uint16, uint32, uint16) (any, error) {
	return nil, nil
}

// GetTemplate returns the requested template.
func (t *templatesAndOptions) GetTemplate(version uint16, obsDomainID uint32, templateID uint16) (any, error) {
	t.templateLock.RLock()
	defer t.templateLock.RUnlock()
	template, ok := t.Templates[templateKey{version: version, obsDomainID: obsDomainID, templateID: templateID}]
	if !ok {
		return nil, netflow.ErrorTemplateNotFound
	}
	return template, nil
}

// AddTemplate stores a template.
func (t *templatesAndOptions) AddTemplate(version uint16, obsDomainID uint32, templateID uint16, template any) error {
	var typeStr string
	switch templateIDConv := template.(type) {
	case netflow.IPFIXOptionsTemplateRecord:
		templateID = templateIDConv.TemplateId
		typeStr = "options_template"
	case netflow.NFv9OptionsTemplateRecord:
		templateID = templateIDConv.TemplateId
		typeStr = "options_template"
	case netflow.TemplateRecord:
		templateID = templateIDConv.TemplateId
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
	return nil
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
