package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/trigger/timer"
	"github.com/stretchr/testify/require"
)

type timerOptionsProvider struct {
	def *model.TimerDefinition
}

func (p *timerOptionsProvider) LoadTimers(context.Context, model.UserScope) ([]*model.TimerDefinition, error) {
	return []*model.TimerDefinition{p.def}, nil
}
func (p *timerOptionsProvider) LoadAllTimers(context.Context) ([]*model.TimerDefinition, error) {
	return []*model.TimerDefinition{p.def}, nil
}
func (p *timerOptionsProvider) LoadTimerForUser(context.Context, model.UserScope, int64) (*model.TimerDefinition, error) {
	if p.def == nil {
		return nil, errors.New("not found")
	}
	return p.def, nil
}
func (p *timerOptionsProvider) SaveTimerForUser(_ context.Context, scope model.UserScope, def *model.TimerDefinition) error {
	def.Id, def.UserName = 991, scope.User
	p.def = def
	return nil
}
func (p *timerOptionsProvider) RemoveTimerForUser(context.Context, model.UserScope, int64) error {
	p.def = nil
	return nil
}
func (p *timerOptionsProvider) SetTimerRuntimeError(context.Context, int64, string) error {
	return nil
}

type timerOptionsLoader struct{}

func (timerOptionsLoader) Load(string) (*tql.Script, error) { return nil, nil }

func TestTimerRPCContinueOnError(t *testing.T) {
	provider := &timerOptionsProvider{}
	s := &Server{timerSvc: timer.NewService(timer.WithProvider(provider), timer.WithTqlLoader(timerOptionsLoader{}))}
	ctx := contextWithModelUser(context.Background(), "sys")
	var add addTimerRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"timer","spec":"@every 1m","command":"task.tql","continueOnError":true}`), &add))
	id, err := s.addTimer(ctx, add)
	require.NoError(t, err)
	t.Cleanup(func() { timer.Unregister(id) })
	info, err := s.getTimer(ctx, id)
	require.NoError(t, err)
	require.True(t, info.ContinueOnError)

	var oldRequest updateTimerRequest
	require.NoError(t, json.Unmarshal([]byte(`{"id":991,"spec":"@every 1m","command":"task.tql"}`), &oldRequest))
	require.NoError(t, s.updateTimer(ctx, oldRequest))
	require.True(t, provider.def.ContinueOnError)

	var disable updateTimerRequest
	require.NoError(t, json.Unmarshal([]byte(`{"id":991,"spec":"@every 1m","command":"task.tql","continueOnError":false}`), &disable))
	require.NoError(t, s.updateTimer(ctx, disable))
	require.False(t, provider.def.ContinueOnError)
}
