package netflow

import (
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"

	"akvorado/flow/decoder"
	"akvorado/helpers"
	"akvorado/reporter"
)

func TestDecode(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r)

	// Send an option template
	template, err := ioutil.ReadFile(filepath.Join("testdata", "options-template-257.data"))
	if err != nil {
		panic(err)
	}
	got := nfdecoder.Decode(template, net.ParseIP("127.0.0.1"))
	if got == nil {
		t.Fatalf("Decode() error on options template")
	}
	if len(got) != 0 {
		t.Fatalf("Decode() on options template got flows")
	}

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_flow_decoder_netflow_")
	expectedMetrics := map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "1",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send option data
	data, err := ioutil.ReadFile(filepath.Join("testdata", "options-data-257.data"))
	if err != nil {
		panic(err)
	}
	got = nfdecoder.Decode(data, net.ParseIP("127.0.0.1"))
	if got == nil {
		t.Fatalf("Decode() error on options data")
	}
	if len(got) != 0 {
		t.Fatalf("Decode() on options data got flows")
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_flow_decoder_netflow_")
	expectedMetrics = map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "2",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send a regular template
	template, err = ioutil.ReadFile(filepath.Join("testdata", "template-260.data"))
	if err != nil {
		panic(err)
	}
	got = nfdecoder.Decode(template, net.ParseIP("127.0.0.1"))
	if got == nil {
		t.Fatalf("Decode() error on template")
	}
	if len(got) != 0 {
		t.Fatalf("Decode() on template got flows")
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_flow_decoder_netflow_")
	expectedMetrics = map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "3",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send data
	data, err = ioutil.ReadFile(filepath.Join("testdata", "data-260.data"))
	if err != nil {
		panic(err)
	}
	got = nfdecoder.Decode(data, net.ParseIP("127.0.0.1"))
	if got == nil {
		t.Fatalf("Decode() error on data")
	}
	expectedFlows := []*decoder.FlowMessage{
		{
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1500,
			Packets:          1,
			SrcAddr:          net.ParseIP("198.38.121.178").To16(),
			DstAddr:          net.ParseIP("91.170.143.87").To16(),
			SrcNet:           24,
			DstNet:           14,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          19624,
			InIf:             335,
			OutIf:            450,
			ForwardingStatus: 64,
			TCPFlags:         16,
		}, {
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1500,
			Packets:          1,
			SrcAddr:          net.ParseIP("198.38.121.219").To16(),
			DstAddr:          net.ParseIP("88.122.57.97").To16(),
			SrcNet:           24,
			DstNet:           14,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          2444,
			InIf:             335,
			OutIf:            452,
			ForwardingStatus: 64,
			TCPFlags:         16,
		}, {
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1400,
			Packets:          1,
			SrcAddr:          net.ParseIP("173.194.190.106").To16(),
			DstAddr:          net.ParseIP("37.165.129.20").To16(),
			SrcNet:           20,
			DstNet:           18,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          53697,
			InIf:             461,
			OutIf:            306,
			ForwardingStatus: 64,
			TCPFlags:         16,
		}, {
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1448,
			Packets:          1,
			SrcAddr:          net.ParseIP("74.125.100.234").To16(),
			DstAddr:          net.ParseIP("88.120.219.117").To16(),
			SrcNet:           16,
			DstNet:           14,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          52300,
			InIf:             461,
			OutIf:            451,
			ForwardingStatus: 64,
			TCPFlags:         16,
		},
	}
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
	gotMetrics = r.GetMetrics(
		"akvorado_flow_decoder_netflow_",
		"count",
		"flowset_",
		"templates_",
	)
	expectedMetrics = map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "4",
		`flowset_records_sum{sampler="127.0.0.1",type="DataFlowSet",version="9"}`:                                      "4",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{sampler="127.0.0.1",type="DataFlowSet",version="9"}`:                                              "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}
