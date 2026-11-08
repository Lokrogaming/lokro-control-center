package uptime

import (
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"lokro-control-center/backend/internal/db"
	"lokro-control-center/backend/internal/models"
)

// Service performs periodic uptime checks and stores results.
type Service struct {
	store   db.Store
	logger  *slog.Logger
	checks  map[int64]*models.UptimeCheck
	mu      sync.RWMutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
}

// NewService creates an uptime service (not started).
func NewService(store db.Store, logger *slog.Logger) *Service {
	return &Service{
		store:   store,
		logger:  logger,
		checks:  make(map[int64]*models.UptimeCheck),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Start loads checks and begins periodic checking.
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	if err := s.loadChecks(); err != nil {
		return err
	}
	s.running = true
	go s.run()
	s.logger.Info("uptime checker started", "checks", len(s.checks))
	return nil
}

// Stop halts the checker.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	close(s.stopCh)
	<-s.doneCh
	s.running = false
	s.logger.Info("uptime checker stopped")
	return nil
}

func (s *Service) loadChecks() error {
	checks, err := s.store.ListUptimeChecks()
	if err != nil {
		return err
	}
	for _, c := range checks {
		s.checks[c.ID] = c
	}
	return nil
}

func (s *Service) run() {
	defer close(s.doneCh)
	for {
		s.checkAll()
		// Wait for next tick or stop.
		select {
		case <-s.stopCh:
			return
		case <-time.After(10 * time.Second): // base tick; individual intervals handled per-check
		}
	}
}

func (s *Service) checkAll() {
	s.mu.RLock()
	checks := make([]*models.UptimeCheck, 0, len(s.checks))
	for _, c := range s.checks {
		if c.IsActive {
			checks = append(checks, c)
		}
	}
	s.mu.RUnlock()

	var wg sync.WaitGroup
	for _, c := range checks {
		wg.Add(1)
		go func(check *models.UptimeCheck) {
			defer wg.Done()
			s.performCheck(check)
		}(c)
	}
	wg.Wait()
}

func (s *Service) performCheck(check *models.UptimeCheck) {
	// Respect per-check interval: skip if last result is within interval.
	if s.recentResultExists(check.ID, check.Interval) {
		return
	}

	start := time.Now()
	status := "down"
	statusCode := 0
	var errStr string

	switch check.Type {
	case "tcp":
		status, statusCode, errStr = s.tcpCheck(check.URL, check.Timeout)
	case "http", "https":
		status, statusCode, errStr = s.httpCheck(check.URL, check.ExpectedStatus, check.Timeout)
	default:
		status, statusCode, errStr = s.httpCheck(check.URL, check.ExpectedStatus, check.Timeout)
	}

	responseTime := int(time.Since(start).Milliseconds())
	if errStr == "" {
		status = "up"
		if statusCode == 0 {
			statusCode = 200
		}
	}

	if err := s.store.InsertUptimeResult(check.ID, status, responseTime, statusCode, errStr); err != nil {
		s.logger.Error("failed to store uptime result", "check_id", check.ID, "error", err)
	}
}

func (s *Service) recentResultExists(checkID int64, intervalSeconds int) bool {
	results, err := s.store.ListUptimeResults(checkID, 1)
	if err != nil || len(results) == 0 {
		return false
	}
	last := results[0].CheckedAt
	return time.Since(last) < time.Duration(intervalSeconds)*time.Second
}

func (s *Service) httpCheck(url string, expectedStatus, timeoutSec int) (string, int, string) {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return "down", 0, err.Error()
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == expectedStatus {
		return "up", resp.StatusCode, ""
	}
	return "down", resp.StatusCode, fmt.Sprintf("unexpected status %d", resp.StatusCode)
}

func (s *Service) tcpCheck(address string, timeoutSec int) (string, int, string) {
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	conn, err := net.DialTimeout("tcp", address, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return "down", 0, err.Error()
	}
	conn.Close()
	return "up", 0, ""
}

// EnsureURLScheme normalizes URLs for http/https checks.
func EnsureURLScheme(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}

var _ = slog.Level(0)
