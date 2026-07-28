package billing

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/config"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

// Status types
const (
	CycleStatusDraft     = "draft"
	CycleStatusPublished = "published"
	CycleStatusClosed    = "closed"

	InvoiceStatusPending  = "pending"
	InvoiceStatusPaid     = "paid"
	InvoiceStatusCanceled = "canceled"

	// Способ дележа суммы цикла между плательщиками.
	SplitModeEqual   = "equal"
	SplitModeTraffic = "traffic"

	trafficWindowDays = 30
)

// Cycle represents a billing cycle.
type Cycle struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	PeriodStart  int64      `json:"periodStart"`
	PeriodEnd    int64      `json:"periodEnd"`
	PaymentDueAt int64      `json:"paymentDueAt"`
	GraceEndsAt  int64      `json:"graceEndsAt"`
	TotalAmount  int64      `json:"totalAmount"` // in kopecks
	Status       string     `json:"status"`      // draft, published, closed
	SplitMode    string     `json:"splitMode"`   // equal, traffic
	PayerCount   int64      `json:"payerCount"`
	CreatedAt    int64      `json:"createdAt"`
	PublishedAt  *int64     `json:"publishedAt,omitempty"`
	Invoices     []*Invoice `json:"invoices,omitempty"`
}

// Invoice represents a subscriber invoice.
type Invoice struct {
	ID             int64  `json:"id"`
	CycleID        int64  `json:"cycleId"`
	SubscriberID   string `json:"subscriberId"`
	SubscriberName string `json:"subscriberName"`
	Amount         int64  `json:"amount"` // in kopecks
	PublicToken    string `json:"publicToken"`
	Status         string `json:"status"` // pending, paid, canceled
	PaidAt         *int64 `json:"paidAt,omitempty"`
}

// Payment represents a payment attempt.
type Payment struct {
	ID              int64   `json:"id"`
	InvoiceID       int64   `json:"invoiceId"`
	ProviderID      string  `json:"providerId"`
	IdempotencyKey  string  `json:"idempotencyKey"`
	Email           *string `json:"email,omitempty"`
	Status          string  `json:"status"`
	ConfirmationURL *string `json:"confirmationUrl,omitempty"`
	CreatedAt       int64   `json:"createdAt"`
}

// Summary statistics for billing dashboard.
type Summary struct {
	TotalReceived int64 `json:"totalReceived"` // paid invoices sum (kopecks)
	TotalPending  int64 `json:"totalPending"`  // pending invoices sum (kopecks)
}

// PublicCabinetSummary represents public client cabinet billing summary.
type PublicCabinetSummary struct {
	BillingRole     string        `json:"billingRole"`
	DerivedStatus   string        `json:"derivedStatus,omitempty"` // exempt, pending, grace, overdue, paid
	CheckoutEnabled bool          `json:"checkoutEnabled"`
	PaymentContact  string        `json:"paymentContact,omitempty"`
	LatestInvoice   *Invoice      `json:"latestInvoice,omitempty"`
	LatestCycle     *Cycle        `json:"latestCycle,omitempty"`
	History         []HistoryItem `json:"history,omitempty"`
}

// HistoryItem — одна строка истории оплат плательщика в кабинете.
type HistoryItem struct {
	CycleTitle string `json:"cycleTitle"`
	Amount     int64  `json:"amount"`
	Status     string `json:"status"`
	PeriodEnd  int64  `json:"periodEnd"`
	PaidAt     *int64 `json:"paidAt,omitempty"`
}

// Service handles billing operations, communicating with the db and awg.Manager.
type Service struct {
	DB     *db.DB
	Mgr    *awg.Manager
	Cfg    config.Config
	client *http.Client

	stopLoop chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewService creates and configures the billing service.
func NewService(dbStore *db.DB, mgr *awg.Manager, cfg config.Config) *Service {
	return &Service{
		DB:       dbStore,
		Mgr:      mgr,
		Cfg:      cfg,
		client:   &http.Client{Timeout: 10 * time.Second},
		stopLoop: make(chan struct{}),
	}
}

// StartBackgroundLoop runs the background reconciliation loop every minute.
func (s *Service) StartBackgroundLoop() {
	slog.Info("billing reconciliation loop started", slog.String("component", "billing"))
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer slog.Debug("billing reconciliation loop stopped", slog.String("component", "billing"))
		var lastReconcileError string
		reconcile := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := s.ReconcileSuspensions(ctx); err != nil {
				if lastReconcileError != err.Error() {
					slog.Error("billing reconciliation failed", slog.String("component", "billing"), slog.String("operation", "reconcile_suspensions"), slog.Any("error", err))
					lastReconcileError = err.Error()
				}
				return
			}
			if lastReconcileError != "" {
				slog.Info("billing reconciliation recovered", slog.String("component", "billing"), slog.String("operation", "reconcile_suspensions"))
				lastReconcileError = ""
			}
		}
		reconcile()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopLoop:
				return
			case <-ticker.C:
				reconcile()
			}
		}
	}()
}

