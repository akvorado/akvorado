// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/gosnmp/gosnmp"

	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

// Poll polls the SNMP provider for the requested interface indexes.
func (p *Provider) Poll(ctx context.Context, exporter, agent netip.Addr, port uint16, ifIndexes []uint, put func(provider.Update)) error {
	// Check if already have a request running
	exporterStr := exporter.Unmap().String()
	filteredIfIndexes := make([]uint, 0, len(ifIndexes))
	keys := make([]string, 0, len(ifIndexes))
	p.pendingRequestsLock.Lock()
	for _, ifIndex := range ifIndexes {
		key := fmt.Sprintf("%s@%d", exporterStr, ifIndex)
		_, ok := p.pendingRequests[key]
		if !ok {
			p.pendingRequests[key] = struct{}{}
			filteredIfIndexes = append(filteredIfIndexes, ifIndex)
			keys = append(keys, key)
		}
	}
	p.pendingRequestsLock.Unlock()
	if len(filteredIfIndexes) == 0 {
		return nil
	}
	ifIndexes = filteredIfIndexes
	defer func() {
		p.pendingRequestsLock.Lock()
		for _, key := range keys {
			delete(p.pendingRequests, key)
		}
		p.pendingRequestsLock.Unlock()
	}()

	// Instantiate an SNMP state
	g := &gosnmp.GoSNMP{
		Context:                 ctx,
		Target:                  agent.Unmap().String(),
		Port:                    port,
		Retries:                 p.config.PollerRetries,
		Timeout:                 p.config.PollerTimeout,
		UseUnconnectedUDPSocket: true,
		Logger:                  gosnmp.NewLogger(&goSNMPLogger{p.r}),
		OnRetry: func(*gosnmp.GoSNMP) {
			p.metrics.retries.WithLabelValues(exporterStr).Inc()
		},
	}
	communities := []string{""}
	if securityParameters, ok := p.config.SecurityParameters.Lookup(exporter); ok {
		g.Version = gosnmp.Version3
		g.SecurityModel = gosnmp.UserSecurityModel
		usmSecurityParameters := gosnmp.UsmSecurityParameters{
			UserName:                 securityParameters.UserName,
			AuthenticationProtocol:   gosnmp.SnmpV3AuthProtocol(securityParameters.AuthenticationProtocol),
			AuthenticationPassphrase: securityParameters.AuthenticationPassphrase,
			PrivacyProtocol:          gosnmp.SnmpV3PrivProtocol(securityParameters.PrivacyProtocol),
			PrivacyPassphrase:        securityParameters.PrivacyPassphrase,
		}
		g.SecurityParameters = &usmSecurityParameters
		if usmSecurityParameters.AuthenticationProtocol == gosnmp.NoAuth {
			if usmSecurityParameters.PrivacyProtocol == gosnmp.NoPriv {
				g.MsgFlags = gosnmp.NoAuthNoPriv
			} else {
				// Not possible
				g.MsgFlags = gosnmp.NoAuthNoPriv
			}
		} else {
			if usmSecurityParameters.PrivacyProtocol == gosnmp.NoPriv {
				g.MsgFlags = gosnmp.AuthNoPriv
			} else {
				g.MsgFlags = gosnmp.AuthPriv
			}
		}
		g.ContextName = securityParameters.ContextName
	} else {
		g.Version = gosnmp.Version2c
		communities = p.config.Communities.LookupOrDefault(exporter, []string{"public"})
	}

	start := time.Now()
	if err := g.Connect(); err != nil {
		p.metrics.errors.WithLabelValues(exporterStr, "connect").Inc()
		p.errLogger.Err(err).Str("exporter", exporterStr).Msg("unable to connect")
	}
	requests := []string{"1.3.6.1.2.1.1.5.0"}
	for _, ifIndex := range ifIndexes {
		moreRequests := []string{
			fmt.Sprintf("1.3.6.1.2.1.2.2.1.2.%d", ifIndex),     // ifDescr
			fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", ifIndex), // ifAlias
			fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.15.%d", ifIndex), // ifSpeed
		}
		requests = append(requests, moreRequests...)
	}
	var results []gosnmp.SnmpPDU
	success := false

	logError := func(err error) error {
		p.metrics.errors.WithLabelValues(exporterStr, "get").Inc()
		p.errLogger.Err(err).
			Str("exporter", exporterStr).
			Msgf("unable to GET (%d OIDs)", len(requests))
		return err
	}

	for idx, community := range communities {
		// Fatal error if last community and no success
		isLast := idx == len(communities)-1
		canError := isLast && !success

		g.Community = community
		currentResult, err := g.Get(requests)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err != nil && canError {
			return logError(err)
		}
		if err != nil {
			continue
		}
		if currentResult.Error != gosnmp.NoError && currentResult.ErrorIndex == 0 && canError {
			// There is some error affecting the whole request
			return logError(fmt.Errorf("SNMP error %s(%d)", currentResult.Error, currentResult.Error))
		}
		success = true
		if results == nil {
			results = slices.Clone(currentResult.Variables)
		} else {
			if len(results) != len(currentResult.Variables) {
				logError(fmt.Errorf("SNMP mismatch on variable lengths"))
			}
			for idx := range results {
				switch results[idx].Type {
				case gosnmp.NoSuchInstance, gosnmp.NoSuchObject, gosnmp.Null:
					results[idx] = currentResult.Variables[idx]
				}
			}
		}
	}
	if len(results) != len(requests) {
		logError(fmt.Errorf("SNMP mismatch on variable lengths"))
	}
	p.metrics.times.WithLabelValues(exporterStr).Observe(time.Now().Sub(start).Seconds())

	processStr := func(idx int, what string, target *string) bool {
		switch results[idx].Type {
		case gosnmp.OctetString:
			*target = string(results[idx].Value.([]byte))
		case gosnmp.NoSuchInstance, gosnmp.NoSuchObject, gosnmp.Null:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s missing", what)).Inc()
			return false
		default:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s unknown type", what)).Inc()
			return false
		}
		return true
	}
	processUint := func(idx int, what string, target *uint) bool {
		switch results[idx].Type {
		case gosnmp.Gauge32:
			*target = results[idx].Value.(uint)
		case gosnmp.NoSuchInstance, gosnmp.NoSuchObject, gosnmp.Null:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s missing", what)).Inc()
			return false
		default:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s unknown type", what)).Inc()
			return false
		}
		return true
	}
	var (
		sysNameVal string
	)
	if !processStr(0, "sysname", &sysNameVal) {
		return errors.New("unable to get sysName")
	}
	for idx := 1; idx < len(requests)-2; idx += 3 {
		var (
			ifDescrVal string
			ifAliasVal string
			ifSpeedVal uint
		)
		ifIndex := ifIndexes[(idx-1)/3]
		ok := true
		// We do not process results when index is 0 (this can happen for local
		// traffic, we only care for exporter name).
		if ifIndex > 0 {
			// ifDescr is not mandatory.
			processStr(idx, "ifdescr", &ifDescrVal)
		}
		if ifIndex > 0 && !processStr(idx+1, "ifalias", &ifAliasVal) {
			ok = false
		}
		if ifIndex > 0 && !processUint(idx+2, "ifspeed", &ifSpeedVal) {
			ok = false
		}
		if ok {
			p.metrics.successes.WithLabelValues(exporterStr).Inc()
		}
		put(provider.Update{
			Query: provider.Query{
				ExporterIP: exporter,
				IfIndex:    ifIndex,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: sysNameVal,
				},
				Interface: provider.Interface{
					Name:        ifDescrVal,
					Description: ifAliasVal,
					Speed:       ifSpeedVal,
				},
			},
		})
	}

	return nil
}

type goSNMPLogger struct {
	r *reporter.Reporter
}

func (l *goSNMPLogger) Print(v ...interface{}) {
	if e := l.r.Debug(); e.Enabled() {
		e.Msg(fmt.Sprint(v...))
	}
}

func (l *goSNMPLogger) Printf(format string, v ...interface{}) {
	if e := l.r.Debug(); e.Enabled() {
		e.Msg(fmt.Sprintf(format, v...))
	}
}
