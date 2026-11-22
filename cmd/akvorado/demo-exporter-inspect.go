// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !akvorado_inlet

package main

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"akvorado/common/constants"
	"akvorado/common/embed"
	"akvorado/demoexporter/flowmodel"
)

type demoExporterInspectOptions struct {
	Flows int
}

// DemoExporterInspectOptions stores the command-line option values for the demo
// exporter inspect command.
var DemoExporterInspectOptions demoExporterInspectOptions

var demoExporterInspectCmd = &cobra.Command{
	Use:   "inspect [model]",
	Short: "Show statistics about a demo exporter flow model",
	Long: `Show the content of a flow model used by the demo exporter. Without
argument, inspect the embedded default model.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		model, err := flowmodel.LoadFromPath(path)
		if err != nil {
			return err
		}
		demoExporterInspect(cmd.OutOrStdout(), model)
		if err := demoExporterInspectSpeed(cmd.OutOrStdout(), model, time.Second); err != nil {
			return err
		}
		return demoExporterInspectFlows(cmd.OutOrStdout(), model, DemoExporterInspectOptions.Flows)
	},
}

func init() {
	demoExporterCmd.AddCommand(demoExporterInspectCmd)
	demoExporterInspectCmd.Flags().IntVarP(&DemoExporterInspectOptions.Flows, "flows", "n", 10,
		"Number of flows to generate")
}

func demoExporterInspect(w io.Writer, model *flowmodel.Model) {
	meta := &model.Metadata
	fmt.Fprintf(w, "%s %d parameters, embeddings of %d, hidden layers of %d\n",
		label(w, "Model"), model.Parameters(), meta.Hyper.EmbeddingDim, meta.Hyper.HiddenDim)

	// Catch-all entries have a zero-length prefix.
	lengths := map[string]int{}
	hostBits := map[uint8]int{}
	locals := 0
	for _, info := range meta.LocalPrefixes {
		if info.Prefix.Bits() == 0 {
			continue
		}
		locals++
		family := "IPv6"
		if info.Prefix.Addr().Is4() {
			family = "IPv4"
		}
		lengths[fmt.Sprintf("%s /%d", family, info.Prefix.Bits())]++
		hostBits[info.HostBits]++
	}
	fmt.Fprintf(w, "%s %d\n", label(w, "Local prefixes"), locals)
	for _, length := range slices.Sorted(maps.Keys(lengths)) {
		fmt.Fprintf(w, "  %s: %d\n", length, lengths[length])
	}
	bits := []string{}
	for _, b := range slices.Sorted(maps.Keys(hostBits)) {
		bits = append(bits, fmt.Sprintf("%d hosts: %d", 1<<b, hostBits[b]))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(bits, ", "))

	asns := map[uint32]int{}
	remotes := 0
	for _, info := range meta.RemotePrefixes {
		if info.Prefix.Bits() == 0 {
			continue
		}
		remotes++
		asns[info.ASN]++
	}
	fmt.Fprintf(w, "%s %d from %d AS numbers\n", label(w, "Remote prefixes"), remotes, len(asns))
	sorted := slices.SortedFunc(maps.Keys(asns), func(a, b uint32) int {
		return cmp.Or(cmp.Compare(asns[b], asns[a]), cmp.Compare(a, b))
	})
	sorted = sorted[:min(len(sorted), 20)]
	names := asnNames(sorted)
	for _, asn := range sorted {
		if name, ok := names[asn]; ok {
			fmt.Fprintf(w, "  AS%d (%s): %d\n", asn, name, asns[asn])
		} else {
			fmt.Fprintf(w, "  AS%d: %d\n", asn, asns[asn])
		}
	}

	protocols := []string{}
	for _, pe := range meta.ProtoEtypes {
		protocols = append(protocols, protoEtypeName(pe))
	}
	fmt.Fprintf(w, "%s %s\n", label(w, "Protocols"), strings.Join(protocols, ", "))

	ports := []uint16{}
	for _, port := range meta.Ports {
		if port.Start == port.End {
			ports = append(ports, port.Start)
		}
	}
	slices.Sort(ports)
	fmt.Fprintf(w, "%s %s\n", label(w, "Ports"), strings.Trim(fmt.Sprint(ports), "[]"))

	empty := 0
	for _, rate := range meta.RateProfile {
		if rate == 0 {
			empty++
		}
	}
	fmt.Fprintf(w, "%s min %.2f, max %.2f, %d hours without flows\n",
		label(w, "Rate profile"), slices.Min(meta.RateProfile[:]), slices.Max(meta.RateProfile[:]), empty)
}

// demoExporterInspectSpeed measures how many flows one sampler generates per
// second.
func demoExporterInspectSpeed(w io.Writer, model *flowmodel.Model, duration time.Duration) error {
	sampler, err := model.NewSampler(flowmodel.SamplerOptions{
		Rate:              10000,
		LocalASN:          64496,
		MaxFlowsPerSecond: 10000,
	})
	if err != nil {
		return err
	}
	// Each call generates the flows of the next second of model time.
	count := 0
	now := time.Now()
	start := now
	for time.Since(start) < duration {
		count += len(sampler.Generate(0, now, nil))
		now = now.Add(time.Second)
	}
	fmt.Fprintf(w, "%s %.0f flows/s on one core\n", label(w, "Speed"), float64(count)/time.Since(start).Seconds())
	return nil
}

func demoExporterInspectFlows(w io.Writer, model *flowmodel.Model, count int) error {
	if count <= 0 {
		return nil
	}
	sampler, err := model.NewSampler(flowmodel.SamplerOptions{
		Rate:              float64(count),
		LocalASN:          64496,
		MaxFlowsPerSecond: count,
	})
	if err != nil {
		return err
	}
	// Some hours may have no flow, try the next ones.
	flows := []flowmodel.Flow{}
	now := time.Now()
	for range 168 {
		if len(flows) >= count {
			break
		}
		flows = append(flows, sampler.Generate(0, now, nil)...)
		now = now.Add(time.Hour)
	}
	fmt.Fprintf(w, "%s\n", label(w, "Flows"))
	for _, flow := range flows[:min(len(flows), count)] {
		direction := "in "
		if flow.Direction == flowmodel.DirectionOutbound {
			direction = "out"
		}
		fmt.Fprintf(w, "  %s %s %s (/%d, AS%d) -> %s (/%d, AS%d), %d packets, %d bytes, local %s\n",
			direction, protoEtypeName(flowmodel.ProtoEtype{EType: flow.EType, Proto: flow.Proto}),
			netip.AddrPortFrom(flow.SrcAddr, flow.SrcPort), flow.SrcMask, flow.SrcAS,
			netip.AddrPortFrom(flow.DstAddr, flow.DstPort), flow.DstMask, flow.DstAS,
			flow.Packets, flow.Octets, flow.LocalBoundary)
	}
	return nil
}

// asnNames returns the names of the provided AS numbers, from the list embedded
// for ClickHouse. Unknown AS numbers are missing from the result.
func asnNames(asns []uint32) map[uint32]string {
	names := map[uint32]string{}
	f, err := embed.Data().Open("orchestrator/clickhouse/data/asns.csv")
	if err != nil {
		return names
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = 2
	for {
		record, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		asn, err := strconv.ParseUint(record[0], 10, 32)
		if err == nil && slices.Contains(asns, uint32(asn)) {
			names[uint32(asn)] = record[1]
		}
	}
	return names
}

// label formats a label, in bold on a terminal.
func label(w io.Writer, name string) string {
	if f, ok := w.(*os.File); ok && isatty.IsTerminal(f.Fd()) {
		return fmt.Sprintf("\033[1m%s:\033[0m", name)
	}
	return fmt.Sprintf("%s:", name)
}

func protoEtypeName(pe flowmodel.ProtoEtype) string {
	etype := fmt.Sprintf("0x%04x", pe.EType)
	switch pe.EType {
	case constants.ETypeIPv4:
		etype = "IPv4"
	case constants.ETypeIPv6:
		etype = "IPv6"
	}
	proto := fmt.Sprintf("%d", pe.Proto)
	switch pe.Proto {
	case constants.ProtoTCP:
		proto = "TCP"
	case constants.ProtoUDP:
		proto = "UDP"
	case constants.ProtoICMPv4, constants.ProtoICMPv6:
		proto = "ICMP"
	}
	return fmt.Sprintf("%s/%s", etype, proto)
}
