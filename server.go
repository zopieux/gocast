package gocast

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/zopieux/gocast/constants"
	"github.com/zopieux/gocast/framing"
	castproto "github.com/zopieux/gocast/proto"
	casttls "github.com/zopieux/gocast/tls"
)

type clientConn struct {
	conn    net.Conn
	writeMu sync.Mutex
}

func (c *clientConn) send(sourceID, destID, namespace string, payload any) error {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	payloadStr := string(jsonBytes)
	protoVersion := castproto.CastMessage_CASTV2_1_0
	payloadType := castproto.CastMessage_STRING

	msg := &castproto.CastMessage{
		ProtocolVersion: &protoVersion,
		SourceId:        &sourceID,
		DestinationId:   &destID,
		Namespace:       &namespace,
		PayloadType:     &payloadType,
		PayloadUtf8:     &payloadStr,
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return framing.WriteMessage(c.conn, msg)
}

// Server implements a CastV2 TLS receiver for Home Assistant.
type Server struct {
	cfg       Config
	handlers  CommandHandlers
	listener  net.Listener
	clientsMu sync.RWMutex
	clients   map[*clientConn]struct{}

	stateMu     sync.RWMutex
	mediaStatus MediaStatus
	volume      float64 // 0.0 to 1.0
	muted       bool
}

// NewServer initializes a new CastV2 receiver instance.
func NewServer(cfg Config, handlers CommandHandlers) *Server {
	return &Server{
		cfg:      cfg,
		handlers: handlers,
		clients:  make(map[*clientConn]struct{}),
		volume:   1.0,
	}
}

// Start begins listening on the configured TLS port.
func StartServer(s *Server) error {
	return s.Start()
}

func (s *Server) Start() error {
	cert, err := casttls.GenerateEphemeralCertificate(s.cfg.FriendlyName)
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to listen TLS on %s: %w", addr, err)
	}
	s.listener = ln
	slog.Info("CastV2 TLS server listening", "address", addr)

	go s.acceptLoop()
	go s.heartbeatLoop()
	return nil
}

// Close terminates the server listener and active connections.
func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		c := &clientConn{conn: conn}
		s.clientsMu.Lock()
		s.clients[c] = struct{}{}
		s.clientsMu.Unlock()

		slog.Debug("client connected", "remote_addr", conn.RemoteAddr().String())
		go s.handleClient(c)
	}
}

func (s *Server) removeClient(c *clientConn) {
	s.clientsMu.Lock()
	delete(s.clients, c)
	s.clientsMu.Unlock()
	c.conn.Close()
	slog.Debug("client disconnected", "remote_addr", c.conn.RemoteAddr().String())
}

func (s *Server) heartbeatLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.clientsMu.RLock()
		for c := range s.clients {
			go func(conn *clientConn) {
				_ = conn.send(constants.PlatformReceiverID, constants.WildcardDestinationID, constants.NsHeartbeat, map[string]any{
					"type": constants.TypePing,
				})
			}(c)
		}
		s.clientsMu.RUnlock()
	}
}

func (s *Server) handleClient(c *clientConn) {
	defer s.removeClient(c)

	for {
		msg, err := framing.ReadMessage(c.conn)
		if err != nil {
			return
		}

		namespace := msg.GetNamespace()
		sourceID := msg.GetSourceId()
		payload := msg.GetPayloadUtf8()

		slog.Debug("received cast message", "ns", namespace, "src", sourceID, "payload", payload)
		s.dispatchMessage(c, sourceID, namespace, payload)
	}
}

