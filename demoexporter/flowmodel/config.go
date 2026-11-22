// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package flowmodel implements a small generative model for network flows. The
// model is an autoregressive neural network over a fixed sequence of tokens
// describing one flow. It is trained on CPU from flows stored in ClickHouse and
// sampled on CPU by the demo exporter.
package flowmodel

// Hyperparameters control the size of the model. The vocabulary fields are
// upper bounds used when building the vocabularies during training. The
// dimensions are used both during training and when loading a model.
type Hyperparameters struct {
	// EmbeddingDim is the size of the token embeddings.
	EmbeddingDim int `validate:"min=2,max=64"`
	// HiddenDim is the size of the hidden layers.
	HiddenDim int `validate:"min=4,max=256"`
	// RemotePrefixes is the maximum number of remote prefixes in the vocabulary.
	RemotePrefixes int `validate:"min=4,max=4096"`
	// LocalPrefixes is the maximum number of local prefixes in the vocabulary.
	LocalPrefixes int `validate:"min=4,max=1024"`
	// Ports is the maximum number of entries in the port vocabulary,
	// including the fixed range buckets.
	Ports int `validate:"min=16,max=1024"`
	// ProtoEtypes is the maximum number of protocol and ethernet type
	// pairs in the vocabulary.
	ProtoEtypes int `validate:"min=2,max=256"`
}

// DefaultHyperparameters returns the default hyperparameters. They
// produce a model well below 1 MB.
func DefaultHyperparameters() Hyperparameters {
	return Hyperparameters{
		EmbeddingDim:   16,
		HiddenDim:      64,
		RemotePrefixes: 512,
		LocalPrefixes:  64,
		Ports:          72,
		ProtoEtypes:    16,
	}
}
