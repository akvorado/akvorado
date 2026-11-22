// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

// trainExamples builds a small distribution: inbound TCP flows to
// port 443 dominate, with a 3:1 ratio between the two remote
// prefixes.
func trainExamples() []Example {
	base := Example{}
	// Friday at 15:00 UTC, the hour used when sampling in the tests.
	base.Tokens[FieldHourOfWeek] = 135
	base.Tokens[FieldDirection] = int32(DirectionInbound)
	base.Tokens[FieldLocalBoundary] = int32(schema.InterfaceBoundaryInternal)
	base.Tokens[FieldProtoEtype] = 0 // IPv4 TCP
	base.Tokens[FieldLocalPrefix] = 0
	base.Tokens[FieldSrcPort] = 7 // 443
	base.Tokens[FieldDstPort] = 6 // 49152-65535
	base.Tokens[FieldSizeBucket] = 8
	base.Tokens[FieldPacketBucket] = 3

	examples := []Example{}
	google := base
	google.Tokens[FieldRemotePrefix] = 0
	google.Weight = 300
	examples = append(examples, google)
	netflix := base
	netflix.Tokens[FieldRemotePrefix] = 2
	netflix.Weight = 100
	examples = append(examples, netflix)
	return examples
}

func TestTrain(t *testing.T) {
	model := NewMock(t)
	examples := trainExamples()
	losses := []float64{}
	err := model.Train(examples, TrainOptions{
		Epochs:       200,
		BatchSize:    16,
		LearningRate: 0.01,
		Workers:      1,
	}, func(_ int, loss float64) {
		losses = append(losses, loss)
	})
	if err != nil {
		t.Fatalf("Train() error:\n%+v", err)
	}
	first, last := losses[0], losses[len(losses)-1]
	if last >= first {
		t.Fatalf("Train() loss did not decrease: %g to %g", first, last)
	}
	if last > 1 {
		t.Fatalf("Train() final loss too high: %g", last)
	}

	// The sampled flows should follow the learned distribution.
	sampler, err := model.NewSampler(SamplerOptions{Rate: 50, LocalASN: 64496, MaxFlowsPerSecond: 5000})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	counts := map[uint32]int{}
	total, stray := 0, 0
	for i := range 40 {
		for _, flow := range sampler.Generate(0, now.Add(time.Duration(i)*time.Second), nil) {
			total++
			if flow.Direction != DirectionInbound || flow.Proto != 6 || flow.SrcPort != 443 {
				stray++
				continue
			}
			counts[flow.RemoteASN]++
		}
	}
	if total == 0 {
		t.Fatalf("Generate() did not produce any flow")
	}
	// The softmax cannot give a zero probability to unseen tokens, so
	// a few flows outside the training distribution are expected.
	if float64(stray) > 0.05*float64(total) {
		t.Fatalf("Generate() produced %d/%d flows outside the training distribution",
			stray, total)
	}
	googleShare := float64(counts[15169]) / float64(total-stray)
	if googleShare < 0.6 || googleShare > 0.9 {
		t.Fatalf("Generate() gives %g%% of flows to AS15169, expected around 75%%",
			googleShare*100)
	}
}

func TestTrainDeterminism(t *testing.T) {
	first := NewMock(t)
	second := NewMock(t)
	examples := trainExamples()
	opts := TrainOptions{
		Epochs:       10,
		BatchSize:    16,
		LearningRate: 0.01,
		Workers:      1,
		Seed:         42,
	}
	if err := first.Train(examples, opts, nil); err != nil {
		t.Fatalf("Train() error:\n%+v", err)
	}
	if err := second.Train(examples, opts, nil); err != nil {
		t.Fatalf("Train() error:\n%+v", err)
	}
	if diff := helpers.Diff(second.w.all(), first.w.all()); diff != "" {
		t.Fatalf("Train() twice gives different weights (-got, +want):\n%s", diff)
	}
}

// TestTrainIdleWorker checks the batches leaving some workers without work.
// Their gradient buffer must not reach the update, otherwise the previous batch
// is applied twice.
func TestTrainIdleWorker(t *testing.T) {
	examples := trainExamples()
	third := examples[0]
	third.Tokens[FieldRemotePrefix] = 1
	third.Tokens[FieldSrcPort] = 3
	third.Tokens[FieldSizeBucket] = 2
	third.Weight = 50
	examples = append(examples, third)

	// With three examples and batches of two, the second batch only keeps the
	// first worker busy.
	opts := TrainOptions{
		Epochs:       3,
		BatchSize:    2,
		LearningRate: 0.01,
		Seed:         7,
	}
	single, double := NewMock(t), NewMock(t)
	opts.Workers = 1
	if err := single.Train(examples, opts, nil); err != nil {
		t.Fatalf("Train() error:\n%+v", err)
	}
	opts.Workers = 2
	if err := double.Train(examples, opts, nil); err != nil {
		t.Fatalf("Train() error:\n%+v", err)
	}
	// Splitting a batch changes the order of the sums, so the weights only
	// match closely. An idle worker adding its previous gradients again would
	// move them far more than that.
	worst := 0.0
	singleTensors := single.w.all()
	for i, tensor := range double.w.all() {
		for j, v := range tensor {
			worst = max(worst, math.Abs(float64(v-singleTensors[i][j])))
		}
	}
	t.Logf("largest weight difference between 1 and 2 workers: %g", worst)
	if worst > 1e-5 {
		t.Fatalf("Train() with 2 workers differs by %g from 1 worker", worst)
	}
}

func TestTrainBadToken(t *testing.T) {
	model := NewMock(t)
	examples := trainExamples()
	examples[0].Tokens[FieldRemotePrefix] = 100
	if err := model.Train(examples, TrainOptions{Epochs: 1, BatchSize: 16, LearningRate: 0.01}, nil); err == nil {
		t.Fatalf("Train() did not error on an out-of-range token")
	}
}

func TestGradients(t *testing.T) {
	model := NewMock(t)
	// Initial weights keep tanh almost linear, which hides errors on its
	// derivative.
	rnd := rand.New(rand.NewPCG(1, 1))
	for _, tensor := range model.w.all() {
		for i := range tensor {
			tensor[i] = rnd.Float32()*2 - 1
		}
	}
	example := trainExamples()[0]
	example.Weight = 1
	sc := model.newScratch()
	grad := newWeights(&model.Metadata, nil)
	model.backward(&example, &grad, sc)

	// Compare each gradient with a central finite difference of the loss.
	unused := newWeights(&model.Metadata, nil)
	const eps = 1e-2
	gradAll := grad.all()
	for ti, tensor := range model.w.all() {
		for i, orig := range tensor {
			tensor[i] = orig + eps
			plus := tensor[i]
			lossPlus := model.backward(&example, &unused, sc)
			tensor[i] = orig - eps
			minus := tensor[i]
			lossMinus := model.backward(&example, &unused, sc)
			tensor[i] = orig
			numeric := (lossPlus - lossMinus) / float64(plus-minus)
			analytic := float64(gradAll[ti][i])
			if math.Abs(numeric-analytic) > 1e-3+1e-2*math.Abs(numeric) {
				t.Fatalf("backward() gradient for tensor %d, index %d: %g, expected %g",
					ti, i, analytic, numeric)
			}
		}
	}
}
