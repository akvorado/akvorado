// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"sort"
	"testing"

	"akvorado/common/helpers"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestSubscribeResponseToEvents(t *testing.T) {
	sortEvents := func(events []event) {
		sort.Slice(events, func(i, j int) bool {
			if events[i].Keys != events[j].Keys {
				return events[i].Keys < events[j].Keys
			}
			return events[i].Path < events[j].Path
		})
	}

	cases := []struct {
		Name     string
		Response *gnmi.SubscribeResponse
		Expected []event
	}{
		{
			Name:     "sync response",
			Response: &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{}},
			Expected: []event{},
		},
		{
			Name: "string value",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "name"},
							{Name: "host-name"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "router1"}},
					}},
				}},
			},
			Expected: []event{
				{"/system/name/host-name", "", "router1"},
			},
		},
		{
			Name: "int value",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "interface", Key: map[string]string{"name": "eth1"}},
							{Name: "ifindex"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 42}},
					}},
				}},
			},
			Expected: []event{
				{"/interface/ifindex", "name=eth1", "42"},
			},
		},
		{
			Name: "uint value",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "interface", Key: map[string]string{"name": "eth1"}},
							{Name: "counters"},
							{Name: "in-octets"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_UintVal{UintVal: 123456}},
					}},
				}},
			},
			Expected: []event{
				{"/interface/counters/in-octets", "name=eth1", "123456"},
			},
		},
		{
			Name: "ascii value",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "system"},
							{Name: "version"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "v1.0"}},
					}},
				}},
			},
			Expected: []event{
				{"/system/version", "", "v1.0"},
			},
		},
		{
			Name: "unsupported value type is skipped",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "bool-leaf"}}},
							Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_BoolVal{BoolVal: true}},
						},
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "string-leaf"}}},
							Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "kept"}},
						},
					},
				}},
			},
			Expected: []event{
				{"/string-leaf", "", "kept"},
			},
		},
		{
			Name: "json_ietf value with nested map",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte(`{"config":{"hostname":"router1"}}`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/system/config/hostname", "", "router1"},
			},
		},
		{
			Name: "json value with array",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte(`{"interface":[{"name":"eth1","state":{"ifindex":100,"description":"first"}},{"name":"eth2","state":{"ifindex":101,"description":"second"}}]}`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/interfaces/interface/name", "name=eth1", "eth1"},
				{"/interfaces/interface/state/description", "name=eth1", "first"},
				{"/interfaces/interface/state/ifindex", "name=eth1", "100"},
				{"/interfaces/interface/name", "name=eth2", "eth2"},
				{"/interfaces/interface/state/description", "name=eth2", "second"},
				{"/interfaces/interface/state/ifindex", "name=eth2", "101"},
			},
		},
		{
			Name: "json array with lag-speed",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte(`{"interface":[{"name":"po1","aggregation":{"state":{"lag-speed":40000}}},{"name":"po126","aggregation":{"state":{"lag-speed":25000}}}]}`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/interfaces/interface/aggregation/state/lag-speed", "name=po1", "40000"},
				{"/interfaces/interface/name", "name=po1", "po1"},
				{"/interfaces/interface/aggregation/state/lag-speed", "name=po126", "25000"},
				{"/interfaces/interface/name", "name=po126", "po126"},
			},
		},
		{
			Name: "json array with port-speed string",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte(`{"interface":[{"name":"eth1/1","ethernet":{"state":{"port-speed":"SPEED_25GB"}}}]}`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/interfaces/interface/ethernet/state/port-speed", "name=eth1/1", "SPEED_25GB"},
				{"/interfaces/interface/name", "name=eth1/1", "eth1/1"},
			},
		},
		{
			Name: "json array elements with only keys",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte(`{"interface":[{"name":"eth1/1"}]}`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/interfaces/interface/name", "name=eth1/1", "eth1/1"},
			},
		},
		{
			Name: "json nested arrays with combined keys",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte(`{"interface":[{"name":"eth1/4","subinterfaces":{"subinterface":[{"index":1,"state":{"name":"eth1/4.1","description":"4th interface","ifindex":105}}]}}]}`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/interfaces/interface/name", "name=eth1/4", "eth1/4"},
				{"/interfaces/interface/subinterfaces/subinterface/index", "name=eth1/4,index=1", "1"},
				{"/interfaces/interface/subinterfaces/subinterface/state/description", "name=eth1/4,index=1", "4th interface"},
				{"/interfaces/interface/subinterfaces/subinterface/state/ifindex", "name=eth1/4,index=1", "105"},
				{"/interfaces/interface/subinterfaces/subinterface/state/name", "name=eth1/4,index=1", "eth1/4.1"},
			},
		},
		{
			Name: "path keys combined with json array keys",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "interface", Key: map[string]string{"name": "eth1/4"}},
							{Name: "subinterface"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte(`[{"index":1,"state":{"name":"eth1/4.1"}}]`),
						}},
					}},
				}},
			},
			Expected: []event{
				{"/interface/subinterface/index", "name=eth1/4,index=1", "1"},
				{"/interface/subinterface/state/name", "name=eth1/4,index=1", "eth1/4.1"},
			},
		},
		{
			Name: "json array with non-map elements are skipped",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "something"}}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte(`["just a string",42]`),
						}},
					}},
				}},
			},
			Expected: []event{},
		},
		{
			Name: "invalid json is skipped",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "bad"}}},
							Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{
								JsonVal: []byte(`{invalid`),
							}},
						},
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "good"}}},
							Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "ok"}},
						},
					},
				}},
			},
			Expected: []event{
				{"/good", "", "ok"},
			},
		},
		{
			Name: "prefix is prepended",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Prefix: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}}},
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "interface", Key: map[string]string{"name": "eth1"}},
							{Name: "state"},
							{Name: "description"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "uplink"}},
					}},
				}},
			},
			Expected: []event{
				{"/interfaces/interface/state/description", "name=eth1", "uplink"},
			},
		},
		{
			Name: "namespace prefix is stripped",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "srl_nokia-interfaces:interface", Key: map[string]string{"name": "ethernet-1/1"}},
							{Name: "description"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "first"}},
					}},
				}},
			},
			Expected: []event{
				{"/interface/description", "name=ethernet-1/1", "first"},
			},
		},
		{
			Name: "multiple path keys",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{
							{Name: "interface", Key: map[string]string{"name": "ethernet-1/4"}},
							{Name: "subinterface", Key: map[string]string{"index": "1"}},
							{Name: "name"},
						}},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "ethernet-1/4.1"}},
					}},
				}},
			},
			Expected: []event{
				{"/interface/subinterface/name", "name=ethernet-1/4,index=1", "ethernet-1/4.1"},
			},
		},
		{
			Name: "multiple updates",
			Response: &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{
								{Name: "interface", Key: map[string]string{"name": "eth1"}},
								{Name: "description"},
							}},
							Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "first"}},
						},
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{
								{Name: "interface", Key: map[string]string{"name": "eth2"}},
								{Name: "description"},
							}},
							Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "second"}},
						},
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{
								{Name: "system"},
								{Name: "name"},
							}},
							Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "router1"}},
						},
					},
				}},
			},
			Expected: []event{
				{"/interface/description", "name=eth1", "first"},
				{"/interface/description", "name=eth2", "second"},
				{"/system/name", "", "router1"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := subscribeResponseToEvents(tc.Response)
			sortEvents(got)
			sortEvents(tc.Expected)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("subscribeResponseToEvents() (-got, +want):\n%s", diff)
			}
		})
	}
}
