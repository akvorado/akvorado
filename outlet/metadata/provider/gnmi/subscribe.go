// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
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
	//
	// Example of update for JSON encoding with arrays:
	// update:{path:{elem:{name:"interfaces"}}
	//         val:{json_val:"{\"interface\":[
	//                {\"name\":\"eth1\",\"state\":{\"ifindex\":100}},
	//                {\"name\":\"eth2\",\"state\":{\"ifindex\":101}}]}"}}
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
			// For JSON, we need to walk the structure to create events.
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
	// Slices: each element is expected to be a map. Simple (non-container)
	// values at the top level of each element are collected as keys (this
	// matches the OpenConfig convention where list keys are leaf nodes at
	// the top level of the list entry).
	case []any:
		for _, item := range value {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			currentEvent := ev
			keys := make([]string, 0)
			for k, v := range itemMap {
				switch v := v.(type) {
				case string:
					keys = append(keys, fmt.Sprintf("%s=%s", k, v))
				case float64:
					keys = append(keys, fmt.Sprintf("%s=%d", k, int64(v)))
				}
			}
			sort.Strings(keys)
			keyStr := strings.Join(keys, ",")
			if currentEvent.Keys != "" && keyStr != "" {
				currentEvent.Keys = fmt.Sprintf("%s,%s", currentEvent.Keys, keyStr)
			} else if keyStr != "" {
				currentEvent.Keys = keyStr
			}
			events = jsonAppendToEvents(events, currentEvent, itemMap)
		}
		return events
	// Maps
	case map[string]any:
		for k, v := range value {
			currentEvent := ev
			currentEvent.Path = path.Join(currentEvent.Path, k)
			events = jsonAppendToEvents(events, currentEvent, v)
		}
		return events
	// Base types
	case float64:
		ev.Value = strconv.FormatInt(int64(value), 10)
	case string:
		ev.Value = value
	default:
		return events
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
		_, after, ok := strings.Cut(name, ":")
		if !ok {
			pathString.WriteString(name)
		} else {
			pathString.WriteString(after)
		}

		// Format keys
		if len(pe.GetKey()) > 0 {
			keys := make([]string, 0, len(pe.GetKey()))
			for k, v := range pe.GetKey() {
				keys = append(keys, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(keys)
			keysString.WriteString(",")
			keysString.WriteString(strings.Join(keys, ","))
		}
	}
	return event{
		Path: pathString.String(),
		Keys: strings.TrimLeft(keysString.String(), ","),
	}
}