// StopBackgroundLoop stops the loop.
func (s *Service) StopBackgroundLoop() {
	s.stopOnce.Do(func() { close(s.stopLoop) })
	s.wg.Wait()
}

// ReconcileSuspensions suspends subscribers with overdue pending invoices.
func (s *Service) ReconcileSuspensions(ctx context.Context) error {
	now := time.Now().Unix()
	// Find all unique payer subscriber IDs who have pending invoices whose grace_ends_at has passed.
	rows, err := s.DB.QueryContext(ctx, `
		SELECT DISTINCT i.subscriber_id 
		FROM invoices i
		JOIN billing_cycles c ON i.cycle_id = c.id
		WHERE i.status = 'pending' AND c.grace_ends_at < ?
	`, now)
	if err != nil {
		return fmt.Errorf("failed to query overdue subscribers: %w", err)
	}
	defer rows.Close()

	var overdueSubscribers []string
	for rows.Next() {
		var subID string
		if err := rows.Scan(&subID); err != nil {
			return err
		}
		overdueSubscribers = append(overdueSubscribers, subID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate overdue subscribers: %w", err)
	}

	// We also want to find active/payer subscriber IDs that DO NOT have overdue pending invoices,
	// so we can make sure we resume them if they were previously suspended.
	// But actually, we only resume subscribers that have paid their invoices, which is handled
	// either on webhook success or manually.
	// The requirement is: "find pending invoices whose grace_ends_at passed, suspend payer subscriber devices. Payment/manual paid resumes. Avoid repeatedly writing state when already suspended."
	// Let's implement SuspendSubscriberClients on the Manager. It only does something if there's any Enabled client.
	for _, subID := range overdueSubscribers {
		sub, err := s.Mgr.FindSubscriber(subID)
		if err != nil {
			slog.Warn("billing subscriber lookup failed", slog.String("component", "billing"), slog.String("operation", "find_subscriber"), slog.Any("error", err))
			continue
		}
		if sub.BillingRole != awg.BillingRolePayer {
			continue
		}
		if err := s.Mgr.SuspendSubscriberClients(subID); err != nil {
			slog.Error("billing suspension failed", slog.String("component", "billing"), slog.String("operation", "suspend_subscriber"), slog.Any("error", err))
		}
	}

	return nil
}

// ListCycles returns a list of cycles.
func (s *Service) ListCycles(ctx context.Context) ([]*Cycle, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, title, period_start, period_end, payment_due_at, grace_ends_at, total_amount, status, split_mode, payer_count, created_at, published_at
		FROM billing_cycles ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cycles []*Cycle
	for rows.Next() {
		var c Cycle
		var publishedAt sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Title, &c.PeriodStart, &c.PeriodEnd, &c.PaymentDueAt, &c.GraceEndsAt, &c.TotalAmount, &c.Status, &c.SplitMode, &c.PayerCount, &c.CreatedAt, &publishedAt); err != nil {
			return nil, err
		}
		if publishedAt.Valid {
			c.PublishedAt = &publishedAt.Int64
		}
		cycles = append(cycles, &c)
	}
	return cycles, nil
}

// CreateDraftCycle creates a new billing cycle in 'draft' state.
func (s *Service) CreateDraftCycle(ctx context.Context, title string, start, end, due, grace int64, totalAmount int64, splitMode string) (*Cycle, error) {
	if title == "" {
		return nil, errors.New("title is required")
	}
	if start >= end {
		return nil, errors.New("period start must be before period end")
	}
	if due < start {
		return nil, errors.New("payment due date must be within or after the period")
	}
	if grace < due {
		return nil, errors.New("grace ends date must be after or equal to payment due date")
	}
	if totalAmount <= 0 {
		return nil, errors.New("total amount must be greater than zero")
	}
	if splitMode == "" {
		splitMode = SplitModeEqual
	}
	if splitMode != SplitModeEqual && splitMode != SplitModeTraffic {
		return nil, fmt.Errorf("invalid splitMode: %s", splitMode)
	}

	now := time.Now().Unix()
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO billing_cycles (title, period_start, period_end, payment_due_at, grace_ends_at, total_amount, status, split_mode, payer_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)
	`, title, start, end, due, grace, totalAmount, CycleStatusDraft, splitMode, now)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Cycle{
		ID:           id,
		Title:        title,
		PeriodStart:  start,
		PeriodEnd:    end,
		PaymentDueAt: due,
		GraceEndsAt:  grace,
		TotalAmount:  totalAmount,
		Status:       CycleStatusDraft,
		SplitMode:    splitMode,
		PayerCount:   0,
		CreatedAt:    now,
	}, nil
}

// GetCycleDetail returns detailed cycle info including its invoices.
func (s *Service) GetCycleDetail(ctx context.Context, id int64) (*Cycle, error) {
	var c Cycle
	var publishedAt sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, title, period_start, period_end, payment_due_at, grace_ends_at, total_amount, status, split_mode, payer_count, created_at, published_at
		FROM billing_cycles WHERE id = ?
	`, id).Scan(&c.ID, &c.Title, &c.PeriodStart, &c.PeriodEnd, &c.PaymentDueAt, &c.GraceEndsAt, &c.TotalAmount, &c.Status, &c.SplitMode, &c.PayerCount, &c.CreatedAt, &publishedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if publishedAt.Valid {
		c.PublishedAt = &publishedAt.Int64
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, cycle_id, subscriber_id, subscriber_name, amount, public_token, status, paid_at
		FROM invoices WHERE cycle_id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var inv Invoice
		var paidAt sql.NullInt64
		if err := rows.Scan(&inv.ID, &inv.CycleID, &inv.SubscriberID, &inv.SubscriberName, &inv.Amount, &inv.PublicToken, &inv.Status, &paidAt); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			inv.PaidAt = &paidAt.Int64
		}
		c.Invoices = append(c.Invoices, &inv)
	}

	return &c, nil
}

