package ovs

import (
	"context"
	"net/netip"
	"testing"

	ovsclient "github.com/ovn-org/libovsdb/client"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

type mockClient struct {
	list []map[string]interface{}
	ch   chan ovsclient.TableUpdates
}

func (m *mockClient) MonitorAll(ctx context.Context, tables []string) (<-chan ovsclient.TableUpdates, error) {
	return m.ch, nil
}
func (m *mockClient) List(ctx context.Context, table string, out interface{}) error { return nil }
func (m *mockClient) Close()                                                        {}

func TestQueryAndWatch(t *testing.T) {
	updates := make(chan ovsclient.TableUpdates, 1)
	mc := &mockClient{ch: updates}
	newClient = func(cfg Configuration) (ovsclient.Client, error) { return mc, nil }
	r := reporter.NewMock(t)
	var got []provider.Update
	p, err := Configuration{}.New(r, func(u provider.Update) { got = append(got, u) })
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	updates <- ovsclient.TableUpdates{"Interface": {Rows: map[string]ovsclient.RowUpdate{"1": {New: map[string]interface{}{"ofport": 1, "name": "eth1"}}}}}
	close(updates)

	p.Query(context.Background(), &provider.BatchQuery{ExporterIP: netip.Addr{}, IfIndexes: []uint{1}})

	expected := []provider.Update{{Query: provider.Query{IfIndex: 1}, Answer: provider.Answer{Interface: provider.Interface{Name: "eth1"}}}}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("updates (-got,+want):\n%s", diff)
	}
}