func (s *Server) dispatchMessage(c *clientConn, sourceID, namespace, payloadStr string) {
	var genericReq struct {
		Type      string `json:"type"`
		RequestID int    `json:"requestId"`
	}
	if err := json.Unmarshal([]byte(payloadStr), &genericReq); err != nil {
		return
	}

	switch namespace {
	case constants.NsConnection:
		// Connection established or closed. No explicit reply required.

	case constants.NsHeartbeat:
		if genericReq.Type == constants.TypePing {
			resp := map[string]any{"type": constants.TypePong}
			if genericReq.RequestID != 0 {
				resp["requestId"] = genericReq.RequestID
			}
			_ = c.send(constants.PlatformReceiverID, sourceID, constants.NsHeartbeat, resp)
		}

	case constants.NsReceiver:
		s.handleReceiverMessage(c, sourceID, genericReq.Type, genericReq.RequestID, payloadStr)

	case constants.NsMedia:
		s.handleMediaMessage(c, sourceID, genericReq.Type, genericReq.RequestID, payloadStr)
	}
}

func (s *Server) handleReceiverMessage(c *clientConn, sourceID, msgType string, requestID int, payloadStr string) {
	switch msgType {
	case constants.TypeGetStatus:
		_ = c.send(constants.PlatformReceiverID, sourceID, constants.NsReceiver, s.buildReceiverStatus(requestID))

	case constants.TypeSetVolume:
		var req struct {
			Volume struct {
				Level *float64 `json:"level"`
				Muted *bool    `json:"muted"`
			} `json:"volume"`
		}
		if err := json.Unmarshal([]byte(payloadStr), &req); err == nil {
			if req.Volume.Level != nil && s.handlers.OnSetVolume != nil {
				pct := int(*req.Volume.Level * 100.0)
				s.handlers.OnSetVolume(pct)
			}
			if req.Volume.Muted != nil && s.handlers.OnSetMute != nil {
				s.handlers.OnSetMute(*req.Volume.Muted)
			}
		}
		_ = c.send(constants.PlatformReceiverID, sourceID, constants.NsReceiver, s.buildReceiverStatus(requestID))
		s.broadcastReceiverStatus(0)

	case constants.TypeStop:
		if s.handlers.OnStop != nil {
			s.handlers.OnStop()
		}
		_ = c.send(constants.PlatformReceiverID, sourceID, constants.NsReceiver, s.buildReceiverStatus(requestID))
		s.broadcastReceiverStatus(0)

	case "LAUNCH":
		_ = c.send(constants.PlatformReceiverID, sourceID, constants.NsReceiver, s.buildReceiverStatus(requestID))
		s.broadcastReceiverStatus(0)
	}
}

func (s *Server) handleMediaMessage(c *clientConn, sourceID, msgType string, requestID int, payloadStr string) {
	switch msgType {
	case constants.TypeGetStatus:
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, s.buildMediaStatus(requestID))

	case constants.TypePlay:
		if s.handlers.OnPlay != nil {
			s.handlers.OnPlay()
		}
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, s.buildMediaStatus(requestID))
		s.broadcastMediaStatus(0)

	case constants.TypePause:
		if s.handlers.OnPause != nil {
			s.handlers.OnPause()
		}
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, s.buildMediaStatus(requestID))
		s.broadcastMediaStatus(0)

	case constants.TypeStop:
		if s.handlers.OnStop != nil {
			s.handlers.OnStop()
		}
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, s.buildMediaStatus(requestID))
		s.broadcastMediaStatus(0)

	case constants.TypeQueueUpdate:
		var req struct {
			Jump int `json:"jump"`
		}
		if err := json.Unmarshal([]byte(payloadStr), &req); err == nil {
			if req.Jump > 0 && s.handlers.OnNext != nil {
				s.handlers.OnNext()
			} else if req.Jump < 0 && s.handlers.OnPrevious != nil {
				s.handlers.OnPrevious()
			}
		}
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, s.buildMediaStatus(requestID))
		s.broadcastMediaStatus(0)

	case constants.TypeSeek:
		var req struct {
			CurrentTime float64 `json:"currentTime"`
		}
		if err := json.Unmarshal([]byte(payloadStr), &req); err == nil {
			s.stateMu.Lock()
			s.mediaStatus.Position = req.CurrentTime
			s.stateMu.Unlock()
			if s.handlers.OnSeek != nil {
				s.handlers.OnSeek(req.CurrentTime)
			}
		}
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, s.buildMediaStatus(requestID))
		s.broadcastMediaStatus(0)

	case constants.TypeLoad:
		_ = c.send(constants.AppTransportID, sourceID, constants.NsMedia, map[string]any{
			"type":      constants.TypeLoadFailed,
			"requestId": requestID,
		})
	}
}

