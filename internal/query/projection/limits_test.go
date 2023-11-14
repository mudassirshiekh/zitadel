package projection

import (
	"testing"
	"time"

	"github.com/zitadel/zitadel/internal/errors"
	"github.com/zitadel/zitadel/internal/eventstore"
	"github.com/zitadel/zitadel/internal/eventstore/handler/v2"
	"github.com/zitadel/zitadel/internal/repository/limits"
)

func TestLimitsProjection_reduces(t *testing.T) {
	type args struct {
		event func(t *testing.T) eventstore.Event
	}
	tests := []struct {
		name   string
		args   args
		reduce func(event eventstore.Event) (*handler.Statement, error)
		want   wantReduce
	}{
		{
			name: "reduceLimitsSet",
			args: args{
				event: getEvent(testEvent(
					limits.SetEventType,
					limits.AggregateType,
					[]byte(`{
							"auditLogRetention": 300000000000
					}`),
				), limits.SetEventMapper),
			},
			reduce: (&limitsProjection{}).reduceLimitsSet,
			want: wantReduce{
				aggregateType: eventstore.AggregateType("limits"),
				sequence:      15,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "INSERT INTO projections.limits (instance_id, resource_owner, creation_date, change_date, sequence, aggregate_id, audit_log_retention) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (instance_id, resource_owner) DO UPDATE SET (creation_date, change_date, sequence, aggregate_id, audit_log_retention) = (EXCLUDED.creation_date, EXCLUDED.change_date, EXCLUDED.sequence, EXCLUDED.aggregate_id, EXCLUDED.audit_log_retention)",
							expectedArgs: []interface{}{
								"instance-id",
								"ro-id",
								anyArg{},
								anyArg{},
								uint64(15),
								"agg-id",
								time.Minute * 5,
							},
						},
					},
				},
			},
		},

		{
			name: "reduceLimitsReset",
			args: args{
				event: getEvent(testEvent(
					limits.ResetEventType,
					limits.AggregateType,
					[]byte(`{}`),
				), limits.ResetEventMapper),
			},
			reduce: (&limitsProjection{}).reduceLimitsReset,
			want: wantReduce{
				aggregateType: eventstore.AggregateType("limits"),
				sequence:      15,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "DELETE FROM projections.limits WHERE (instance_id = $1) AND (resource_owner = $2)",
							expectedArgs: []interface{}{
								"instance-id",
								"ro-id",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := baseEvent(t)
			got, err := tt.reduce(event)
			if !errors.IsErrorInvalidArgument(err) {
				t.Errorf("no wrong event mapping: %v, got: %v", err, got)
			}
			event = tt.args.event(t)
			got, err = tt.reduce(event)
			assertReduce(t, got, err, LimitsProjectionTable, tt.want)
		})
	}
}