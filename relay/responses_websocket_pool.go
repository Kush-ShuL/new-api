package relay

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/wsmanager"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var responsesConnectionPool = struct {
	sync.Mutex
	entries map[[32]byte][]*responsesPooledConnection
	count   int
}{entries: make(map[[32]byte][]*responsesPooledConnection)}

// A pooled socket has one permanent reader and one exclusive lease. A lease
// owns its event queue; releasing it cannot leave a reader on the next request.
type responsesPooledConnection struct {
	key            [32]byte
	conn           *websocket.Conn
	lease          *responsesConnectionLease
	lastResponseID string
	created        time.Time
	idleTimer      *time.Timer
	unregister     func()
	closed         bool
}

type responsesConnectionLease struct {
	entry         *responsesPooledConnection
	events        chan responsesWSMessage
	done          chan struct{}
	once          sync.Once
	reusedContext bool
}

func acquireResponsesConnection(c *gin.Context, adaptor relaychannel.Adaptor, info *relaycommon.RelayInfo, previousID string) (*responsesConnectionLease, error) {
	url, headers, err := relaychannel.PrepareWebSocketHandshake(adaptor, c, info)
	if err != nil {
		return nil, err
	}
	identity, err := common.Marshal(struct {
		UserID, TokenID, ChannelID int
		Group, Model, URL          string
		Headers                    http.Header
		Settings                   dto.ChannelSettings
	}{common.GetContextKeyInt(c, constant.ContextKeyUserId), common.GetContextKeyInt(c, constant.ContextKeyTokenId), info.ChannelId, info.UsingGroup, info.UpstreamModelName, url, headers, info.ChannelSetting})
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(identity)
	pool := &responsesConnectionPool
	pool.Lock()
	var selected *responsesPooledConnection
	for _, entry := range pool.entries[key] {
		if entry.closed || entry.lease != nil || time.Since(entry.created) > 20*time.Minute {
			continue
		}
		if selected == nil || (previousID != "" && entry.lastResponseID == previousID) {
			selected = entry
		}
		if previousID != "" && entry.lastResponseID == previousID {
			break
		}
	}
	if selected == nil {
		if pool.count >= 128 {
			pool.Unlock()
			return nil, errors.New("Responses connection pool is full")
		}
		pool.count++
		pool.Unlock()
		conn, err := relaychannel.DialWebSocket(c, info, url, headers)
		pool.Lock()
		if err != nil {
			pool.count--
			pool.Unlock()
			return nil, err
		}
		selected = &responsesPooledConnection{key: key, conn: conn, created: time.Now()}
		pool.entries[key] = append(pool.entries[key], selected)
		selected.unregister = wsmanager.Register(info.ChannelId, wsmanager.KindResponses, func(string) { selected.close() })
		limit := int64(helper.DefaultMaxScannerBufferSize)
		if constant.StreamScannerMaxBufferMB > 0 {
			limit = int64(constant.StreamScannerMaxBufferMB) << 20
		}
		conn.SetReadLimit(limit)
		go selected.read()
	}
	if selected.idleTimer != nil {
		selected.idleTimer.Stop()
	}
	lease := &responsesConnectionLease{entry: selected, events: make(chan responsesWSMessage, 1), done: make(chan struct{}), reusedContext: previousID != "" && selected.lastResponseID == previousID}
	selected.lease = lease
	pool.Unlock()
	return lease, nil
}

func (entry *responsesPooledConnection) read() {
	defer entry.close()
	for {
		kind, body, err := entry.conn.ReadMessage()
		responsesConnectionPool.Lock()
		lease := entry.lease
		closed := entry.closed
		responsesConnectionPool.Unlock()
		if closed || lease == nil {
			return
		}
		select {
		case lease.events <- responsesWSMessage{target: entry.conn, kind: kind, body: body, err: err}:
		case <-lease.done:
			return
		}
		if err != nil {
			return
		}
	}
}

func (lease *responsesConnectionLease) release(keep bool, responseID string) {
	lease.once.Do(func() {
		pool := &responsesConnectionPool
		pool.Lock()
		entry := lease.entry
		close(lease.done)
		entry.lease = nil
		if keep && len(lease.events) == 0 && !entry.closed && time.Since(entry.created) < 20*time.Minute {
			entry.lastResponseID = responseID
			entry.idleTimer = time.AfterFunc(2*time.Minute, func() { entry.closeIfIdle(true) })
			pool.Unlock()
			return
		}
		pool.Unlock()
		entry.close()
	})
}

func (entry *responsesPooledConnection) close() {
	entry.closeIfIdle(false)
}

func (entry *responsesPooledConnection) closeIfIdle(onlyIdle bool) {
	pool := &responsesConnectionPool
	pool.Lock()
	if entry.closed || (onlyIdle && entry.lease != nil) {
		pool.Unlock()
		return
	}
	entry.closed = true
	if entry.idleTimer != nil {
		entry.idleTimer.Stop()
	}
	entries := pool.entries[entry.key]
	for i, candidate := range entries {
		if candidate == entry {
			entries = append(entries[:i], entries[i+1:]...)
			break
		}
	}
	if len(entries) == 0 {
		delete(pool.entries, entry.key)
	} else {
		pool.entries[entry.key] = entries
	}
	pool.count--
	pool.Unlock()
	if entry.unregister != nil {
		entry.unregister()
	}
	_ = entry.conn.Close()
}

func (s *responsesWSSession) finishPooledResponse(keep bool, responseID string) {
	s.connectionMu.Lock()
	lease := s.poolLease
	if lease == nil {
		s.connectionMu.Unlock()
		return
	}
	unregister := s.unregister
	s.poolLease, s.target, s.unregister = nil, nil, nil
	s.connectionMu.Unlock()
	if unregister != nil {
		unregister()
	}
	lease.release(keep, responseID)
}
