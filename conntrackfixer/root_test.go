// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package conntrackfixer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/go-connections/nat"
	"github.com/ti-mo/conntrack"
	"go.uber.org/mock/gomock"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/conntrackfixer/mocks"
)

func TestRoot(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)
	c, err := New(r, Dependencies{
		HTTP:   h,
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Replace docker client and conntrack connection with mocks
	ctrl := gomock.NewController(t)
	dockerClientMock := mocks.NewMockDockerClient(ctrl)
	c.dockerClient = dockerClientMock
	conntrackConnMock := mocks.NewMockConntrackConn(ctrl)
	c.conntrackConn = conntrackConnMock

	dockerClientMock.EXPECT().
		Close().
		Return(nil)
	conntrackConnMock.EXPECT().
		Close().
		Return(nil)

	dockerEvents := make(chan events.Message)
	dockerErrors := make(chan error)
	dockerClientMock.EXPECT().
		Events(gomock.Any(), gomock.Any()).
		Return(dockerEvents, dockerErrors)

	// Initial trigger
	dockerClientMock.EXPECT().
		ContainerList(gomock.Any(), gomock.Any()).
		Return([]types.Container{{ID: "initial"}}, nil)
	dockerClientMock.EXPECT().
		ContainerInspect(gomock.Any(), "initial").
		Return(types.ContainerJSON{
			NetworkSettings: &types.NetworkSettings{
				NetworkSettingsBase: types.NetworkSettingsBase{
					Ports: map[nat.Port][]nat.PortBinding{
						"2055/udp": {
							nat.PortBinding{
								HostIP:   "127.0.0.1",
								HostPort: "6776",
							},
						},
					},
				},
			},
		}, nil)
	conntrackConnMock.EXPECT().
		Dump(nil).
		Return([]conntrack.Flow{
			{
				ID: 1,
				TupleOrig: conntrack.Tuple{
					Proto: conntrack.ProtoTuple{
						Protocol:        17,
						DestinationPort: 6777,
					},
				},
			}, {
				ID: 2,
				TupleOrig: conntrack.Tuple{
					Proto: conntrack.ProtoTuple{
						Protocol:        17,
						DestinationPort: 6776,
					},
				},
			},
		}, nil)
	conntrackConnMock.EXPECT().
		Delete(conntrack.Flow{
			ID: 2,
			TupleOrig: conntrack.Tuple{
				Proto: conntrack.ProtoTuple{
					Protocol:        17,
					DestinationPort: 6776,
				},
			},
		}).
		Return(nil)
	helpers.StartStop(t, c)

	// Healthcheck test
	t.Run("healthcheck", func(t *testing.T) {
		dockerClientMock.EXPECT().Ping(gomock.Any()).Return(types.Ping{}, nil)
		got := r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["conntrack-fixer"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckOK,
			Reason: "docker client alive",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}
		dockerClientMock.EXPECT().Ping(gomock.Any()).Return(types.Ping{}, errors.New("unexpected"))
		got = r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["conntrack-fixer"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckWarning,
			Reason: "docker client unavailable",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}
	})

	// New container
	t.Run("new container", func(t *testing.T) {
		dockerClientMock.EXPECT().
			ContainerList(gomock.Any(), gomock.Any()).
			Return([]types.Container{{ID: "new one"}}, nil)
		dockerClientMock.EXPECT().
			ContainerInspect(gomock.Any(), "new one").
			Return(types.ContainerJSON{
				NetworkSettings: &types.NetworkSettings{
					NetworkSettingsBase: types.NetworkSettingsBase{
						Ports: map[nat.Port][]nat.PortBinding{
							"2055/udp": {
								nat.PortBinding{
									HostIP:   "127.0.0.1",
									HostPort: "6777",
								},
							},
						},
					},
				},
			}, nil)
		conntrackConnMock.EXPECT().
			Dump(nil).
			Return([]conntrack.Flow{
				{
					ID: 3,
					TupleOrig: conntrack.Tuple{
						Proto: conntrack.ProtoTuple{
							Protocol:        6, // TCP!
							DestinationPort: 6777,
						},
					},
				}, {
					ID: 4,
					TupleOrig: conntrack.Tuple{
						Proto: conntrack.ProtoTuple{
							Protocol:        17,
							DestinationPort: 6777,
						},
					},
				}, {
					ID: 5,
					TupleOrig: conntrack.Tuple{
						Proto: conntrack.ProtoTuple{
							Protocol:        17,
							DestinationPort: 6777,
						},
					},
				},
			}, nil)
		conntrackConnMock.EXPECT().
			Delete(conntrack.Flow{
				ID: 4,
				TupleOrig: conntrack.Tuple{
					Proto: conntrack.ProtoTuple{
						Protocol:        17,
						DestinationPort: 6777,
					},
				},
			}).
			Return(errors.New("already deleted"))
		conntrackConnMock.EXPECT().
			Delete(conntrack.Flow{
				ID: 5,
				TupleOrig: conntrack.Tuple{
					Proto: conntrack.ProtoTuple{
						Protocol:        17,
						DestinationPort: 6777,
					},
				},
			}).
			Return(nil)
		dockerEvents <- events.Message{
			ID:   "new one",
			From: "some image",
		}
		time.Sleep(20 * time.Millisecond)
	})

	t.Run("metrics", func(t *testing.T) {
		gotMetrics := r.GetMetrics("akvorado_conntrackfixer_")
		expectedMetrics := map[string]string{
			`conntrack_deleted_total{container="initial",port="6776"}`: "1",
			`conntrack_deleted_total{container="new one",port="6777"}`: "1",
			`errors_total{error="cannot delete conntrack entries"}`:    "1",
			`runs_total{reason="new container"}`:                       "1",
			`runs_total{reason="start"}`:                               "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
		}
	})
}
