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

	Key           string
	Templates     templates
	SamplingRates map[samplingRateKey]uint32
	// samplerRates contains the sampling rates received from observation
	// domains without data template, indexed by sampler only. Cisco IOS XE
	// exports the sampler options and the flows with different source IDs. It
	// is rebuilt from SamplingRates and Templates when restoring the state.
	samplerRates map[samplerKey]samplerRate
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

// samplerKey is the key structure to access a sampling rate from any
// observation domain.
type samplerKey struct {
	version   uint16
	samplerID uint64
}

// samplerRate is a sampling rate with the observation domain it comes from. A
// zero rate is a tombstone: the sampler is also used by a domain with data.
type samplerRate struct {
	rate        uint32
	obsDomainID uint32
}

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
		samplerRates:  make(map[samplerKey]samplerRate),
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
	if _, ok := template.(netflow.TemplateRecord); ok {
		t.samplingRateLock.Lock()
		defer t.samplingRateLock.Unlock()
		for key, sr := range t.samplerRates {
			if key.version == version && sr.obsDomainID == obsDomainID {
				t.samplerRates[key] = samplerRate{}
			}
		}
	}
	return netflow.TemplateAdded, nil
}

// hasDataTemplate tells if an observation domain has a data template. The
// template lock should be held.
func (t *templatesAndOptions) hasDataTemplate(version uint16, obsDomainID uint32) bool {
	for key, template := range t.Templates {
		if key.version == version && key.obsDomainID == obsDomainID {
			if _, ok := template.(netflow.TemplateRecord); ok {
				return true
			}
		}
	}
	return false
}

// setSamplerRate records a sampling rate for a sampler, regardless of the
// observation domain. When the domain also has a data template, the sampler
// is marked as ambiguous for good. Otherwise, the most recent rate wins. The
// sampling rate lock should be held.
func (t *templatesAndOptions) setSamplerRate(version uint16, obsDomainID uint32, samplerID uint64, samplingRate uint32, hasData bool) {
	if t.samplerRates == nil {
		t.samplerRates = make(map[samplerKey]samplerRate)
	}
	key := samplerKey{version: version, samplerID: samplerID}
	if current, ok := t.samplerRates[key]; ok && current.rate == 0 {
		return
	}
	if hasData {
		t.samplerRates[key] = samplerRate{}
		return
	}
	t.samplerRates[key] = samplerRate{rate: samplingRate, obsDomainID: obsDomainID}
}

// rebuildSamplerRates rebuilds the sampling rates indexed by sampler after
// restoring the state.
func (t *templatesAndOptions) rebuildSamplerRates() {
	t.templateLock.RLock()
	defer t.templateLock.RUnlock()
	t.samplingRateLock.Lock()
	defer t.samplingRateLock.Unlock()
	t.samplerRates = make(map[samplerKey]samplerRate)
	for key, rate := range t.SamplingRates {
		t.setSamplerRate(key.version, key.obsDomainID, key.samplerID, rate,
			t.hasDataTemplate(key.version, key.obsDomainID))
	}
}

// GetSamplingRate returns the requested sampling rate. When there is none for
// the provided observation domain, the sampling rate of the same sampler from
// an observation domain without data template is used.
func (t *templatesAndOptions) GetSamplingRate(version uint16, obsDomainID uint32, samplerID uint64) uint32 {
	t.samplingRateLock.RLock()
	defer t.samplingRateLock.RUnlock()
	rate, ok := t.SamplingRates[samplingRateKey{
		version:     version,
		obsDomainID: obsDomainID,
		samplerID:   samplerID,
	}]
	if ok {
		return rate
	}
	return t.samplerRates[samplerKey{version: version, samplerID: samplerID}].rate
}

// SetSamplingRate sets the sampling rate.
func (t *templatesAndOptions) SetSamplingRate(version uint16, obsDomainID uint32, samplerID uint64, samplingRate uint32) {
	t.templateLock.RLock()
	defer t.templateLock.RUnlock()
	t.samplingRateLock.Lock()
	defer t.samplingRateLock.Unlock()
	t.SamplingRates[samplingRateKey{
		version:     version,
		obsDomainID: obsDomainID,
		samplerID:   samplerID,
	}] = samplingRate
	t.setSamplerRate(version, obsDomainID, samplerID, samplingRate,
		t.hasDataTemplate(version, obsDomainID))
}
