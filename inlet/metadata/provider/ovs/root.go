package ovs

import (
	"context"
	"sync"

	ovsclient "github.com/ovn-org/libovsdb/client"

	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

// newClient is used to create a new OVSDB client. It is overridden in tests.
var newClient = func(cfg Configuration) (ovsclient.Client, error) {
	return ovsclient.NewOVSDBClient()
}

// Provider implements metadata retrieval using OVSDB.
type Provider struct {
	r      *reporter.Reporter
	client ovsclient.Client
	put    func(provider.Update)

	mu    sync.RWMutex
	cache map[uint]provider.Interface
}

// New creates a new OVS provider.
func (cfg Configuration) New(r *reporter.Reporter, put func(provider.Update)) (provider.Provider, error) {
	cli, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	p := &Provider{
		r:      r,
		client: cli,
		put:    put,
		cache:  map[uint]provider.Interface{},
	}
	go p.watch(context.Background())
	return p, nil
}

// Query returns cached interface information.
func (p *Provider) Query(_ context.Context, q *provider.BatchQuery) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, idx := range q.IfIndexes {
		if iface, ok := p.cache[idx]; ok {
			p.put(provider.Update{Query: provider.Query{ExporterIP: q.ExporterIP, IfIndex: idx}, Answer: provider.Answer{Interface: iface}})
		}
	}
	return nil
}

func (p *Provider) watch(ctx context.Context) {
	ch, err := p.client.MonitorAll(ctx, []string{"Interface"})
	if err != nil {
		return
	}
	for update := range ch {
		table, ok := update["Interface"]
		if !ok {
			continue
		}
		for _, row := range table.Rows {
			ofport, _ := row.New["ofport"].(int)
			name, _ := row.New["name"].(string)
			mac, _ := row.New["mac"].(string)
			mtu, _ := row.New["mtu"].(int)
			stats, _ := row.New["statistics"].(map[string]interface{})
			iface := provider.Interface{Name: name}
			p.mu.Lock()
			p.cache[uint(ofport)] = iface
			p.mu.Unlock()
			_ = mac
			_ = mtu
			_ = stats
			p.put(provider.Update{Query: provider.Query{IfIndex: uint(ofport)}, Answer: provider.Answer{Interface: iface}})
		}
	}
}
