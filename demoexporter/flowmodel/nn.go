// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"math"
	"math/rand/v2"
)

// Model is a generative flow model: metadata describing the vocabularies and
// the weights of a small autoregressive network. See:
//
// - https://github.com/karpathy/makemore
// - https://www.jmlr.org/papers/volume3/bengio03a/bengio03a.pdf
//
// The network predicts the token of each field from the tokens of the previous
// fields. The context is an additive sum of per-slot linear projections of the
// token embeddings, which makes sampling incremental and cheap:
//
//	a_i = b1 + sum for j<i of slotW_j · embed(t_j)
//	h_i = tanh(w2 · tanh(a_i) + b2)
//	logits_i = headW_i · h_i + headB_i
type Model struct {
	Metadata Metadata
	w        weights
}

// weights holds all the tensors of the network. The same structure is reused to
// hold gradients and Adam moments.
type weights struct {
	embed [][]float32 // per context slot, nil for slots sharing another table
	slotW [][]float32 // per context slot, hiddenDim×embeddingDim
	b1    []float32   // bias for first layer
	w2    []float32   // hiddenDim×hiddenDim
	b2    []float32   // bias for second layer
	headW [][]float32 // per predicted field, vocab×hiddenDim, nil for field 0
	headB [][]float32
}

// embedTable returns the field owning the embedding table used by a field.
// Source and destination ports share one table.
func embedTable(f int) int {
	if f == FieldDstPort {
		return FieldSrcPort
	}
	return f
}

// all returns every tensor in a fixed order. This order defines the
// serialization format and the Adam state layout.
func (w *weights) all() [][]float32 {
	out := make([][]float32, 0, 4*FieldCount)
	for j := range FieldCount - 1 {
		if embedTable(j) == j {
			out = append(out, w.embed[j])
		}
	}
	for j := range FieldCount - 1 {
		out = append(out, w.slotW[j])
	}
	out = append(out, w.b1, w.w2, w.b2)
	for f := 1; f < FieldCount; f++ {
		out = append(out, w.headW[f], w.headB[f])
	}
	return out
}

// newWeights allocates the tensors for the given metadata. When rnd is not nil,
// weights are initialized to small random values, else they are left at zero.
func newWeights(meta *Metadata, rnd *rand.Rand) weights {
	dE, dH := meta.Hyper.EmbeddingDim, meta.Hyper.HiddenDim
	sizes := meta.vocabSizes()
	w := weights{
		embed: make([][]float32, FieldCount-1),
		slotW: make([][]float32, FieldCount-1),
		b1:    make([]float32, dH),
		w2:    make([]float32, dH*dH),
		b2:    make([]float32, dH),
		headW: make([][]float32, FieldCount),
		headB: make([][]float32, FieldCount),
	}
	for j := range FieldCount - 1 {
		if embedTable(j) == j {
			w.embed[j] = make([]float32, sizes[j]*dE)
		}
	}
	for j := range FieldCount - 1 {
		w.slotW[j] = make([]float32, dH*dE)
	}
	for f := 1; f < FieldCount; f++ {
		w.headW[f] = make([]float32, sizes[f]*dH)
		w.headB[f] = make([]float32, sizes[f])
	}
	if rnd != nil {
		initUniform := func(data []float32, scale float32) {
			for i := range data {
				data[i] = (rnd.Float32()*2 - 1) * scale
			}
		}
		for _, table := range w.embed {
			if table != nil {
				initUniform(table, 0.05)
			}
		}
		scaleE := float32(1 / math.Sqrt(float64(dE)))
		scaleH := float32(1 / math.Sqrt(float64(dH)))
		for j := range FieldCount - 1 {
			initUniform(w.slotW[j], scaleE)
		}
		initUniform(w.w2, scaleH)
		for f := 1; f < FieldCount; f++ {
			initUniform(w.headW[f], scaleH)
		}
	}
	return w
}

// New creates a model with freshly initialized weights.
func New(meta Metadata, seed uint64) *Model {
	rnd := rand.New(rand.NewPCG(seed, 0))
	return &Model{
		Metadata: meta,
		w:        newWeights(&meta, rnd),
	}
}

// Parameters returns the number of weights of the model.
func (m *Model) Parameters() int {
	count := 0
	for _, tensor := range m.w.all() {
		count += len(tensor)
	}
	return count
}

// contextAdd adds the contribution of a token at a given slot to the context
// vector.
func (m *Model) contextAdd(acc []float32, slot int, token int32) {
	dE := m.Metadata.Hyper.EmbeddingDim
	table := m.w.embed[embedTable(slot)]
	e := table[int(token)*dE : int(token)*dE+dE]
	matVecAdd(acc, m.w.slotW[slot], e, len(acc), dE)
}

// forwardStep computes the hidden layers and the logits for a predicted field
// from the accumulated context vector. The u, h and logits slices are filled by
// the call.
func (m *Model) forwardStep(acc []float32, field int, u, h, logits []float32) {
	dH := m.Metadata.Hyper.HiddenDim
	for i := range u {
		u[i] = tanh32(acc[i])
	}
	matVec(h, m.w.w2, u, dH, dH)
	for i := range h {
		h[i] = tanh32(h[i] + m.w.b2[i])
	}
	matVec(logits, m.w.headW[field], h, len(logits), dH)
	for i := range logits {
		logits[i] += m.w.headB[field][i]
	}
}

// tanh32 is a float32 tanh.
func tanh32(x float32) float32 {
	return float32(math.Tanh(float64(x)))
}

// The float32 conversions in the linear algebra helpers force the products to
// be rounded, so the compiler cannot use fused multiply-add. The results can
// still differ a bit between architectures, as exp and tanh have several
// implementations.

// matVec computes dst = w·x with w a rows×cols row-major matrix.
func matVec(dst, w, x []float32, rows, cols int) {
	for r := range rows {
		row := w[r*cols : r*cols+cols]
		var s float32
		for c, xv := range x {
			s += float32(row[c] * xv)
		}
		dst[r] = s
	}
}

// matVecAdd computes dst += w·x with w a rows×cols row-major matrix.
func matVecAdd(dst, w, x []float32, rows, cols int) {
	for r := range rows {
		row := w[r*cols : r*cols+cols]
		s := dst[r]
		for c, xv := range x {
			s += float32(row[c] * xv)
		}
		dst[r] = s
	}
}

// matVecT computes dst = wᵀ·y with w a rows×cols row-major matrix.
func matVecT(dst, w, y []float32, rows, cols int) {
	for c := range cols {
		dst[c] = 0
	}
	for r := range rows {
		row := w[r*cols : r*cols+cols]
		yv := y[r]
		for c := range cols {
			dst[c] += float32(row[c] * yv)
		}
	}
}

// outerAdd computes w += y⊗x with w a rows×cols row-major matrix.
func outerAdd(w, y, x []float32, rows, cols int) {
	for r := range rows {
		row := w[r*cols : r*cols+cols]
		yv := y[r]
		for c, xv := range x {
			row[c] += float32(yv * xv)
		}
	}
}
