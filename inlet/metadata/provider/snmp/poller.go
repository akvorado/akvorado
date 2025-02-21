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
		Version: gosnmp.Version2c,
	}
	communities := []string{"public"}
	if credentials, ok := p.config.Credentials.Lookup(exporter); ok {
		if credentials.UserName != "" {
			g.Version = gosnmp.Version3
			g.SecurityModel = gosnmp.UserSecurityModel
			usmSecurityParameters := gosnmp.UsmSecurityParameters{
				UserName:                 credentials.UserName,
				AuthenticationProtocol:   gosnmp.SnmpV3AuthProtocol(credentials.AuthenticationProtocol),
				AuthenticationPassphrase: credentials.AuthenticationPassphrase,
				PrivacyProtocol:          gosnmp.SnmpV3PrivProtocol(credentials.PrivacyProtocol),
				PrivacyPassphrase:        credentials.PrivacyPassphrase,
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
			g.ContextName = credentials.ContextName
		} else {
			g.Version = gosnmp.Version2c
			communities = credentials.Communities
		}
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
			fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.1.%d", ifIndex),  // ifName
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

	processStr := func(idx int, what string) (string, bool) {
		switch results[idx].Type {
		case gosnmp.OctetString:
			return string(results[idx].Value.([]byte)), true
		case gosnmp.NoSuchInstance, gosnmp.NoSuchObject, gosnmp.Null:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s missing", what)).Inc()
			return "", false
		default:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s unknown type", what)).Inc()
			return "", false
		}
	}
	processUint := func(idx int, what string) (uint, bool) {
		switch results[idx].Type {
		case gosnmp.Gauge32:
			return results[idx].Value.(uint), true
		case gosnmp.NoSuchInstance, gosnmp.NoSuchObject, gosnmp.Null:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s missing", what)).Inc()
			return 0, false
		default:
			p.metrics.errors.WithLabelValues(exporterStr, fmt.Sprintf("%s unknown type", what)).Inc()
			return 0, false
		}
	}
	sysNameVal, ok := processStr(0, "sysname")
	if !ok {
		return errors.New("unable to get sysName")
	}
	for idx := 1; idx < len(requests)-3; idx += 4 {
		var (
			name, description string
			speed             uint
		)
		ifIndex := ifIndexes[(idx-1)/4]
		ok := true
		// We do not process results when index is 0 (this can happen for local
		// traffic, we only care for exporter name).
		if ifIndex > 0 {
			ifDescrVal, okDescr := processStr(idx, "ifdescr")
			ifNameVal, okName := processStr(idx+1, "ifname")
			ifAliasVal, okAlias := processStr(idx+2, "ifalias")
			ifSpeedVal, okSpeed := processUint(idx+3, "ifspeed")

			// Many equipments are using ifDescr for the interface name and
			// ifAlias for the description, which is counter-intuitive. We want
			// both the name and the description.

			if okName {
				// If we have ifName, use ifDescr if it is different and ifAlias
				// is not. Otherwise, keep description empty.
				name = ifNameVal
				if okAlias && ifAliasVal != ifNameVal {
					description = ifAliasVal
				} else if okDescr && ifDescrVal != ifNameVal {
					description = ifDescrVal
				}
			} else {
				// Don't handle the other case yet. It would be unexpected to
				// have ifAlias and not ifName. And if we have only ifDescr, we
				// can't really know what this is.
				ok = false
			}

			// Speed is mandatory
			ok = ok && okSpeed
			speed = ifSpeedVal
		}
		if ok {
			p.metrics.successes.WithLabelValues(exporterStr).Inc()
		} else {
			name = ""
			description = ""
			speed = 0
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
					Name:        name,
					Description: description,
					Speed:       speed,
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