// PublishCycle transition cycle to published, snapshots payers, and splits the
// total_amount according to cycle.split_mode. Implemented as a single database
// transaction. Idempotently rejects non-draft/zero payers.
func (s *Service) PublishCycle(ctx context.Context, cycleID int64) error {
	// 1. Get payers from manager
	payerIDs := s.Mgr.ListPayerSubscriberIDs()
	payerCount := int64(len(payerIDs))
	if payerCount == 0 {
		return errors.New("cannot publish cycle with zero payers")
	}

	// 2. Fetch cycle meta and ensure status is draft (single SQLite conn → no TOCTOU).
	var status, splitMode string
	var totalAmount int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT status, total_amount, split_mode FROM billing_cycles WHERE id = ?
	`, cycleID).Scan(&status, &totalAmount, &splitMode)
	if err != nil {
		return err
	}
	if status != CycleStatusDraft {
		return fmt.Errorf("cannot publish cycle in %s status", status)
	}
	if splitMode == "" {
		splitMode = SplitModeEqual
	}

	// 3. Compute per-payer amounts (traffic query reads SQLite + manager outside tx).
	amounts := s.computeSplit(ctx, totalAmount, payerIDs, splitMode)

	// Begin SQL transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()

	// 4. Create invoices
	for _, subID := range payerIDs {
		sub, err := s.Mgr.FindSubscriber(subID)
		subName := "Unknown"
		if err == nil && sub != nil {
			subName = sub.Name
		}

		publicToken, err := generateRandomToken()
		if err != nil {
			return fmt.Errorf("failed to generate public token: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO invoices (cycle_id, subscriber_id, subscriber_name, amount, public_token, status)
			VALUES (?, ?, ?, ?, ?, ?)
		`, cycleID, subID, subName, amounts[subID], publicToken, InvoiceStatusPending)
		if err != nil {
			return err
		}
	}

	// 5. Update cycle status
	_, err = tx.ExecContext(ctx, `
		UPDATE billing_cycles
		SET status = ?, payer_count = ?, published_at = ?
		WHERE id = ?
	`, CycleStatusPublished, payerCount, now, cycleID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// MarkInvoicePaid manually marks an invoice as paid.
// Resumes the subscriber's devices. Idempotent.
func (s *Service) MarkInvoicePaid(ctx context.Context, invoiceID int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var subID string
	err = tx.QueryRowContext(ctx, `
		SELECT status, subscriber_id FROM invoices WHERE id = ?
	`, invoiceID).Scan(&status, &subID)
	if err != nil {
		return err
	}

	if status == InvoiceStatusPaid {
		_ = tx.Rollback()
		return s.resumeSubscriberIfSettled(ctx, subID)
	}

	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `
		UPDATE invoices SET status = ?, paid_at = ? WHERE id = ?
	`, InvoiceStatusPaid, now, invoiceID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return s.resumeSubscriberIfSettled(ctx, subID)
}

// CancelInvoice списывает pending-счёт (админ «простил»). Idempotent для уже
// списанных; paid отменить нельзя. Если у подписчика не осталось просроченных
// pending-счетов — устройства реактивируются.
func (s *Service) CancelInvoice(ctx context.Context, invoiceID int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status, subID string
	err = tx.QueryRowContext(ctx, `
		SELECT status, subscriber_id FROM invoices WHERE id = ?
	`, invoiceID).Scan(&status, &subID)
	if err != nil {
		return err
	}
	if status == InvoiceStatusCanceled {
		_ = tx.Rollback()
		return s.resumeSubscriberIfSettled(ctx, subID)
	}
	if status == InvoiceStatusPaid {
		return errors.New("cannot cancel a paid invoice")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE invoices SET status = ? WHERE id = ?`, InvoiceStatusCanceled, invoiceID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.resumeSubscriberIfSettled(ctx, subID)
}

func (s *Service) resumeSubscriberIfSettled(ctx context.Context, subscriberID string) error {
	var overdue int
	err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM invoices i
		JOIN billing_cycles c ON c.id = i.cycle_id
		WHERE i.subscriber_id = ? AND i.status = 'pending' AND c.grace_ends_at < ?
	`, subscriberID, time.Now().Unix()).Scan(&overdue)
	if err != nil {
		return err
	}
	if overdue != 0 {
		return nil
	}
	return s.Mgr.ResumeSubscriberClients(subscriberID)
}

// CloseCycle переводит опубликованный цикл в closed (архив). Чисто маркер: на
// derived-статус и доступ не влияет — закрытые циклы остаются в истории.
func (s *Service) CloseCycle(ctx context.Context, cycleID int64) error {
	var status string
	err := s.DB.QueryRowContext(ctx, `SELECT status FROM billing_cycles WHERE id = ?`, cycleID).Scan(&status)
	if err != nil {
		return err
	}
	if status != CycleStatusPublished {
		return fmt.Errorf("cannot close cycle in %s status", status)
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE billing_cycles SET status = ? WHERE id = ?`, CycleStatusClosed, cycleID)
	return err
}

// DeleteCycle удаляет цикл. Разрешён только draft — у опубликованных/закрытых
// уже есть счета и, возможно, платежи; их архивируйте через CloseCycle.
// У draft нет счетов, так что каскад через FK остаётся страховкой.
func (s *Service) DeleteCycle(ctx context.Context, cycleID int64) error {
	var status string
	err := s.DB.QueryRowContext(ctx, `SELECT status FROM billing_cycles WHERE id = ?`, cycleID).Scan(&status)
	if err != nil {
		return err
	}
	if status != CycleStatusDraft {
		return fmt.Errorf("only draft cycles can be deleted (this one is %s)", status)
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM billing_cycles WHERE id = ?`, cycleID)
	return err
}

// SubscriberAccessAllowed checks the authoritative invoice state before a
// payer creates another device. Exempt roles are always allowed.
func (s *Service) SubscriberAccessAllowed(ctx context.Context, subscriberID string) (bool, error) {
	sub, err := s.Mgr.FindSubscriber(subscriberID)
	if err != nil {
		return false, err
	}
	if sub.BillingRole != awg.BillingRolePayer {
		return true, nil
	}
	var overdue int
	err = s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM invoices i
		JOIN billing_cycles c ON c.id = i.cycle_id
		WHERE i.subscriber_id = ? AND i.status = 'pending' AND c.grace_ends_at < ?
	`, subscriberID, time.Now().Unix()).Scan(&overdue)
	return overdue == 0, err
}

// GetSummary returns total received and total pending amounts.
func (s *Service) GetSummary(ctx context.Context) (*Summary, error) {
	var sum Summary
	err := s.DB.QueryRowContext(ctx, `
		SELECT IFNULL(SUM(amount), 0) FROM invoices WHERE status = 'paid'
	`).Scan(&sum.TotalReceived)
	if err != nil {
		return nil, err
	}

	err = s.DB.QueryRowContext(ctx, `
		SELECT IFNULL(SUM(amount), 0) FROM invoices WHERE status = 'pending'
	`).Scan(&sum.TotalPending)
	if err != nil {
		return nil, err
	}

	return &sum, nil
}

// GetCabinetSummary returns the cabinet billing summary based on the access token.
func (s *Service) GetCabinetSummary(ctx context.Context, token string) (*PublicCabinetSummary, error) {
	sub, err := s.Mgr.FindSubscriberByToken(token)
	if err != nil {
		return nil, err
	}

	role := sub.BillingRole
	if role == "" {
		role = "trusted"
	}

	res := &PublicCabinetSummary{
		BillingRole:     role,
		CheckoutEnabled: s.Cfg.YookassaShopID != "" && s.Cfg.YookassaSecretKey != "" && s.Cfg.PublicURL != "",
		PaymentContact:  s.Cfg.PaymentContact,
	}

	if role == "owner" || role == "trusted" {
		res.DerivedStatus = "exempt"
		return res, nil
	}

	// For payers, find the latest published or closed cycle invoice
	// In SQLite: get the latest invoice based on cycle's published_at or created_at.
	// Since invoices are created when the cycle is published, latest invoice is from the latest published cycle.
	var inv Invoice
	var c Cycle
	var paidAt sql.NullInt64
	var publishedAt sql.NullInt64

	err = s.DB.QueryRowContext(ctx, `
		SELECT i.id, i.cycle_id, i.subscriber_id, i.subscriber_name, i.amount, i.public_token, i.status, i.paid_at,
		       c.id, c.title, c.period_start, c.period_end, c.payment_due_at, c.grace_ends_at, c.total_amount, c.status, c.split_mode, c.payer_count, c.created_at, c.published_at
		FROM invoices i
		JOIN billing_cycles c ON i.cycle_id = c.id
		WHERE i.subscriber_id = ? AND c.status IN ('published', 'closed')
		ORDER BY c.published_at DESC, i.id DESC LIMIT 1
	`, sub.ID).Scan(
		&inv.ID, &inv.CycleID, &inv.SubscriberID, &inv.SubscriberName, &inv.Amount, &inv.PublicToken, &inv.Status, &paidAt,
		&c.ID, &c.Title, &c.PeriodStart, &c.PeriodEnd, &c.PaymentDueAt, &c.GraceEndsAt, &c.TotalAmount, &c.Status, &c.SplitMode, &c.PayerCount, &c.CreatedAt, &publishedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			res.DerivedStatus = "paid" // No invoices means no pending debts
			return res, nil
		}
		return nil, err
	}

	if paidAt.Valid {
		inv.PaidAt = &paidAt.Int64
	}
	if publishedAt.Valid {
		c.PublishedAt = &publishedAt.Int64
	}

	res.LatestInvoice = &inv
	res.LatestCycle = &c

	// Derived status pending/grace/overdue/paid
	if inv.Status == InvoiceStatusPaid {
		res.DerivedStatus = "paid"
	} else if inv.Status == InvoiceStatusCanceled {
		// Canceled = списан/не востребован: доступ не блокируем.
		res.DerivedStatus = "paid"
	} else {
		now := time.Now().Unix()
		if now > c.GraceEndsAt {
			res.DerivedStatus = "overdue"
		} else if now > c.PaymentDueAt {
			res.DerivedStatus = "grace"
		} else {
			res.DerivedStatus = "pending"
		}
	}

	res.History = s.loadSubscriberHistory(ctx, sub.ID)
	return res, nil
}

// loadSubscriberHistory возвращает последние 12 счетов плательщика с названием
// периода. Ошибки логируются и проглатываются — история необязательна для UX.
func (s *Service) loadSubscriberHistory(ctx context.Context, subscriberID string) []HistoryItem {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT c.title, c.period_end, i.amount, i.status, i.paid_at
		FROM invoices i
		JOIN billing_cycles c ON i.cycle_id = c.id
		WHERE i.subscriber_id = ?
		ORDER BY c.published_at DESC, i.id DESC
		LIMIT 12
	`, subscriberID)
	if err != nil {
		slog.Warn("billing history query failed", slog.String("component", "billing"), slog.String("operation", "load_subscriber_history"), slog.Any("error", err))
		return nil
	}
	defer rows.Close()
	var items []HistoryItem
	for rows.Next() {
		var h HistoryItem
		var paidAt sql.NullInt64
		if err := rows.Scan(&h.CycleTitle, &h.PeriodEnd, &h.Amount, &h.Status, &paidAt); err != nil {
			slog.Warn("billing history scan failed", slog.String("component", "billing"), slog.String("operation", "load_subscriber_history"), slog.Any("error", err))
			return nil
		}
		if paidAt.Valid {
			h.PaidAt = &paidAt.Int64
		}
		items = append(items, h)
	}
	if err := rows.Err(); err != nil {
		slog.Warn("billing history iteration failed", slog.String("component", "billing"), slog.String("operation", "load_subscriber_history"), slog.Any("error", err))
		return nil
	}
	return items
}

