// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"fmt"
	"net"
	"strconv"

	"github.com/slayercat/GoSNMPServer"
	"github.com/slayercat/gosnmp"
)

func (c *Component) newOID(oid string, t gosnmp.Asn1BER, onGet GoSNMPServer.FuncPDUControlGet) *GoSNMPServer.PDUValueControlItem {
	return &GoSNMPServer.PDUValueControlItem{
		OID:  oid,
		Type: t,
		OnGet: func() (interface{}, error) {
			c.metrics.requests.WithLabelValues(oid).Inc()
			return onGet()
		},
	}
}

func (c *Component) startSNMPServer() error {
	oids := make([]*GoSNMPServer.PDUValueControlItem, 1+3*len(c.config.Interfaces))
	oids[0] = c.newOID("1.3.6.1.2.1.1.5.0",
		gosnmp.OctetString,
		func() (interface{}, error) {
			return c.config.Name, nil
		},
	)
	count := 0
	for idx, description := range c.config.Interfaces {
		i := idx
		d := description
		oids[3*count+1] = c.newOID(fmt.Sprintf("1.3.6.1.2.1.2.2.1.2.%d", i),
			gosnmp.OctetString,
			func() (interface{}, error) {
				return fmt.Sprintf("Gi0/0/0/%d", i), nil
			},
		)
		oids[3*count+2] = c.newOID(fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.15.%d", i),
			gosnmp.Gauge32,
			func() (interface{}, error) {
				return uint(10000), nil
			},
		)
		oids[3*count+3] = c.newOID(fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", i),
			gosnmp.OctetString,
			func() (interface{}, error) {
				return d, nil
			},
		)
		count++
	}
	agent := GoSNMPServer.MasterAgent{
		SubAgents: []*GoSNMPServer.SubAgent{
			{
				CommunityIDs: []string{"public"},
				OIDs:         oids,
			},
		},
	}
	server := GoSNMPServer.NewSNMPServer(agent)
	err := server.ListenUDP("udp", c.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to bind SNMP server: %w", err)
	}
	c.t.Go(func() error {
		<-c.t.Dying()
		server.Shutdown()
		return nil
	})

	// Get port for easier testing
	_, portStr, err := net.SplitHostPort(server.Address().String())
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	c.snmpPort = port

	c.r.Debug().Int("port", port).Msg("SNMP server listening")
	c.t.Go(server.ServeForever)
	return nil
}
