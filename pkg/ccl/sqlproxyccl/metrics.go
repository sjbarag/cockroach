// Copyright 2020 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package sqlproxyccl

import (
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/errors"
)

// metrics contains pointers to the metrics for monitoring proxy operations.
type metrics struct {
	BackendDisconnectCount *metric.Counter
	IdleDisconnectCount    *metric.Counter
	BackendDownCount       *metric.Counter
	ClientDisconnectCount  *metric.Counter
	CurConnCount           *metric.Gauge
	RoutingErrCount        *metric.Counter
	RefusedConnCount       *metric.Counter
	SuccessfulConnCount    *metric.Counter
	AuthFailedCount        *metric.Counter
	ExpiredClientConnCount *metric.Counter

	ConnMigrationSuccessCount                *metric.Counter
	ConnMigrationErrorFatalCount             *metric.Counter
	ConnMigrationErrorRecoverableCount       *metric.Counter
	ConnMigrationAttemptedCount              *metric.Counter
	ConnMigrationAttemptedLatency            *metric.Histogram
	ConnMigrationTransferResponseMessageSize *metric.Histogram
}

// MetricStruct implements the metrics.Struct interface.
func (metrics) MetricStruct() {}

var _ metric.Struct = metrics{}

const (
	// maxExpectedTransferResponseMessageSize corresponds to maximum expected
	// response message size for the SHOW TRANSFER STATE query. We choose 16MB
	// here to match the defaultMaxReadBufferSize used for ingesting SQL
	// statements in the SQL server (see pkg/sql/pgwire/pgwirebase/encoding.go).
	//
	// This will be used to tune sql.session_transfer.max_session_size.
	maxExpectedTransferResponseMessageSize = 1 << 24 // 16MB
)

var (
	metaCurConnCount = metric.Metadata{
		Name:        "proxy.sql.conns",
		Help:        "Number of connections being proxied",
		Measurement: "Connections",
		Unit:        metric.Unit_COUNT,
	}
	metaRoutingErrCount = metric.Metadata{
		Name:        "proxy.err.routing",
		Help:        "Number of errors encountered when attempting to route clients",
		Measurement: "Errors",
		Unit:        metric.Unit_COUNT,
	}
	metaBackendDownCount = metric.Metadata{
		Name:        "proxy.err.backend_down",
		Help:        "Number of errors encountered when connecting to backend servers",
		Measurement: "Errors",
		Unit:        metric.Unit_COUNT,
	}
	metaBackendDisconnectCount = metric.Metadata{
		Name:        "proxy.err.backend_disconnect",
		Help:        "Number of disconnects initiated by proxied backends",
		Measurement: "Disconnects",
		Unit:        metric.Unit_COUNT,
	}
	metaIdleDisconnectCount = metric.Metadata{
		Name:        "proxy.err.idle_disconnect",
		Help:        "Number of disconnects due to idle timeout",
		Measurement: "Idle Disconnects",
		Unit:        metric.Unit_COUNT,
	}
	metaClientDisconnectCount = metric.Metadata{
		Name:        "proxy.err.client_disconnect",
		Help:        "Number of disconnects initiated by clients",
		Measurement: "Client Disconnects",
		Unit:        metric.Unit_COUNT,
	}
	metaRefusedConnCount = metric.Metadata{
		Name:        "proxy.err.refused_conn",
		Help:        "Number of refused connections initiated by a given IP",
		Measurement: "Refused",
		Unit:        metric.Unit_COUNT,
	}
	metaSuccessfulConnCount = metric.Metadata{
		Name:        "proxy.sql.successful_conns",
		Help:        "Number of successful connections that were/are being proxied",
		Measurement: "Successful Connections",
		Unit:        metric.Unit_COUNT,
	}
	metaAuthFailedCount = metric.Metadata{
		Name:        "proxy.sql.authentication_failures",
		Help:        "Number of authentication failures",
		Measurement: "Authentication Failures",
		Unit:        metric.Unit_COUNT,
	}
	metaExpiredClientConnCount = metric.Metadata{
		Name:        "proxy.sql.expired_client_conns",
		Help:        "Number of expired client connections",
		Measurement: "Expired Client Connections",
		Unit:        metric.Unit_COUNT,
	}
	// Connection migration metrics.
	//
	// attempted = success + error_fatal + error_recoverable
	metaConnMigrationSuccessCount = metric.Metadata{
		Name:        "proxy.conn_migration.success",
		Help:        "Number of successful connection migrations",
		Measurement: "Connection Migrations",
		Unit:        metric.Unit_COUNT,
	}
	metaConnMigrationErrorFatalCount = metric.Metadata{
		// When connection migrations errored out, connections will be closed.
		Name:        "proxy.conn_migration.error_fatal",
		Help:        "Number of failed connection migrations which resulted in terminations",
		Measurement: "Connection Migrations",
		Unit:        metric.Unit_COUNT,
	}
	metaConnMigrationErrorRecoverableCount = metric.Metadata{
		// Connections are recoverable, so they won't be closed.
		Name:        "proxy.conn_migration.error_recoverable",
		Help:        "Number of failed connection migrations that were recoverable",
		Measurement: "Connection Migrations",
		Unit:        metric.Unit_COUNT,
	}
	metaConnMigrationAttemptedCount = metric.Metadata{
		Name:        "proxy.conn_migration.attempted",
		Help:        "Number of attempted connection migrations",
		Measurement: "Connection Migrations",
		Unit:        metric.Unit_COUNT,
	}
	metaConnMigrationAttemptedLatency = metric.Metadata{
		Name:        "proxy.conn_migration.attempted.latency",
		Help:        "Latency histogram for attempted connection migrations",
		Measurement: "Latency",
		Unit:        metric.Unit_NANOSECONDS,
	}
	metaConnMigrationTransferResponseMessageSize = metric.Metadata{
		Name:        "proxy.conn_migration.transfer_response.message_size",
		Help:        "Message size for the SHOW TRANSFER STATE response",
		Measurement: "Bytes",
		Unit:        metric.Unit_BYTES,
	}
)