// InitiateCheckout creates a YooKassa payment attempt for the given invoice and customer email.
func (s *Service) InitiateCheckout(ctx context.Context, cabinetToken string, invoiceID int64, email string) (string, error) {
	sub, err := s.Mgr.FindSubscriberByToken(cabinetToken)
	if err != nil {
		return "", err
	}
	// Verify invoice is pending
	var inv Invoice
	var c Cycle
	err = s.DB.QueryRowContext(ctx, `
		SELECT i.id, i.subscriber_id, i.amount, i.public_token, i.status, c.title
		FROM invoices i
		JOIN billing_cycles c ON i.cycle_id = c.id
		WHERE i.id = ?
	`, invoiceID).Scan(&inv.ID, &inv.SubscriberID, &inv.Amount, &inv.PublicToken, &inv.Status, &c.Title)
	if err != nil {
		return "", err
	}
	if inv.Status != InvoiceStatusPending {
		return "", errors.New("invoice is not in pending status")
	}
	if inv.SubscriberID != sub.ID {
		return "", errors.New("invoice does not belong to this cabinet")
	}

	// Check if YooKassa is enabled
	if s.Cfg.YookassaShopID == "" || s.Cfg.YookassaSecretKey == "" {
		return "", errors.New("yookassa payment provider is not configured")
	}
	if s.Cfg.PublicURL == "" {
		return "", errors.New("PUBLIC_URL is required for yookassa checkout")
	}

	// Generate idempotency key
	idempotencyKey, err := generateRandomToken()
	if err != nil {
		return "", err
	}

	// Setup YooKassa payment request
	amountStr := fmt.Sprintf("%d.%02d", inv.Amount/100, inv.Amount%100)
	returnURL := fmt.Sprintf("%s/payment/return/%s", strings.TrimSuffix(s.Cfg.PublicURL, "/"), inv.PublicToken)

	type AmountReq struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	}
	type ConfirmationReq struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	}
	type ItemReq struct {
		Description string    `json:"description"`
		Quantity    string    `json:"quantity"`
		Amount      AmountReq `json:"amount"`
		VatCode     int       `json:"vat_code"`
		PaymentMode string    `json:"payment_mode"`
		Subject     string    `json:"payment_subject"`
	}
	type ReceiptReq struct {
		Customer struct {
			Email string `json:"email"`
		} `json:"customer"`
		Items []ItemReq `json:"items"`
	}
	type PaymentReq struct {
		Amount       AmountReq       `json:"amount"`
		Capture      bool            `json:"capture"`
		Confirmation ConfirmationReq `json:"confirmation"`
		Metadata     map[string]any  `json:"metadata"`
		Receipt      *ReceiptReq     `json:"receipt,omitempty"`
	}

	reqPayload := PaymentReq{
		Amount: AmountReq{
			Value:    amountStr,
			Currency: "RUB",
		},
		Capture: true,
		Confirmation: ConfirmationReq{
			Type:      "redirect",
			ReturnURL: returnURL,
		},
		Metadata: map[string]any{
			"invoice_id": strconv.FormatInt(inv.ID, 10),
		},
	}

	if email != "" {
		reqPayload.Receipt = &ReceiptReq{}
		reqPayload.Receipt.Customer.Email = email
		reqPayload.Receipt.Items = []ItemReq{
			{
				Description: fmt.Sprintf("Доступ к VPN: %s", c.Title),
				Quantity:    "1.00",
				Amount: AmountReq{
					Value:    amountStr,
					Currency: "RUB",
				},
				VatCode:     s.Cfg.YookassaVatCode,
				PaymentMode: "full_payment",
				Subject:     "service",
			},
		}
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.yookassa.ru/v3/payments", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", idempotencyKey)
	req.SetBasicAuth(s.Cfg.YookassaShopID, s.Cfg.YookassaSecretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("yookassa payment creation failed: status %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var respPayload struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Confirmation struct {
			ConfirmationURL string `json:"confirmation_url"`
		} `json:"confirmation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", err
	}

	// Store payment attempt
	now := time.Now().Unix()
	_, err = s.DB.ExecContext(ctx, `
		INSERT INTO payments (invoice_id, provider_id, idempotency_key, email, status, confirmation_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, inv.ID, respPayload.ID, idempotencyKey, email, respPayload.Status, respPayload.Confirmation.ConfirmationURL, now, now)
	if err != nil {
		return "", err
	}

	return respPayload.Confirmation.ConfirmationURL, nil
}

