package messaging

import "sync/atomic"

type Metrics struct{ Published, Failed, Backlog, LagSeconds atomic.Int64 }

func (m *Metrics) Snapshot() (published, failed, backlog, lagSeconds int64) {
	return m.Published.Load(), m.Failed.Load(), m.Backlog.Load(), m.LagSeconds.Load()
}
