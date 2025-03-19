// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api"
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/transport"
)

func waitSRLManagementServerReady(t *testing.T, d *network.Driver) {
	const (
		mgmtServerRdyCmd  = "info from state system app-management application mgmt_server state | grep running"
		readyForConfigCmd = "file cat /etc/opt/srlinux/devices/app_ephemeral.mgmt_server.ready_for_config"
	)
	retryTimer := time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("SR Linux management server not ready in time")
			return
		default:
			resp, err := d.SendCommand(mgmtServerRdyCmd)
			if err != nil || resp.Failed != nil {
				time.Sleep(retryTimer)
				continue
			}

			if !strings.Contains(resp.Result, "running") {
				t.Log("SR Linux did not start management server yet")
				time.Sleep(retryTimer)
				continue
			}

			resp, err = d.SendCommand(readyForConfigCmd)
			if err != nil || resp.Failed != nil {
				time.Sleep(retryTimer)
				continue
			}

			if !strings.Contains(resp.Result, "loaded initial configuration") {
				t.Log("SR Linux did not load configuration yet")
				time.Sleep(retryTimer)
				continue
			}

			t.Log("SR Linux ready")
			return
		}
	}
}

func TestSRLinux(t *testing.T) {
	const (
		srLinuxUsername = "admin"
		srLinuxPassword = "NokiaSrl1!"
	)
	// Connect to SR Linux container
	srLinux := helpers.CheckExternalService(t, "SR Linux",
		[]string{"srlinux:22", "127.0.0.1:57401"})
	srLinuxHostname, srLinuxPortStr, _ := net.SplitHostPort(srLinux)
	srLinuxPort, _ := strconv.Atoi(srLinuxPortStr)
	t.Logf("SR Linux is listening at %s:%d", srLinuxHostname, srLinuxPort)
	logger, err := logging.NewInstance(
		logging.WithLogger(t.Log),
		logging.WithLevel(logging.Critical), // can be changed if needed
	)
	if err != nil {
		t.Fatalf("NewInstance() error:\n%+v", err)
	}
	plat, err := platform.NewPlatform(
		"nokia_srl", srLinuxHostname,
		options.WithPort(srLinuxPort),
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(srLinuxUsername),
		options.WithAuthPassword(srLinuxPassword),
		options.WithTransportType(transport.StandardTransport),
		options.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("NewPlatform() error:\n%+v", err)
	}
	driver, err := plat.GetNetworkDriver()
	if err != nil {
		t.Fatalf("GetNetworkDriver() error:\n%+v", err)
	}
	for remaining := 3; remaining >= 0; remaining++ {
		err = driver.Open()
		if err != nil {
			if remaining == 0 {
				t.Fatalf("Open() error:\n%+v", err)
			}
			time.Sleep(300 * time.Millisecond)
			continue
		}
		break
	}
	defer driver.Close()
	waitSRLManagementServerReady(t, driver)
	resp, err := driver.SendCommand("show version")
	if err != nil {
		t.Fatalf("SendCommand(show version) error:\n%+v", err)
	}
	t.Logf(
		"sent command '%s', output received (SendCommand):\n %s",
		resp.Input,
		resp.Result,
	)
	if resp.Failed != nil {
		t.Fatalf("SendCommand(show version) error:\n%+v", resp.Failed)
	}

	// Initial configuration
	resp, err = driver.SendConfig(`
load factory

/ system gnmi-server
set admin-state enable network-instance mgmt admin-state enable

commit now
`)
	if err != nil {
		t.Fatalf("SendConfig() error:\n%+v", err)
	}
	t.Logf(
		"sent command '%s', output received (SendCommand):\n %s",
		resp.Input,
		resp.Result,
	)
	if resp.Failed != nil {
		t.Fatalf("SendConfig() error:\n%+v", resp.Failed)
	}

	// gNMI setup
	srLinuxGNMI := helpers.CheckExternalService(t, "SR Linux gNMI",
		[]string{"srlinux:57400", "127.0.0.1:57400"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tg, err := api.NewTarget(
		api.Address(srLinuxGNMI),
		api.Username(srLinuxUsername),
		api.Password(srLinuxPassword),
		api.Timeout(time.Second),
		api.Insecure(true),
	)
	tg.Config.RetryTimer = 10 * time.Millisecond
	if err != nil {
		t.Fatalf("api.NewTarget() error:\n%+v", err)
	}
	err = tg.CreateGNMIClient(ctx)
	if err != nil {
		t.Fatalf("CreateGNMIClient() error:\n%+v", err)
	}
	defer tg.Close()

	resetConfig := func(t *testing.T) {
		t.Helper()
		resp, err := driver.SendConfig(`
set / system name host-name "srlinux"

/ interface ethernet-1/1
set admin-state enable
set description "1st interface"

/ interface ethernet-1/2
set admin-state enable
set description "2nd interface"
set ethernet aggregate-id lag1
/ interface ethernet-1/3
set admin-state enable
set description "3rd interface"
set ethernet aggregate-id lag1
/ interface lag1
set admin-state enable
set description "lag interface"
set lag lag-type static

/ interface ethernet-1/4 subinterface 1
set admin-state enable
set description "4th interface"

commit now
`)
		if err != nil {
			t.Fatalf("SendConfig() error:\n%+v", err)
		}
		if resp.Failed != nil {
			t.Fatalf("SendConfig() error:\n%+v", resp.Failed)
		}
	}
	for _, encoding := range []string{"json", "json_ietf"} {
		// Test a "once" subscription
		t.Run(fmt.Sprintf("subscribe once %s", encoding), func(t *testing.T) {
			resetConfig(t)
			subscribeReq, err := api.NewSubscribeRequest(
				api.Subscription(api.Path("/system/name/host-name")),
				api.Subscription(api.Path("/interface/name")),
				api.Subscription(api.Path("/interface/description")),
				api.Subscription(api.Path("/interface/subinterface/name")),
				api.Subscription(api.Path("/interface/subinterface/description")),
				api.Subscription(api.Path("/interface/ethernet/port-speed")),
				api.Subscription(api.Path("/interface/lag/lag-speed")),
				api.SubscriptionListModeONCE(),
				api.Encoding(encoding),
			)
			if err != nil {
				t.Fatalf("NewSubscribeRequest() error:\n%+v", err)
			}
			subscribeResp, err := tg.SubscribeOnce(ctx, subscribeReq)
			if err != nil {
				t.Fatalf("SubscribeOnce() error:\n%+v", err)
			}
			got := subscribeResponsesToEvents(subscribeResp)
			sort.Slice(got, func(i, j int) bool {
				if got[i].Path != got[j].Path {
					return got[i].Path < got[j].Path
				}
				return got[i].Keys < got[j].Keys
			})
			expected := []event{
				{"/interface/description", "name=ethernet-1/1", "1st interface"},
				{"/interface/description", "name=ethernet-1/2", "2nd interface"},
				{"/interface/description", "name=ethernet-1/3", "3rd interface"},
				{"/interface/description", "name=lag1", "lag interface"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/1", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/10", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/11", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/12", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/13", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/14", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/15", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/16", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/17", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/18", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/19", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/2", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/20", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/21", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/22", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/23", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/24", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/25", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/26", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/27", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/28", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/29", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/3", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/30", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/31", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/32", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/33", "10G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/34", "10G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/4", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/5", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/6", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/7", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/8", "100G"},
				{"/interface/ethernet/port-speed", "name=ethernet-1/9", "100G"},
				{"/interface/ethernet/port-speed", "name=mgmt0", "1G"},
				{"/interface/lag/lag-speed", "name=lag1", "0"},
				// Despite being subscribed, we don't get these...
				// {"/interface/name", "name=ethernet-1/1", "ethernet-1/1"},
				// {"/interface/name", "name=ethernet-1/2", "ethernet-1/2"},
				// {"/interface/name", "name=ethernet-1/3", "ethernet-1/3"},
				// {"/interface/name", "name=lag1", "lag1"},
				{"/interface/subinterface/description", "name=ethernet-1/4,index=1", "4th interface"},
				{"/interface/subinterface/name", "name=ethernet-1/4,index=1", "ethernet-1/4.1"},
				{"/interface/subinterface/name", "name=mgmt0,index=0", "mgmt0.0"},
				{"/system/name/host-name", "", "srlinux"},
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Get() (-got, +want):\n%s", diff)
			}
		})

		// Test a regular subscription with onchange
		if t.Failed() {
			return
		}
		t.Run(fmt.Sprintf("subscribe changes %s", encoding), func(t *testing.T) {
			subscribeReq, err := api.NewSubscribeRequest(
				api.Subscription(api.Path("/system/name/host-name"), api.SubscriptionModeON_CHANGE()),
				api.Subscription(api.Path("/interface/description"), api.SubscriptionModeON_CHANGE()),
				api.Subscription(api.Path("/interface/subinterface/name"), api.SubscriptionModeON_CHANGE()),
				api.SubscriptionListModeSTREAM(),
				api.Encoding(encoding),
			)
			if err != nil {
				t.Fatalf("NewSubscribeRequest() error:\n%+v", err)
			}
			subscriptionName := fmt.Sprintf("changes-%s", encoding)
			go tg.Subscribe(ctx, subscribeReq, subscriptionName)
			defer tg.StopSubscription(subscriptionName)
			subRspChan, subErrChan := tg.ReadSubscriptions()

			// Wait for first set of answers
			timer := time.NewTimer(time.Second)
			responses := []*gnmi.SubscribeResponse{}
		outer1:
			for {
				select {
				case <-ctx.Done():
				case <-timer.C:
					t.Fatalf("Subscribe(): no sync response")
				case resp := <-subRspChan:
					if resp.SubscriptionName == subscriptionName {
						switch resp.Response.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
							responses = append(responses, resp.Response)
						case *gnmi.SubscribeResponse_SyncResponse:
							break outer1
						default:
							t.Fatalf("Subscribe(): unknown type: %v", reflect.TypeOf(resp.Response.Response))
						}
					}
				case err := <-subErrChan:
					if err.SubscriptionName == subscriptionName {
						t.Fatalf("Subscribe() error:\n%+v", err)
					}
				}
			}
			got := subscribeResponsesToEvents(responses)
			sort.Slice(got, func(i, j int) bool {
				if got[i].Path != got[j].Path {
					return got[i].Path < got[j].Path
				}
				return got[i].Keys < got[j].Keys
			})
			expected := []event{
				{"/interface/description", "name=ethernet-1/1", "1st interface"},
				{"/interface/description", "name=ethernet-1/2", "2nd interface"},
				{"/interface/description", "name=ethernet-1/3", "3rd interface"},
				{"/interface/description", "name=lag1", "lag interface"},
				{"/interface/subinterface/name", "name=ethernet-1/4,index=1", "ethernet-1/4.1"},
				{"/interface/subinterface/name", "name=mgmt0,index=0", "mgmt0.0"},
				{"/system/name/host-name", "", "srlinux"},
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Subscribe() initial sync (-got, +want):\n%s", diff)
			}

			// Change the configuration and check for a change.
			resp, err = driver.SendConfig(`
set / system name host-name "srlinux-new"

/ interface ethernet-1/1
set description "1st interface new"

/ interface ethernet-1/4
delete subinterface 1

commit now
`)
			if err != nil {
				t.Fatalf("SendConfig() error:\n%+v", err)
			}
			if resp.Failed != nil {
				t.Fatalf("SendConfig() error:\n%+v", resp.Failed)
			}

			responses = []*gnmi.SubscribeResponse{}
			expected = []event{
				{"/interface/description", "name=ethernet-1/1", "1st interface new"},
				// They should happen but they do not...
				// {"/interface/subinterface/description", "name=ethernet-1/4,index=1", ""},
				// {"/interface/subinterface/name", "name=ethernet-1/4,index=1", ""},
				{"/system/name/host-name", "", "srlinux-new"},
			}
			timer = time.NewTimer(time.Second)
		outer2:
			for {
				select {
				case <-ctx.Done():
				case <-timer.C:
					break outer2
				case resp := <-subRspChan:
					if resp.SubscriptionName == subscriptionName {
						switch resp.Response.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
							responses = append(responses, resp.Response)
						default:
							t.Fatalf("Subscribe(): unknown type: %v", reflect.TypeOf(resp.Response.Response))
						}
					}
				case err := <-subErrChan:
					if err.SubscriptionName == subscriptionName {
						t.Fatalf("Subscribe() error:\n%+v", err)
					}
				}
				got := subscribeResponsesToEvents(responses)
				if len(got) >= len(expected) {
					break
				}
			}
			got = subscribeResponsesToEvents(responses)
			sort.Slice(got, func(i, j int) bool {
				if got[i].Path != got[j].Path {
					return got[i].Path < got[j].Path
				}
				return got[i].Keys < got[j].Keys
			})
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Subscribe() after change (-got, +want):\n%s", diff)
			}

			// Test what happens when we disconnect
			if encoding == "json" {
				resp, err = driver.SendConfig(`
/ system gnmi-server admin-state enable network-instance mgmt
set admin-state disable
commit stay
set admin-state enable
commit now
`)
				if err != nil {
					t.Fatalf("SendConfig() error:\n%+v", err)
				}
				if resp.Failed != nil {
					t.Fatalf("SendConfig() error:\n%+v", resp.Failed)
				}

				responses = []*gnmi.SubscribeResponse{}
				errors := []string{}
				timer = time.NewTimer(time.Second)
			outer3:
				for {
					select {
					case <-ctx.Done():
					case <-timer.C:
						t.Fatalf("Subscribe(): no sync response")
					case resp := <-subRspChan:
						if resp.SubscriptionName == subscriptionName {
							switch resp.Response.Response.(type) {
							case *gnmi.SubscribeResponse_Update:
								responses = append(responses, resp.Response)
							case *gnmi.SubscribeResponse_SyncResponse:
								break outer3
							default:
								t.Fatalf("Subscribe(): unknown type: %v", reflect.TypeOf(resp.Response.Response))
							}
						}
					case err := <-subErrChan:
						if err.SubscriptionName == subscriptionName {
							errors = append(errors, err.Err.Error())
						}
					}
				}
				got = subscribeResponsesToEvents(responses)
				sort.Slice(got, func(i, j int) bool {
					if got[i].Path != got[j].Path {
						return got[i].Path < got[j].Path
					}
					return got[i].Keys < got[j].Keys
				})
				expected := []event{
					{"/interface/description", "name=ethernet-1/1", "1st interface new"},
					{"/interface/description", "name=ethernet-1/2", "2nd interface"},
					{"/interface/description", "name=ethernet-1/3", "3rd interface"},
					{"/interface/description", "name=lag1", "lag interface"},
					{"/interface/subinterface/name", "name=mgmt0,index=0", "mgmt0.0"},
					{"/system/name/host-name", "", "srlinux-new"},
				}
				if diff := helpers.Diff(got, expected); diff != "" {
					t.Fatalf("Subscribe() after disconnect (-got, +want):\n%s", diff)
				}
				expectedErrors := []string{
					"rpc error: code = Unavailable desc = Cancelling all calls",
					"retrying in 10ms",
				}
				if diff := helpers.Diff(errors, expectedErrors); diff != "" {
					t.Fatalf("Subscribe() errors after disconnect (-got, +want):\n%s", diff)
				}
			}
		})

		// Test a regular subscription with sampling
		if t.Failed() {
			return
		}
		t.Run(fmt.Sprintf("subscribe sampling %s", encoding), func(t *testing.T) {
			subscribeReq, err := api.NewSubscribeRequest(
				api.Subscription(
					api.Path("/system/name/host-name"),
					api.SubscriptionModeSAMPLE(),
					api.SampleInterval(time.Second)),
				api.Subscription(
					api.Path("/interface/description"),
					api.SubscriptionModeSAMPLE(),
					api.SampleInterval(time.Second)),
				api.SubscriptionListModeSTREAM(),
				api.Encoding(encoding),
			)
			if err != nil {
				t.Fatalf("NewSubscribeRequest() error:\n%+v", err)
			}
			subscriptionName := fmt.Sprintf("sampling-%s", encoding)
			go tg.Subscribe(ctx, subscribeReq, subscriptionName)
			defer tg.StopSubscription(subscriptionName)
			subRspChan, subErrChan := tg.ReadSubscriptions()

			// Wait for first set of answers
			responses := []*gnmi.SubscribeResponse{}
			timer := time.NewTimer(time.Second)
		outer4:
			for {
				select {
				case <-ctx.Done():
				case <-timer.C:
					t.Fatal("Subscribe(): no sync response")
				case resp := <-subRspChan:
					if resp.SubscriptionName == subscriptionName {
						switch resp.Response.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
							responses = append(responses, resp.Response)
						case *gnmi.SubscribeResponse_SyncResponse:
							break outer4
						default:
							t.Fatalf("Subscribe(): unknown type: %v", reflect.TypeOf(resp.Response.Response))
						}
					}
				case err := <-subErrChan:
					if err.SubscriptionName == subscriptionName {
						t.Fatalf("Subscribe() error:\n%+v", err)
					}
				}
			}
			got := subscribeResponsesToEvents(responses)
			sort.Slice(got, func(i, j int) bool {
				if got[i].Path != got[j].Path {
					return got[i].Path < got[j].Path
				}
				return got[i].Keys < got[j].Keys
			})
			expected := []event{
				{"/interface/description", "name=ethernet-1/1", "1st interface new"},
				{"/interface/description", "name=ethernet-1/2", "2nd interface"},
				{"/interface/description", "name=ethernet-1/3", "3rd interface"},
				{"/interface/description", "name=lag1", "lag interface"},
				{"/system/name/host-name", "", "srlinux-new"},
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Subscribe() initial sync (-got, +want):\n%s", diff)
			}

			// Get a second batch
			timer = time.NewTimer(1200 * time.Millisecond)
			responses = []*gnmi.SubscribeResponse{}
		outer5:
			for {
				select {
				case <-ctx.Done():
				case <-timer.C:
					break outer5
				case resp := <-subRspChan:
					if resp.SubscriptionName == subscriptionName {
						switch resp.Response.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
							responses = append(responses, resp.Response)
						default:
							t.Fatalf("Subscribe(): unknown type: %v", reflect.TypeOf(resp.Response.Response))
						}
					}
				case err := <-subErrChan:
					if err.SubscriptionName == subscriptionName {
						t.Fatalf("Subscribe() error:\n%+v", err)
					}
				}
				if len(subscribeResponsesToEvents(responses)) >= len(expected) {
					break
				}
			}
			got = subscribeResponsesToEvents(responses)
			sort.Slice(got, func(i, j int) bool {
				if got[i].Path != got[j].Path {
					return got[i].Path < got[j].Path
				}
				return got[i].Keys < got[j].Keys
			})
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Subscribe() after sampling (-got, +want):\n%s", diff)
			}
		})

		// Test a regular subscription with polling
		if t.Failed() {
			return
		}
		t.Run(fmt.Sprintf("subscribe polling %s", encoding), func(t *testing.T) {
			// This does not work as expected.
			t.Skip()
			subscribeReq, err := api.NewSubscribeRequest(
				api.Subscription(api.Path("/system/name/host-name")),
				api.Subscription(api.Path("/interface/description")),
				api.SubscriptionListModePOLL(),
				api.Encoding(encoding),
			)
			if err != nil {
				t.Fatalf("NewSubscribeRequest() error:\n%+v", err)
			}
			subscriptionName := fmt.Sprintf("polling-%s", encoding)
			go tg.Subscribe(ctx, subscribeReq, subscriptionName)
			defer tg.StopSubscription(subscriptionName)
			subRspChan, subErrChan := tg.ReadSubscriptions()

			expected := []event{
				{"/interface/description", "name=ethernet-1/1", "1st interface new"},
				{"/interface/description", "name=ethernet-1/2", "2nd interface"},
				{"/interface/description", "name=ethernet-1/3", "3rd interface"},
				{"/interface/description", "name=lag1", "lag interface"},
				{"/system/name/host-name", "", "srlinux-new"},
			}

			// Get a few batches
			time.Sleep(100 * time.Millisecond)
			for i := range 3 {
				if err := tg.SubscribePoll(ctx, subscriptionName); err != nil {
					t.Fatalf("SubscribePoll() error:\n%+v", err)
				}
				timer := time.NewTimer(time.Second)
				responses := []*gnmi.SubscribeResponse{}
			outer6:
				for {
					select {
					case <-ctx.Done():
					case <-timer.C:
						break outer6
					case resp := <-subRspChan:
						if resp.SubscriptionName == subscriptionName {
							switch resp.Response.Response.(type) {
							case *gnmi.SubscribeResponse_Update:
								responses = append(responses, resp.Response)
							default:
								t.Fatalf("Subscribe(): unknown type: %v", reflect.TypeOf(resp.Response.Response))
							}
						}
					case err := <-subErrChan:
						if err.SubscriptionName == subscriptionName {
							t.Fatalf("Subscribe() error:\n%+v", err)
						}
					}
					if len(subscribeResponsesToEvents(responses)) >= len(expected) {
						break
					}
				}
				got := subscribeResponsesToEvents(responses)
				sort.Slice(got, func(i, j int) bool {
					if got[i].Path != got[j].Path {
						return got[i].Path < got[j].Path
					}
					return got[i].Keys < got[j].Keys
				})
				if diff := helpers.Diff(got, expected); diff != "" {
					t.Fatalf("Subscribe() after polling %d (-got, +want):\n%s", i+1, diff)
				}
			}
		})
	}

	// Test the provider
	if t.Failed() {
		return
	}
	t.Run("provider", func(t *testing.T) {
		resetConfig(t)
		got := []string{}
		lo := netip.MustParseAddr("::ffff:127.0.0.1")
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.MinimalRefreshInterval = time.Second
		configP.AuthenticationParameters = helpers.MustNewSubnetMap(map[string]AuthenticationParameter{
			"::/0": {
				Username: srLinuxUsername,
				Password: srLinuxPassword,
				Insecure: true,
			},
		})
		configP.Targets = helpers.MustNewSubnetMap(map[string]netip.Addr{
			"::/0": netip.MustParseAddrPort(srLinuxGNMI).Addr(),
		})
		configP.Ports = helpers.MustNewSubnetMap(map[string]uint16{
			"::/0": netip.MustParseAddrPort(srLinuxGNMI).Port(),
		})
		put := func(update provider.Update) {
			got = append(got, fmt.Sprintf("%s %s %d %s %s %d",
				update.ExporterIP.Unmap().String(), update.Exporter.Name,
				update.IfIndex, update.Interface.Name, update.Interface.Description, update.Interface.Speed))
		}
		r := reporter.NewMock(t)
		p, err := configP.New(r, put)
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		// Let's trigger a request now
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo, IfIndexes: []uint{641}})

		// We need the indexes
		subscribeReq, err := api.NewSubscribeRequest(
			api.Subscription(api.Path("/interface/ifindex")),
			api.Subscription(api.Path("/interface/subinterface/ifindex")),
			api.SubscriptionListModeONCE(),
			api.EncodingJSON(),
		)
		if err != nil {
			t.Fatalf("NewSubscribeRequest() error:\n%+v", err)
		}
		subscribeResp, err := tg.SubscribeOnce(ctx, subscribeReq)
		if err != nil {
			t.Fatalf("SubscribeOnce() error:\n%+v", err)
		}
		indexes := map[string]uint{}
		for _, event := range subscribeResponsesToEvents(subscribeResp) {
			index, err := strconv.ParseUint(event.Value, 10, 32)
			if err != nil {
				t.Fatalf("ParseUint(%q) error:\n%+v", event.Value, err)
			}
			indexes[event.Keys] = uint(index)
		}
		t.Logf("indexes: %v", indexes)

		// Wait a bit
		time.Sleep(500 * time.Millisecond)
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo, IfIndexes: []uint{indexes["name=ethernet-1/1"]}})
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo, IfIndexes: []uint{indexes["name=ethernet-1/2"]}})
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo,
			IfIndexes: []uint{indexes["name=lag1"], indexes["name=ethernet-1/3"]}})
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo, IfIndexes: []uint{5}})
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo,
			IfIndexes: []uint{indexes["name=ethernet-1/4,index=1"]}})

		time.Sleep(50 * time.Millisecond)
		if diff := helpers.Diff(got, []string{
			fmt.Sprintf("127.0.0.1 srlinux %d ethernet-1/1 1st interface 100000", indexes["name=ethernet-1/1"]),
			fmt.Sprintf("127.0.0.1 srlinux %d ethernet-1/2 2nd interface 100000", indexes["name=ethernet-1/2"]),
			fmt.Sprintf("127.0.0.1 srlinux %d lag1 lag interface 0", indexes["name=lag1"]),
			fmt.Sprintf("127.0.0.1 srlinux %d ethernet-1/3 3rd interface 100000", indexes["name=ethernet-1/3"]),
			"127.0.0.1 srlinux 5   0",
			fmt.Sprintf("127.0.0.1 srlinux %d ethernet-1/4.1 4th interface 100000", indexes["name=ethernet-1/4,index=1"]),
		}); diff != "" {
			t.Fatalf("Query() (-got, +want):\n%s", diff)
		}

		gotMetrics := r.GetMetrics("akvorado_inlet_metadata_provider_gnmi_", "-collector_seconds")
		expectedMetrics := map[string]string{
			`collector_count`: "1",
			`collector_ready_info{exporter="127.0.0.1"}`:               "1",
			`encoding_info{encoding="json_ietf",exporter="127.0.0.1"}`: "1",
			`model_info{exporter="127.0.0.1",model="Nokia SR Linux"}`:  "1",
			`paths_count{exporter="127.0.0.1"}`:                        "82",
			`updates_total{exporter="127.0.0.1"}`:                      "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		// Change configuration and check again
		t.Log("modify and check again")
		got = []string{}
		resp, err = driver.SendConfig(`
/ interface ethernet-1/1
set description "1st interface new"

/ interface ethernet-1/4
delete subinterface 1

commit now
`)
		if err != nil {
			t.Fatalf("SendConfig() error:\n%+v", err)
		}
		if resp.Failed != nil {
			t.Fatalf("SendConfig() error:\n%+v", resp.Failed)
		}
		time.Sleep(500 * time.Millisecond) // We should exceed the second now and next request will trigger a refresh
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo, IfIndexes: []uint{indexes["name=ethernet-1/1"]}})
		time.Sleep(300 * time.Millisecond) // Do it again to get the fresh value
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo, IfIndexes: []uint{indexes["name=ethernet-1/1"]}})
		p.Query(context.Background(), &provider.BatchQuery{ExporterIP: lo,
			IfIndexes: []uint{indexes["name=ethernet-1/4,index=1"]}})
		time.Sleep(50 * time.Millisecond)
		if diff := helpers.Diff(got, []string{
			// Previous value
			fmt.Sprintf("127.0.0.1 srlinux %d ethernet-1/1 1st interface 100000", indexes["name=ethernet-1/1"]),
			// Fresh value
			fmt.Sprintf("127.0.0.1 srlinux %d ethernet-1/1 1st interface new 100000", indexes["name=ethernet-1/1"]),
			// Removed value
			fmt.Sprintf("127.0.0.1 srlinux %d   0", indexes["name=ethernet-1/4,index=1"]),
		}); diff != "" {
			t.Fatalf("Query() (-got, +want):\n%s", diff)
		}

		gotMetrics = r.GetMetrics("akvorado_inlet_metadata_provider_gnmi_", "-collector_seconds")
		expectedMetrics = map[string]string{
			`collector_count`: "1",
			`collector_ready_info{exporter="127.0.0.1"}`:               "1",
			`encoding_info{encoding="json_ietf",exporter="127.0.0.1"}`: "1",
			`model_info{exporter="127.0.0.1",model="Nokia SR Linux"}`:  "1",
			`paths_count{exporter="127.0.0.1"}`:                        "79",
			`updates_total{exporter="127.0.0.1"}`:                      "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

	})
}