type yookassaPayment struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Paid   bool   `json:"paid"`
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Metadata struct {
		InvoiceID string `json:"invoice_id"`
	} `json:"metadata"`
}

func (s *Service) fetchYookassaPayment(ctx context.Context, providerID string) (*yookassaPayment, error) {
	if s.Cfg.YookassaShopID == "" || s.Cfg.YookassaSecretKey == "" {
		return nil, errors.New("yookassa payment provider is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.yookassa.ru/v3/payments/"+providerID, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.Cfg.YookassaShopID, s.Cfg.YookassaSecretKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch yookassa payment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yookassa verification returned status %d", resp.StatusCode)
	}
	var payment yookassaPayment
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payment); err != nil {
		return nil, fmt.Errorf("decode yookassa payment: %w", err)
	}
	if payment.ID != providerID {
		return nil, errors.New("yookassa payment ID mismatch")
	}
	return &payment, nil
}

func parseRUBKopecks(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("invalid RUB amount %q", value)
	}
	rubles, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	kopecks, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || kopecks < 0 || kopecks > 99 {
		return 0, fmt.Errorf("invalid RUB amount %q", value)
	}
	return rubles*100 + kopecks, nil
}

func (s *Service) applyVerifiedPayment(ctx context.Context, payment *yookassaPayment) error {
	invoiceID, err := strconv.ParseInt(payment.Metadata.InvoiceID, 10, 64)
	if err != nil || invoiceID <= 0 {
		return errors.New("missing invoice_id in yookassa metadata")
	}
	amount, err := parseRUBKopecks(payment.Amount.Value)
	if err != nil {
		return err
	}
	var localInvoiceID, expectedAmount int64
	var subscriberID string
	err = s.DB.QueryRowContext(ctx, `
		SELECT p.invoice_id, i.subscriber_id, i.amount
		FROM payments p JOIN invoices i ON i.id = p.invoice_id
		WHERE p.provider_id = ?
	`, payment.ID).Scan(&localInvoiceID, &subscriberID, &expectedAmount)
	if err != nil {
		return fmt.Errorf("unknown yookassa payment: %w", err)
	}
	if localInvoiceID != invoiceID || expectedAmount != amount || payment.Amount.Currency != "RUB" {
		return errors.New("yookassa payment does not match local invoice")
	}
	if payment.Status == "succeeded" && !payment.Paid {
		return errors.New("yookassa succeeded payment is not marked paid")
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `UPDATE payments SET status = ?, updated_at = ? WHERE provider_id = ?`, payment.Status, now, payment.ID); err != nil {
		return err
	}
	if payment.Status == "succeeded" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE invoices SET status = 'paid', paid_at = COALESCE(paid_at, ?)
			WHERE id = ? AND status IN ('pending', 'paid')
		`, now, invoiceID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if payment.Status == "succeeded" {
		return s.resumeSubscriberIfSettled(ctx, subscriberID)
	}
	return nil
}

// HandleYookassaWebhook verifies the notification against YooKassa's API.
func (s *Service) HandleYookassaWebhook(ctx context.Context, body []byte) error {
	var notification struct {
		Event  string `json:"event"`
		Object struct {
			ID string `json:"id"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &notification); err != nil {
		return fmt.Errorf("invalid webhook json: %w", err)
	}

	providerPaymentID := notification.Object.ID
	if providerPaymentID == "" {
		return errors.New("empty payment ID in webhook")
	}
	if notification.Event != "payment.succeeded" && notification.Event != "payment.canceled" {
		return fmt.Errorf("unsupported yookassa event %q", notification.Event)
	}
	payment, err := s.fetchYookassaPayment(ctx, providerPaymentID)
	if err != nil {
		return err
	}
	if notification.Event != "payment."+payment.Status {
		return errors.New("yookassa notification status mismatch")
	}
	return s.applyVerifiedPayment(ctx, payment)
}

