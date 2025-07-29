// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"encoding/json"
	"path"
	"strconv"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
)

// event describes an event received in a subscription. No deletion is handled
// as we don't really care about them.
type event struct {
	Path  string // path without keys
	Keys  string // comma-separated keys
	Value string // value
}

func subscribeResponseToEvents(response *gnmi.SubscribeResponse) []event {
	events := []event{}
	n := response.GetUpdate()
	// Example of update for JSON encoding:
	// update:{path:{elem:{name:"interface"
	//                     key:{key:"name" value:"ethernet-1/4"}}
	//               elem:{name:"subinterface"
	//                     key:{key:"index" value:"1"}}
	//               elem:{name:"name"}}
	//         val:{string_val:"ethernet-1/4.1"}}
	//
	// Example of update for JSON IETF encoding:
	// update:{path:{elem:{name:"srl_nokia-interfaces:interface"
	//                     key:{key:"name"  value:"ethernet-1/4"}}
	//               elem:{name:"subinterface"
	//                     key:{key:"index"  value:"1"}}}
	//         val:{json_ietf_val:"{\"name\": \"ethernet-1/4.1\"}"}}
	if n != nil {
		prefixEvent := gnmiPathToEvent(n.GetPrefix(), event{})
		for _, u := range n.GetUpdate() {
			ev := gnmiPathToEvent(u.GetPath(), prefixEvent)
			val := u.GetVal()
			var jsondata []byte
			switch val.Value.(type) {
			case *gnmi.TypedValue_AsciiVal:
				ev.Value = val.GetAsciiVal()
			case *gnmi.TypedValue_IntVal:
				ev.Value = strconv.FormatInt(val.GetIntVal(), 10)
			case *gnmi.TypedValue_UintVal:
				ev.Value = strconv.FormatUint(val.GetUintVal(), 10)
			case *gnmi.TypedValue_StringVal:
				ev.Value = val.GetStringVal()
			case *gnmi.TypedValue_JsonIetfVal:
				jsondata = val.GetJsonIetfVal()
			case *gnmi.TypedValue_JsonVal:
				jsondata = val.GetJsonVal()
			default:
				continue
			}
			// For non-JSON, we are done
			if len(jsondata) == 0 {
				events = append(events, ev)
				continue
			}
			// For JSON, we need to walk the structure to create events. We
			// assume that we only get simple cases: no keys, no slice.
			var value any
			if err := json.Unmarshal(jsondata, &value); err != nil {
				continue
			}
			events = jsonAppendToEvents(events, ev, value)
		}
		// No need to proceed delete events, we don't rely on them
		// for _, u := range n.GetDelete() {
		// 	ev := gnmiPathToEvent(u, prefixEvent)
		// 	events = jsonAppendToEvents(events, ev, "")
		// }
	}
	return events
}

func subscribeResponsesToEvents(responses []*gnmi.SubscribeResponse) []event {
	events := []event{}
	for _, response := range responses {
		events = append(events, subscribeResponseToEvents(response)...)
	}
	return events
}

// jsonAppendToEvents appends the events derived from the provided event plus
// the JSON-decoded value.
func jsonAppendToEvents(events []event, ev event, value any) []event {
	switch value := value.(type) {
	default:
		return events
	// Slices: not handled
	// Maps
	case map[string]any:
		for k, v := range value {
			currentEvent := ev
			currentEvent.Path = path.Join(currentEvent.Path, k)
			events = jsonAppendToEvents(events, currentEvent, v)
		}
		return events
	// Base types
	case int64:
		ev.Value = strconv.FormatInt(value, 10)
	case float64:
		ev.Value = strconv.FormatInt(int64(value), 10)
	case string:
		ev.Value = value
	}
	return append(events, ev)
}

// gnmiPathToXPath turns a gNMI path to an event (without a value).
func gnmiPathToEvent(p *gnmi.Path, prefix event) event {
	if p == nil {
		return prefix
	}
	pathString := &strings.Builder{}
	pathString.WriteString(prefix.Path)
	keysString := &strings.Builder{}
	keysString.WriteString(prefix.Keys)
	elems := p.GetElem()

	for _, pe := range elems {
		name := pe.GetName()
		pathString.WriteString("/")

		// Remove namespace if present and format path
		colon := strings.Index(name, ":")
		if colon == -1 {
			pathString.WriteString(name)
		} else {
			pathString.WriteString(name[colon+1:])
		}

		// Format keys
		for k, v := range pe.GetKey() {
			keysString.WriteString(",")
			keysString.WriteString(k)
			keysString.WriteString("=")
			keysString.WriteString(v)
		}
	}
	return event{
		Path: pathString.String(),
		Keys: strings.TrimLeft(keysString.String(), ","),
	}
}
