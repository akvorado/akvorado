// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
)

// MarshalText implements encoding.TextMarshaler for templateKey.
func (tk templateKey) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%d-%d-%d", tk.version, tk.obsDomainID, tk.templateID), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for templateKey.
func (tk *templateKey) UnmarshalText(text []byte) error {
	_, err := fmt.Sscanf(string(text), "%d-%d-%d", &tk.version, &tk.obsDomainID, &tk.templateID)
	if err != nil {
		return fmt.Errorf("invalid template key %q: %w", string(text), err)
	}
	return nil
}

// MarshalText implements encoding.TextMarshaler for samplingRateKey.
func (srk samplingRateKey) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%d-%d-%d", srk.version, srk.obsDomainID, srk.samplerID), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for samplingRateKey.
func (srk *samplingRateKey) UnmarshalText(text []byte) error {
	_, err := fmt.Sscanf(string(text), "%d-%d-%d", &srk.version, &srk.obsDomainID, &srk.samplerID)
	if err != nil {
		return fmt.Errorf("invalid sampling rate key %q: %w", string(text), err)
	}
	return nil
}

// MarshalJSON encodes a set of NetFlow templates.
func (t *templates) MarshalJSON() ([]byte, error) {
	type typedTemplate struct {
		Type     string
		Template any
	}
	data := make(map[templateKey]typedTemplate, len(*t))
	for k, v := range *t {
		switch v := v.(type) {
		case netflow.TemplateRecord:
			data[k] = typedTemplate{
				Type:     "data",
				Template: v,
			}
		case netflow.IPFIXOptionsTemplateRecord:
			data[k] = typedTemplate{
				Type:     "ipfix-option",
				Template: v,
			}
		case netflow.NFv9OptionsTemplateRecord:
			data[k] = typedTemplate{
				Type:     "nfv9-option",
				Template: v,
			}
		default:
			return nil, fmt.Errorf("unknown template type %q", reflect.TypeOf(v).String())
		}
	}
	return json.Marshal(&data)
}

// UnmarshalJSON decodes a set of NetFlow templates.
func (t *templates) UnmarshalJSON(data []byte) error {
	type typedTemplate struct {
		Type     string
		Template json.RawMessage
	}
	var templatesWithTypes map[templateKey]typedTemplate
	if err := json.Unmarshal(data, &templatesWithTypes); err != nil {
		return err
	}
	targetTemplates := make(templates, len(templatesWithTypes))
	for k, v := range templatesWithTypes {
		var targetTemplate any
		var err error
		switch v.Type {
		case "data":
			var tmpl netflow.TemplateRecord
			err = json.Unmarshal(v.Template, &tmpl)
			targetTemplate = tmpl
		case "ipfix-option":
			var tmpl netflow.IPFIXOptionsTemplateRecord
			err = json.Unmarshal(v.Template, &tmpl)
			targetTemplate = tmpl
		case "nfv9-option":
			var tmpl netflow.NFv9OptionsTemplateRecord
			err = json.Unmarshal(v.Template, &tmpl)
			targetTemplate = tmpl
		default:
			return fmt.Errorf("unknown type %q", v.Type)
		}
		if err != nil {
			return err
		}
		targetTemplates[k] = targetTemplate
	}
	*t = targetTemplates
	return nil
}

// MarshalJSON encodes the NetFlow decoder's collection.
func (nd *Decoder) MarshalJSON() ([]byte, error) {
	return json.Marshal(&nd.collection.Collection)
}

// UnmarshalJSON decodes the NetFlow decoder's collection.
func (nd *Decoder) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &nd.collection.Collection); err != nil {
		return err
	}
	for _, tao := range nd.collection.Collection {
		tao.nd = nd
	}
	return nil
}
