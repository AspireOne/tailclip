package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"tailclip/internal/config"
	"tailclip/internal/event"
)

const sharePath = "/share"

type ClipboardApplier func(context.Context, event.ClipboardEvent) error

type Server struct {
	logger    *slog.Logger
	addr      string
	authToken string
	deviceID  string
	now       func() time.Time
	apply     ClipboardApplier
}

func NewServer(logger *slog.Logger, cfg config.Config, apply ClipboardApplier) *Server {
	return &Server{
		logger:    logger,
		addr:      cfg.WindowsListenAddr,
		authToken: cfg.AuthToken,
		deviceID:  cfg.DeviceID,
		now:       time.Now,
		apply:     apply,
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if strings.TrimSpace(s.addr) == "" {
		return nil
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen for inbound shares on %q: %w", s.addr, err)
	}
	defer listener.Close()

	s.logger.Info("listening for inbound shares", "listen_addr", listener.Addr().String(), "path", sharePath)

	server := &http.Server{
		Handler: http.HandlerFunc(s.handleShare),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve inbound shares: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("shutdown inbound share server: %w", err)
		}
		<-errCh
		return nil
	}
}

func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != sharePath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r.Header.Get("Authorization")) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	evt, err := s.decodeEvent(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	evt = s.normalizeEvent(evt)
	if strings.TrimSpace(evt.Content) == "" {
		http.Error(w, "missing content", http.StatusBadRequest)
		return
	}

	if err := s.apply(r.Context(), evt); err != nil {
		s.logger.Warn("apply inbound share failed", "error", err, "event_id", evt.ID)
		http.Error(w, "apply failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) decodeEvent(r *http.Request) (event.ClipboardEvent, error) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "application/json") {
		var evt event.ClipboardEvent
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			return event.ClipboardEvent{}, errors.New("invalid json")
		}
		return evt, nil
	}
	if !strings.HasPrefix(contentType, "text/plain") {
		return event.ClipboardEvent{}, errors.New("unsupported media type")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return event.ClipboardEvent{}, errors.New("invalid request body")
	}

	return event.ClipboardEvent{Content: string(body)}, nil
}

func (s *Server) authorized(header string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return false
	}
	token := strings.TrimSpace(header[len("Bearer "):])
	return token == s.authToken
}

func (s *Server) normalizeEvent(evt event.ClipboardEvent) event.ClipboardEvent {
	if evt.SourceDeviceID == "" {
		evt.SourceDeviceID = "android-tasker"
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = s.now().UTC()
	} else {
		evt.CreatedAt = evt.CreatedAt.UTC()
	}
	evt.ContentHash = event.HashContent(evt.Content)
	if evt.ID == "" {
		evt.ID = event.NewEventID()
	}
	return evt
}