func (s *Server) buildReceiverStatus(requestID int) map[string]any {
	s.stateMu.RLock()
	vol := s.volume
	muted := s.muted
	s.stateMu.RUnlock()

	return map[string]any{
		"type":      constants.TypeReceiverStatus,
		"requestId": requestID,
		"status": map[string]any{
			"volume": map[string]any{
				"level":        vol,
				"muted":        muted,
				"controlType":  "master",
				"stepInterval": 0.05,
			},
			"applications": []map[string]any{
				{
					"appId":       constants.DefaultMediaAppID,
					"displayName": "Default Media Receiver",
					"namespaces": []map[string]string{
						{"name": constants.NsMedia},
					},
					"sessionId":   constants.AppSessionID,
					"statusText":  "Default Media Receiver",
					"transportId": constants.AppTransportID,
				},
			},
		},
	}
}

func (s *Server) buildMediaStatus(requestID int) map[string]any {
	s.stateMu.RLock()
	st := s.mediaStatus
	vol := s.volume
	muted := s.muted
	s.stateMu.RUnlock()

	castPlayerState := "IDLE"
	switch strings.ToUpper(st.PlayerState) {
	case "PLAYING":
		castPlayerState = "PLAYING"
	case "PAUSED":
		castPlayerState = "PAUSED"
	case "BUFFERING":
		castPlayerState = "BUFFERING"
	}

	mediaObj := map[string]any{
		"streamType":  "BUFFERED",
		"contentType": "audio/mp3",
		"metadata": map[string]any{
			"metadataType": constants.MetadataTypeMusic,
			"title":        st.Title,
			"artist":       st.Artist,
		},
	}
	if st.Album != "" {
		mediaObj["metadata"].(map[string]any)["albumName"] = st.Album
	}
	if st.Duration > 0 {
		mediaObj["duration"] = st.Duration
	}

	return map[string]any{
		"type":      constants.TypeMediaStatus,
		"requestId": requestID,
		"status": []map[string]any{
			{
				"mediaSessionId":         constants.MediaSessionID,
				"playbackRate":           1.0,
				"playerState":            castPlayerState,
				"currentTime":            st.Position,
				"supportedMediaCommands": constants.SupportedCommands,
				"volume": map[string]any{
					"level": vol,
					"muted": muted,
				},
				"media": mediaObj,
			},
		},
	}
}

func (s *Server) broadcastReceiverStatus(requestID int) {
	msg := s.buildReceiverStatus(requestID)
	s.broadcast(constants.PlatformReceiverID, constants.NsReceiver, msg)
}

func (s *Server) broadcastMediaStatus(requestID int) {
	msg := s.buildMediaStatus(requestID)
	s.broadcast(constants.AppTransportID, constants.NsMedia, msg)
}

func (s *Server) broadcast(sourceID, namespace string, payload any) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for c := range s.clients {
		go func(conn *clientConn) {
			_ = conn.send(sourceID, constants.WildcardDestinationID, namespace, payload)
		}(c)
	}
}

// UpdateMedia stores updated media state and broadcasts updated MEDIA_STATUS.
func (s *Server) UpdateMedia(status MediaStatus) {
	s.stateMu.Lock()
	s.mediaStatus = status
	s.stateMu.Unlock()

	s.broadcastMediaStatus(0)
}

// UpdateVolume stores volume and mute state and broadcasts updated RECEIVER_STATUS.
func (s *Server) UpdateVolume(volPct int, muted bool) {
	s.stateMu.Lock()
	s.volume = float64(volPct) / 100.0
	s.muted = muted
	s.stateMu.Unlock()

	s.broadcastReceiverStatus(0)
}
