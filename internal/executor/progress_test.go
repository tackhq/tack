package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tackhq/tack/internal/connector"
)

func TestHostActivity_Label(t *testing.T) {
	a := newHostActivity([]string{"web1", "web2", "web3"})
	assert.Equal(t, "applying 0/3 hosts", a.label("applying"))

	a.set("web2", "Install nginx", "SSM InProgress (12s)")
	a.set("web3", "", "connecting to web3")
	a.finish("web1")
	assert.Equal(t, "applying 1/3 hosts · web2: Install nginx · SSM InProgress (12s) (+1 more)", a.label("applying"))

	a.finish("web2")
	assert.Equal(t, "applying 2/3 hosts · web3: connecting to web3", a.label("applying"))
}

type detailRecorder struct {
	nullEmitter
	details []string
}

func (d *detailRecorder) TaskDetail(s string) { d.details = append(d.details, s) }

func TestWithTaskProgress_NoLogHidesCommands(t *testing.T) {
	for _, noLog := range []bool{false, true} {
		rec := &detailRecorder{}
		var hooked []string
		pctx := &PlayContext{Output: rec, OnStatus: func(task, detail string) { hooked = append(hooked, task+"|"+detail) }}

		ctx := withTaskProgress(context.Background(), pctx, "Set password", noLog)
		connector.ReportCommand(ctx, "usermod -p secret app")
		connector.ReportProgress(ctx, "SSM InProgress (2s)")

		if noLog {
			assert.Equal(t, []string{"SSM InProgress (2s)"}, rec.details)
		} else {
			assert.Equal(t, []string{"running: usermod -p secret app", "SSM InProgress (2s)"}, rec.details)
		}
		assert.Equal(t, "Set password|", hooked[0], "task start is reported to the hook")
	}
}