// makeProxyMetrics instantiates the metrics holder for proxy monitoring.
func makeProxyMetrics() metrics {
	return metrics{
		BackendDisconnectCount: metric.NewCounter(metaBackendDisconnectCount),
		IdleDisconnectCount:    metric.NewCounter(metaIdleDisconnectCount),
		BackendDownCount:       metric.NewCounter(metaBackendDownCount),
		ClientDisconnectCount:  metric.NewCounter(metaClientDisconnectCount),
		CurConnCount:           metric.NewGauge(metaCurConnCount),
		RoutingErrCount:        metric.NewCounter(metaRoutingErrCount),
		RefusedConnCount:       metric.NewCounter(metaRefusedConnCount),
		SuccessfulConnCount:    metric.NewCounter(metaSuccessfulConnCount),
		AuthFailedCount:        metric.NewCounter(metaAuthFailedCount),
		ExpiredClientConnCount: metric.NewCounter(metaExpiredClientConnCount),
		// Connection migration metrics.
		ConnMigrationSuccessCount:          metric.NewCounter(metaConnMigrationSuccessCount),
		ConnMigrationErrorFatalCount:       metric.NewCounter(metaConnMigrationErrorFatalCount),
		ConnMigrationErrorRecoverableCount: metric.NewCounter(metaConnMigrationErrorRecoverableCount),
		ConnMigrationAttemptedCount:        metric.NewCounter(metaConnMigrationAttemptedCount),
		ConnMigrationAttemptedLatency: metric.NewLatency(
			metaConnMigrationAttemptedLatency,
			base.DefaultHistogramWindowInterval(),
		),
		ConnMigrationTransferResponseMessageSize: metric.NewHistogram(
			metaConnMigrationTransferResponseMessageSize,
			base.DefaultHistogramWindowInterval(),
			maxExpectedTransferResponseMessageSize,
			1,
		),
	}
}

// updateForError updates the metrics relevant for the type of the error
// message.
func (metrics *metrics) updateForError(err error) {
	if err == nil {
		return
	}
	codeErr := (*codeError)(nil)
	if errors.As(err, &codeErr) {
		switch codeErr.code {
		case codeExpiredClientConnection:
			metrics.ExpiredClientConnCount.Inc(1)
		case codeBackendDisconnected:
			metrics.BackendDisconnectCount.Inc(1)
		case codeClientDisconnected:
			metrics.ClientDisconnectCount.Inc(1)
		case codeIdleDisconnect:
			metrics.IdleDisconnectCount.Inc(1)
		case codeProxyRefusedConnection:
			metrics.RefusedConnCount.Inc(1)
			metrics.BackendDownCount.Inc(1)
		case codeParamsRoutingFailed, codeUnavailable:
			metrics.RoutingErrCount.Inc(1)
			metrics.BackendDownCount.Inc(1)
		case codeBackendDown:
			metrics.BackendDownCount.Inc(1)
		case codeAuthFailed:
			metrics.AuthFailedCount.Inc(1)
		}
	}
}
