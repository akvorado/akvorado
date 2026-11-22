// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
)

// Example is one training example: a token for each field and a weight, usually
// the number of identical flows.
type Example struct {
	Tokens [FieldCount]int32
	Weight float32
}

// TrainOptions control the training loop.
type TrainOptions struct {
	// Epochs is the number of passes over the training set.
	Epochs int `validate:"min=1"`
	// BatchSize is the number of examples per gradient step.
	BatchSize int `validate:"min=1"`
	// LearningRate is the Adam learning rate.
	LearningRate float64 `validate:"gt=0"`
	// Seed makes the training reproducible.
	Seed uint64
	// Workers is the number of goroutines used for training. 0 uses all CPUs.
	// The result only depends on the number of workers and the seed.
	Workers int `validate:"min=0"`
}

// DefaultTrainOptions returns the default training options.
func DefaultTrainOptions() TrainOptions {
	return TrainOptions{
		Epochs:       30,
		BatchSize:    256,
		LearningRate: 0.003,
	}
}

// Train fits the model weights to the examples with Adam and a weighted
// cross-entropy loss over all predicted fields. When not nil, progress is
// called after each epoch with the mean loss.
func (m *Model) Train(examples []Example, opts TrainOptions, progress func(epoch int, loss float64)) error {
	if len(examples) == 0 {
		return errors.New("no training examples")
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	sizes := m.Metadata.vocabSizes()
	for _, example := range examples {
		if !(example.Weight > 0) {
			return fmt.Errorf("example weight %g is not positive", example.Weight)
		}
		for f, token := range example.Tokens {
			if token < 0 || int(token) >= sizes[f] {
				return fmt.Errorf("example token %d out of range for field %d", token, f)
			}
		}
	}

	grad := newWeights(&m.Metadata, nil)
	adamM := newWeights(&m.Metadata, nil)
	adamV := newWeights(&m.Metadata, nil)
	workerGrads := make([]weights, workers)
	workerScratch := make([]*scratch, workers)
	for i := range workers {
		workerGrads[i] = newWeights(&m.Metadata, nil)
		workerScratch[i] = m.newScratch()
	}

	rnd := rand.New(rand.NewPCG(opts.Seed, 1))
	perm := make([]int, len(examples))
	for i := range perm {
		perm[i] = i
	}
	step := 0
	for epoch := range opts.Epochs {
		rnd.Shuffle(len(perm), func(i, j int) {
			perm[i], perm[j] = perm[j], perm[i]
		})
		var epochLoss, epochWeight float64
		for start := 0; start < len(perm); start += opts.BatchSize {
			batch := perm[start:min(start+opts.BatchSize, len(perm))]

			// Each worker accumulates gradients for a contiguous chunk of the
			// batch into its own buffer. The last workers may get nothing, so
			// every buffer is reset here and not in the goroutines.
			chunk := (len(batch) + workers - 1) / workers
			losses := make([]float64, workers)
			batchWeight := 0.0
			var wg sync.WaitGroup
			for wid := range workers {
				zeroWeights(&workerGrads[wid])
				lo := min(wid*chunk, len(batch))
				hi := min(lo+chunk, len(batch))
				if lo == hi {
					continue
				}
				wg.Go(func() {
					for _, idx := range batch[lo:hi] {
						losses[wid] += m.backward(&examples[idx], &workerGrads[wid],
							workerScratch[wid])
					}
				})
			}
			wg.Wait()

			// Reduce in worker order to stay deterministic for a
			// given worker count.
			zeroWeights(&grad)
			gradAll := grad.all()
			for wid := range workers {
				epochLoss += losses[wid]
				for t, tensor := range workerGrads[wid].all() {
					dst := gradAll[t]
					for i, v := range tensor {
						dst[i] += v
					}
				}
			}
			for _, idx := range batch {
				batchWeight += float64(examples[idx].Weight)
			}
			epochWeight += batchWeight

			step++
			adamStep(m.w.all(), gradAll, adamM.all(), adamV.all(),
				opts.LearningRate, 1/batchWeight, step)
		}
		if progress != nil {
			progress(epoch+1, epochLoss/epochWeight)
		}
	}
	return nil
}

// scratch holds the intermediate values needed to compute the gradients of one
// example.
type scratch struct {
	x       [][]float32 // slot contributions
	acc     []float32
	u, h    [][]float32 // hidden activations per predicted field
	probs   [][]float32 // softmax outputs per predicted field
	suffix  []float32
	dlogits []float32
	dh      []float32
	da2     []float32
	du      []float32
	de      []float32
}

func (m *Model) newScratch() *scratch {
	dE, dH := m.Metadata.Hyper.EmbeddingDim, m.Metadata.Hyper.HiddenDim
	sizes := m.Metadata.vocabSizes()
	sc := scratch{
		x:       make([][]float32, FieldCount-1),
		acc:     make([]float32, dH),
		u:       make([][]float32, FieldCount),
		h:       make([][]float32, FieldCount),
		probs:   make([][]float32, FieldCount),
		suffix:  make([]float32, dH),
		dlogits: make([]float32, slices.Max(sizes[:])),
		dh:      make([]float32, dH),
		da2:     make([]float32, dH),
		du:      make([]float32, dH),
		de:      make([]float32, dE),
	}
	for j := range FieldCount - 1 {
		sc.x[j] = make([]float32, dH)
	}
	for f := 1; f < FieldCount; f++ {
		sc.u[f] = make([]float32, dH)
		sc.h[f] = make([]float32, dH)
		sc.probs[f] = make([]float32, sizes[f])
	}
	return &sc
}

// backward runs the forward and backward pass for one example and accumulates
// the weighted gradients. It returns the weighted loss.
func (m *Model) backward(example *Example, grad *weights, sc *scratch) float64 {
	dE, dH := m.Metadata.Hyper.EmbeddingDim, m.Metadata.Hyper.HiddenDim
	weight := example.Weight

	// Forward pass, keeping the activations.
	for j := range FieldCount - 1 {
		token := int(example.Tokens[j])
		table := m.w.embed[embedTable(j)]
		matVec(sc.x[j], m.w.slotW[j], table[token*dE:token*dE+dE], dH, dE)
	}
	copy(sc.acc, m.w.b1)
	loss := 0.0
	for f := 1; f < FieldCount; f++ {
		for i := range sc.acc {
			sc.acc[i] += sc.x[f-1][i]
		}
		m.forwardStep(sc.acc, f, sc.u[f], sc.h[f], sc.probs[f])
		// Turn the logits into probabilities in place.
		logits := sc.probs[f]
		maxLogit := logits[0]
		for _, l := range logits {
			maxLogit = max(maxLogit, l)
		}
		var sum float64
		for i, l := range logits {
			logits[i] = float32(math.Exp(float64(l - maxLogit)))
			sum += float64(logits[i])
		}
		inv := float32(1 / sum)
		for i := range logits {
			logits[i] *= inv
		}
		p := float64(logits[example.Tokens[f]])
		loss -= float64(weight) * math.Log(max(p, 1e-30))
	}

	// Backward pass, from the last field to the first.
	for i := range sc.suffix {
		sc.suffix[i] = 0
	}
	for f := FieldCount - 1; f >= 1; f-- {
		vocab := len(sc.probs[f])
		dlogits := sc.dlogits[:vocab]
		copy(dlogits, sc.probs[f])
		dlogits[example.Tokens[f]]--
		for i := range dlogits {
			dlogits[i] *= weight
		}
		outerAdd(grad.headW[f], dlogits, sc.h[f], vocab, dH)
		for i, v := range dlogits {
			grad.headB[f][i] += v
		}
		matVecT(sc.dh, m.w.headW[f], dlogits, vocab, dH)
		for i := range sc.da2 {
			sc.da2[i] = float32(sc.dh[i] * (1 - sc.h[f][i]*sc.h[f][i]))
		}
		outerAdd(grad.w2, sc.da2, sc.u[f], dH, dH)
		for i, v := range sc.da2 {
			grad.b2[i] += v
		}
		matVecT(sc.du, m.w.w2, sc.da2, dH, dH)
		for i := range sc.suffix {
			da := float32(sc.du[i] * (1 - sc.u[f][i]*sc.u[f][i]))
			grad.b1[i] += da
			sc.suffix[i] += da
		}

		// The context slot f-1 feeds every prediction from field f
		// on, so it receives the accumulated gradient.
		j := f - 1
		token := int(example.Tokens[j])
		table := m.w.embed[embedTable(j)]
		e := table[token*dE : token*dE+dE]
		outerAdd(grad.slotW[j], sc.suffix, e, dH, dE)
		matVecT(sc.de, m.w.slotW[j], sc.suffix, dH, dE)
		gradTable := grad.embed[embedTable(j)]
		for i, v := range sc.de {
			gradTable[token*dE+i] += v
		}
	}
	return loss
}

// zeroWeights resets all the tensors to zero.
func zeroWeights(w *weights) {
	for _, tensor := range w.all() {
		clear(tensor)
	}
}

// adamStep applies one Adam update. The gradients are scaled before use,
// usually by the inverse of the batch weight.
func adamStep(w, g, mBuf, vBuf [][]float32, lr, scale float64, step int) {
	const beta1, beta2, eps = 0.9, 0.999, 1e-8
	c1 := 1 / (1 - math.Pow(beta1, float64(step)))
	c2 := 1 / (1 - math.Pow(beta2, float64(step)))
	for t := range w {
		wt, gt, mt, vt := w[t], g[t], mBuf[t], vBuf[t]
		for i := range wt {
			gi := float64(gt[i]) * scale
			mi := float64(beta1*float64(mt[i])) + float64((1-beta1)*gi)
			vi := float64(beta2*float64(vt[i])) + float64((1-beta2)*gi*gi)
			mt[i] = float32(mi)
			vt[i] = float32(vi)
			update := lr * (mi * c1) / (math.Sqrt(vi*c2) + eps)
			wt[i] = float32(float64(wt[i]) - update)
		}
	}
}
