package httpapi

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type channelHealth struct {
	Provider      string     `json:"provider"`
	Channel       string     `json:"channel"`
	State         string     `json:"state" enum:"healthy,throttled,rate_limited,credentials_refused,circuit_open,probing"`
	Until         *time.Time `json:"until,omitempty"`
	RatePerSecond int        `json:"rate_per_second"`
	Simulate      string     `json:"simulate,omitempty"`
	SimulateUntil *time.Time `json:"simulate_until,omitempty"`
	Calls         int        `json:"calls_last_10m"`
	Accepted      int        `json:"accepted_last_10m"`
	Failed        int        `json:"failed_last_10m"`
	Held          int        `json:"held_last_10m"`
	LastProblem   *struct {
		Code     int       `json:"code"`
		Response string    `json:"response"`
		At       time.Time `json:"at"`
	} `json:"last_problem,omitempty"`
}

type channelSettings struct {
	RatePerSecond   int    `json:"rate_per_second" minimum:"1" maximum:"1000"`
	Simulate        string `json:"simulate" enum:",throttle,outage,credentials"`
	SimulateMinutes int    `json:"simulate_minutes,omitempty" minimum:"0" maximum:"30"`
}

type notificationList struct {
	Unread int `json:"unread"`
	Items  []struct {
		ID        string    `json:"id"`
		Kind      string    `json:"kind"`
		Severity  string    `json:"severity"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		JourneyID *string   `json:"journey_id,omitempty"`
		CreatedAt time.Time `json:"created_at"`
		Unread    bool      `json:"unread"`
	} `json:"items"`
}

// The providers a workspace sends through, in the order they are shown.
const channelHealthQuery = `WITH providers(provider,channel,position) AS (VALUES ('smtp','email',1),('mock_sms','sms',2))
SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
  'provider',p.provider,'channel',p.channel,
  'state',CASE WHEN c.credentials_refused_at IS NOT NULL THEN 'credentials_refused'
               WHEN c.breaker='open' THEN 'circuit_open'
               WHEN c.breaker='probing' THEN 'probing'
               WHEN c.paused_until>now() AND c.pause_reason='throttled' THEN 'throttled'
               WHEN c.paused_until>now() THEN 'rate_limited'
               ELSE 'healthy' END,
  'until',CASE WHEN c.credentials_refused_at IS NOT NULL THEN c.next_probe_at WHEN c.breaker='open' THEN c.breaker_until WHEN c.paused_until>now() THEN c.paused_until END,
  'rate_per_second',COALESCE(c.rate_per_second,50),
  'simulate',CASE WHEN c.simulate_until>now() THEN c.simulate END,
  'simulate_until',CASE WHEN c.simulate_until>now() THEN c.simulate_until END,
  'calls_last_10m',COALESCE(a.calls,0),'accepted_last_10m',COALESCE(a.accepted,0),'failed_last_10m',COALESCE(a.failed,0),'held_last_10m',COALESCE(a.held,0),
  'last_problem',a.problem)) ORDER BY p.position),'[]'::jsonb)
FROM providers p
LEFT JOIN provider_channels c ON c.workspace_id=$1 AND c.provider=p.provider
LEFT JOIN LATERAL (
  SELECT count(*) FILTER (WHERE status<>'CANCELLED') calls,
         count(*) FILTER (WHERE status='ACCEPTED') accepted,
         count(*) FILTER (WHERE status IN ('RETRY','UNKNOWN','FAILED') AND signal IS NULL) failed,
         count(*) FILTER (WHERE signal IS NOT NULL) held,
         (SELECT jsonb_build_object('code',COALESCE(x.provider_code,0),'response',COALESCE(x.response,''),'at',x.completed_at) FROM delivery_attempts x
           WHERE x.workspace_id=$1 AND x.provider=p.provider AND x.started_at>now()-interval '10 minutes' AND x.status IN ('RETRY','UNKNOWN','FAILED')
           ORDER BY x.started_at DESC LIMIT 1) problem
  FROM delivery_attempts WHERE workspace_id=$1 AND provider=p.provider AND started_at>now()-interval '10 minutes'
) a ON true`

func registerChannels(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/delivery/channels", func(ctx context.Context, _ *struct{}) (*Result[[]channelHealth], error) {
		return jsonRow[[]channelHealth](pool.QueryRow(ctx, channelHealthQuery, who(ctx).WorkspaceID))
	})
	huma.Put(api, "/api/v1/delivery/channels/{provider}", func(ctx context.Context, in *struct {
		Provider string `path:"provider" enum:"smtp,mock_sms"`
		Body     channelSettings
	}) (*Result[[]channelHealth], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		var simulate *string
		var until *time.Time
		if in.Body.Simulate != "" {
			if in.Body.SimulateMinutes < 1 {
				return nil, huma.Error422UnprocessableEntity("Choose how many minutes to simulate the problem")
			}
			at := time.Now().Add(time.Duration(in.Body.SimulateMinutes) * time.Minute)
			simulate, until = &in.Body.Simulate, &at
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		workspace := who(ctx).WorkspaceID
		if _, err = tx.Exec(ctx, `INSERT INTO provider_channels(workspace_id,provider,rate_per_second,simulate,simulate_until) VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(workspace_id,provider) DO UPDATE SET rate_per_second=$3,simulate=$4,simulate_until=$5,updated_at=now()`, workspace, in.Provider, in.Body.RatePerSecond, simulate, until); err != nil {
			return nil, dbError(err)
		}
		if err = audit(ctx, tx, "delivery.channel_settings", &workspace, map[string]any{"provider": in.Provider, "rate_per_second": in.Body.RatePerSecond, "simulate": in.Body.Simulate, "simulate_minutes": in.Body.SimulateMinutes}); err != nil {
			return nil, dbError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return jsonRow[[]channelHealth](pool.QueryRow(ctx, channelHealthQuery, workspace))
	})
	huma.Get(api, "/api/v1/notifications", func(ctx context.Context, in *struct {
		Limit int `query:"limit" default:"30" minimum:"1" maximum:"100"`
	}) (*Result[notificationList], error) {
		me := who(ctx)
		return jsonRow[notificationList](pool.QueryRow(ctx, `WITH me AS (SELECT notifications_seen_at seen FROM users WHERE id=$2 AND workspace_id=$1),
			recent AS (SELECT n.*,(n.created_at>(SELECT seen FROM me) AND n.actor_id IS DISTINCT FROM $2::uuid) unread FROM notifications n WHERE n.workspace_id=$1 ORDER BY n.created_at DESC,n.id DESC LIMIT $3)
			SELECT jsonb_build_object(
			  'unread',(SELECT count(*) FROM notifications n WHERE n.workspace_id=$1 AND n.created_at>(SELECT seen FROM me) AND n.actor_id IS DISTINCT FROM $2::uuid),
			  'items',COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object('id',id,'kind',kind,'severity',severity,'title',title,'body',body,'journey_id',journey_id,'created_at',created_at,'unread',unread)) ORDER BY created_at DESC,id DESC) FROM recent),'[]'::jsonb))`,
			me.WorkspaceID, me.ID, in.Limit))
	})
	huma.Post(api, "/api/v1/notifications/read", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		me := who(ctx)
		if _, err := pool.Exec(ctx, `UPDATE users SET notifications_seen_at=GREATEST(notifications_seen_at,now()) WHERE id=$1 AND workspace_id=$2`, me.ID, me.WorkspaceID); err != nil {
			return nil, dbError(err)
		}
		return &struct{}{}, nil
	})
}