// ReconcilePaymentByPublicToken retrieves the latest payment for the invoice, fetches its current state from YooKassa,
// and processes it. Returns the subscriber's AccessToken.
func (s *Service) ReconcilePaymentByPublicToken(ctx context.Context, publicToken string) (string, string, error) {
	var invoiceID int64
	var subID string
	var invStatus string
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, subscriber_id, status FROM invoices WHERE public_token = ?
	`, publicToken).Scan(&invoiceID, &subID, &invStatus)
	if err != nil {
		return "", "", err
	}

	sub, err := s.Mgr.FindSubscriber(subID)
	if err != nil {
		return "", "", err
	}

	if invStatus == InvoiceStatusPaid {
		return sub.AccessToken, "success", nil
	}

	// Find the latest payment attempt for this invoice.
	var providerID sql.NullString
	err = s.DB.QueryRowContext(ctx, `
		SELECT provider_id FROM payments WHERE invoice_id = ? ORDER BY id DESC LIMIT 1
	`, invoiceID).Scan(&providerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No payments attempts made yet.
			return sub.AccessToken, "pending", nil
		}
		return "", "", err
	}
	if !providerID.Valid || providerID.String == "" {
		return sub.AccessToken, "pending", nil
	}
	payment, err := s.fetchYookassaPayment(ctx, providerID.String)
	if err == nil {
		err = s.applyVerifiedPayment(ctx, payment)
	}
	if err != nil {
		slog.Warn("billing payment reconciliation failed", slog.String("component", "billing"), slog.String("operation", "reconcile_payment"), slog.Any("error", err))
	}
	if err == nil && payment.Status == "succeeded" {
		return sub.AccessToken, "success", nil
	}
	// A payment-provider outage must not strand the user on an error page.
	return sub.AccessToken, "pending", nil
}

func generateRandomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// computeSplit маршрутизирует на equal/traffic в зависимости от режима цикла.
func (s *Service) computeSplit(ctx context.Context, totalAmount int64, payerIDs []string, mode string) map[string]int64 {
	if mode == SplitModeTraffic {
		traffic := s.subscribersTraffic(ctx, payerIDs, trafficWindowDays)
		return splitByTraffic(totalAmount, traffic, payerIDs, s.Cfg.BillingMinSharePct)
	}
	return splitEqual(totalAmount, payerIDs)
}

// splitEqual делит поровну с детерминированным распределением остатка (коп.) на
// первые по sorted-ID плательщиков.
func splitEqual(totalAmount int64, payerIDs []string) map[string]int64 {
	n := int64(len(payerIDs))
	out := make(map[string]int64, n)
	if n == 0 {
		return out
	}
	share := totalAmount / n
	remainder := totalAmount % n
	for i, id := range payerIDs {
		out[id] = share
		if int64(i) < remainder {
			out[id]++
		}
	}
	return out
}

// splitByTraffic делит totalAmount пропорционально трафику с полом: каждый платит
// не меньше minSharePct% от равной доли. Остаток в копейках раздаётся
// детерминированно (по убыванию дробной части, при равенстве — по sorted-ID).
func splitByTraffic(totalAmount int64, traffic map[string]uint64, payerIDs []string, minSharePct int) map[string]int64 {
	n := len(payerIDs)
	out := make(map[string]int64, n)
	if n == 0 {
		return out
	}
	if minSharePct < 0 {
		minSharePct = 0
	}
	if minSharePct > 100 {
		minSharePct = 100
	}

	equalShare := float64(totalAmount) / float64(n)
	floor := equalShare * float64(minSharePct) / 100.0

	raw := make(map[string]float64, n)
	active := make(map[string]bool, n)
	remaining := float64(totalAmount)
	for _, id := range payerIDs {
		active[id] = true
	}
	// Water-filling: тех, у кого пропорция ниже пола — фиксируем на полу, остаток
	// перекладываем пропорционально на оставшихся. Итеративно сходится.
	for {
		var sumW uint64
		for id := range active {
			sumW += traffic[id]
		}
		if sumW == 0 {
			share := remaining / float64(len(active))
			for id := range active {
				raw[id] = share
			}
			break
		}
		var clamp []string
		for id := range active {
			if remaining*float64(traffic[id])/float64(sumW) < floor {
				clamp = append(clamp, id)
			}
		}
		if len(clamp) == 0 {
			for id := range active {
				raw[id] = remaining * float64(traffic[id]) / float64(sumW)
			}
			break
		}
		for _, id := range clamp {
			raw[id] = floor
			remaining -= floor
			delete(active, id)
		}
		if len(active) == 0 {
			// Вырожденный случай: все на полу. Добиваем остаток поровну, чтобы
			// сумма осталась равна totalAmount (при minSharePct<100 недостижимо).
			if remaining > 0 {
				add := remaining / float64(n)
				for _, id := range payerIDs {
					raw[id] += add
				}
			}
			break
		}
	}

	// Округляем до копеек, остаток копеек — тем, у кого больше дробная часть.
	type frac struct {
		id string
		f  float64
	}
	fracs := make([]frac, 0, n)
	var assigned int64
	for _, id := range payerIDs {
		k := int64(raw[id])
		out[id] = k
		assigned += k
		fracs = append(fracs, frac{id, raw[id] - float64(k)})
	}
	leftover := totalAmount - assigned
	sort.Slice(fracs, func(i, j int) bool {
		if fracs[i].f != fracs[j].f {
			return fracs[i].f > fracs[j].f
		}
		return fracs[i].id < fracs[j].id
	})
	for i := 0; i < int(leftover) && i < len(fracs); i++ {
		out[fracs[i].id]++
	}
	return out
}

// subscribersTraffic суммирует rx+tx за последние days дней по всем устройствам
// каждого плательщика из peer_daily (day — unix-секунды полуночи UTC).
func (s *Service) subscribersTraffic(ctx context.Context, subscriberIDs []string, days int) map[string]uint64 {
	out := make(map[string]uint64, len(subscriberIDs))
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	cutoff := dayStart.Add(-time.Duration(days) * 24 * time.Hour).Unix()

	for _, subID := range subscriberIDs {
		devIDs := s.Mgr.DeviceIDsBySubscriber(subID)
		if len(devIDs) == 0 {
			continue
		}
		placeholders := make([]string, len(devIDs))
		args := make([]any, 0, len(devIDs)+1)
		for i, id := range devIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		args = append(args, cutoff)
		query := fmt.Sprintf(
			`SELECT COALESCE(SUM(rx),0)+COALESCE(SUM(tx),0) FROM peer_daily WHERE client_id IN (%s) AND day >= ?`,
			strings.Join(placeholders, ","),
		)
		var total uint64
		if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
			slog.Warn("billing traffic query failed", slog.String("component", "billing"), slog.String("operation", "subscriber_traffic"), slog.Any("error", err))
		}
		out[subID] = total
	}
	return out
}

// PreviewLine — строка предпросмотра дележа для админа (до публикации).
type PreviewLine struct {
	SubscriberID   string `json:"subscriberId"`
	SubscriberName string `json:"subscriberName"`
	Bytes          uint64 `json:"bytes"`
	Amount         int64  `json:"amount"` // копейки
}

// PreviewSplit считает делёж цикла по его режиму без записи в БД. Трафик берётся
// за последние 30 дней — ровно тот же, что применится при публикации.
func (s *Service) PreviewSplit(ctx context.Context, cycleID int64) ([]PreviewLine, error) {
	var totalAmount int64
	var splitMode string
	err := s.DB.QueryRowContext(ctx, `SELECT total_amount, split_mode FROM billing_cycles WHERE id = ?`, cycleID).Scan(&totalAmount, &splitMode)
	if err != nil {
		return nil, err
	}
	if splitMode == "" {
		splitMode = SplitModeEqual
	}
	payerIDs := s.Mgr.ListPayerSubscriberIDs()
	amounts := s.computeSplit(ctx, totalAmount, payerIDs, splitMode)
	traffic := map[string]uint64{}
	if splitMode == SplitModeTraffic {
		traffic = s.subscribersTraffic(ctx, payerIDs, trafficWindowDays)
	}
	out := make([]PreviewLine, 0, len(payerIDs))
	for _, id := range payerIDs {
		name := "Unknown"
		if sub, err := s.Mgr.FindSubscriber(id); err == nil && sub != nil {
			name = sub.Name
		}
		out = append(out, PreviewLine{SubscriberID: id, SubscriberName: name, Bytes: traffic[id], Amount: amounts[id]})
	}
	return out, nil
}
