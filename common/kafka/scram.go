// SPDX-FileCopyrightText: 2013 Shopify
// SPDX-License-Identifier: MIT

// From https://github.com/IBM/sarama/blob/main/examples/sasl_scram_client/scram_client.go

package kafka

import "github.com/xdg-go/scram"

type xdgSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *xdgSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *xdgSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *xdgSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
