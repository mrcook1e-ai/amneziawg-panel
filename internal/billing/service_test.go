package billing_test

import (
	"context"
	"database/sql"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/billing"
	"github.com/mrcook1e/amneziawg-panel/internal/config"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

func init() {
	log.SetOutput(io.Discard)
}

func setupTestDB(t *testing.T) (*db.DB, string) {
	tmpDir, err := os.MkdirTemp("", "billing-test-db-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")
	dbStore, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	return dbStore, tmpDir
}

func setupTestManager(t *testing.T) (*awg.Manager, string) {
	tmpDir, err := os.MkdirTemp("", "billing-test-awg-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	statePath := filepath.Join(tmpDir, "state.json")
	initialState := `{
		"schemaVersion": 4,
		"profiles": {
			"p1": {
				"id": "p1",
				"name": "p1",
				"iface": "wg0",
				"address": "10.0.0.1/24",
				"port": 51820,
				"privateKey": "privkey",
				"publicKey": "pubkey",
				"mtu": 1420
			}
		},
		"clients": {},
		"subscribers": {}
	}`
	if err := os.WriteFile(statePath, []byte(initialState), 0644); err != nil {
		t.Fatalf("failed to write initial state: %v", err)
	}

	cfg := config.Config{
		WGPath:         tmpDir,
		AWGBin:         "echo",
		AWGQuickBin:    "echo",
		WGHost:         "1.1.1.1",
		WGPort:         51820,
		Subnet:         "10.8.0.x",
		PortRangeStart: 50000,
		PortRangeEnd:   60000,
	}

	mgr, err := awg.NewManager(cfg)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	if err := mgr.Start(); err != nil {
		t.Fatalf("manager start error: %v", err)
	}

	return mgr, tmpDir
}

func cleanUp(dirs ...string) {
	for _, d := range dirs {
		_ = os.RemoveAll(d)
	}
}

func TestEqualSplitAndRemainder(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	cfg := config.Config{}
	svc := billing.NewService(dbStore, mgr, cfg)

	// Create subscribers: 3 payers, 1 trusted (exempt)
	_, err := mgr.CreateSubscriber("sub1", "notes", "payer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.CreateSubscriber("sub2", "notes", "payer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.CreateSubscriber("sub3", "notes", "payer")
	if err != nil {
		t.Fatal(err)
	}
	s4, err := mgr.CreateSubscriber("sub4", "notes", "trusted")
	if err != nil {
		t.Fatal(err)
	}

	// Create draft cycle
	cycle, err := svc.CreateDraftCycle(context.Background(), "Cycle 1", 100, 200, 300, 400, 1000)
	if err != nil {
		t.Fatalf("failed to create draft cycle: %v", err)
	}

	// Publish - should split 1000 kopecks among 3 payers:
	// 1000 / 3 = 333 kopecks each, remainder 1.
	// Payer IDs sorted deterministically.
	// Remainder distributed to the first payer.
	err = svc.PublishCycle(context.Background(), cycle.ID)
	if err != nil {
		t.Fatalf("publish error: %v", err)
	}

	// Read updated cycle
	var publishedCycle billing.Cycle
	err = dbStore.QueryRowContext(context.Background(), "SELECT id, status, payer_count FROM billing_cycles WHERE id = ?", cycle.ID).
		Scan(&publishedCycle.ID, &publishedCycle.Status, &publishedCycle.PayerCount)
	if err != nil {
		t.Fatal(err)
	}

	if publishedCycle.Status != "published" {
		t.Errorf("expected published status, got %s", publishedCycle.Status)
	}
	if publishedCycle.PayerCount != 3 {
		t.Errorf("expected 3 payers, got %d", publishedCycle.PayerCount)
	}

	// Check invoices
	rows, err := dbStore.QueryContext(context.Background(), "SELECT subscriber_id, amount, status FROM invoices WHERE cycle_id = ?", cycle.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	amounts := map[string]int64{}
	for rows.Next() {
		var subID, status string
		var amt int64
		if err := rows.Scan(&subID, &amt, &status); err != nil {
			t.Fatal(err)
		}
		amounts[subID] = amt
		if status != "pending" {
			t.Errorf("expected status pending, got %s", status)
		}
	}

	// sub4 should not have an invoice
	if _, ok := amounts[s4.ID]; ok {
		t.Errorf("trusted subscriber s4 got an invoice, should be exempt")
	}

	// Verify split amounts - order depends on deterministic IDs
	var sum int64
	for id, amt := range amounts {
		sum += amt
		if amt != 333 && amt != 334 {
			t.Errorf("subscriber %s got unexpected amount %d", id, amt)
		}
	}
	if sum != 1000 {
		t.Errorf("expected total sum 1000, got %d", sum)
	}

	_, _ = mgr.CreateSubscriber("late payer", "", "payer")
	if err := svc.PublishCycle(context.Background(), cycle.ID); err == nil {
		t.Fatal("published cycle must be immutable")
	}
	var invoiceCount int
	if err := dbStore.QueryRow("SELECT COUNT(*) FROM invoices WHERE cycle_id = ?", cycle.ID).Scan(&invoiceCount); err != nil {
		t.Fatal(err)
	}
	if invoiceCount != 3 {
		t.Fatalf("immutable cycle has %d invoices, want 3", invoiceCount)
	}
}

func TestCheckoutRejectsAnotherCabinetInvoice(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	svc := billing.NewService(dbStore, mgr, config.Config{})
	first, _ := mgr.CreateSubscriber("first", "", "payer")
	second, _ := mgr.CreateSubscriber("second", "", "payer")
	cycle, _ := svc.CreateDraftCycle(context.Background(), "Cycle", 100, 200, 300, 400, 500)
	if err := svc.PublishCycle(context.Background(), cycle.ID); err != nil {
		t.Fatal(err)
	}
	var firstInvoice int64
	if err := dbStore.QueryRow("SELECT id FROM invoices WHERE subscriber_id = ?", first.ID).Scan(&firstInvoice); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitiateCheckout(context.Background(), second.AccessToken, firstInvoice, "second@example.com"); err == nil {
		t.Fatal("another cabinet must not initiate checkout for this invoice")
	}
}

func TestManualPaymentIdempotency(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	cfg := config.Config{}
	svc := billing.NewService(dbStore, mgr, cfg)

	s1, _ := mgr.CreateSubscriber("sub1", "notes", "payer")
	cycle, _ := svc.CreateDraftCycle(context.Background(), "Cycle", 100, 200, 300, 400, 500)
	err := svc.PublishCycle(context.Background(), cycle.ID)
	if err != nil {
		t.Fatalf("publish error: %v", err)
	}

	// Find the invoice ID
	var invoiceID int64
	err = dbStore.QueryRowContext(context.Background(), "SELECT id FROM invoices WHERE subscriber_id = ?", s1.ID).Scan(&invoiceID)
	if err != nil {
		t.Fatalf("failed to find invoice: %v", err)
	}

	// Manually mark paid - first time
	err = svc.MarkInvoicePaid(context.Background(), invoiceID)
	if err != nil {
		t.Fatalf("manual pay failed: %v", err)
	}

	var status1 string
	var paidAt1 sql.NullInt64
	err = dbStore.QueryRowContext(context.Background(), "SELECT status, paid_at FROM invoices WHERE id = ?", invoiceID).Scan(&status1, &paidAt1)
	if err != nil {
		t.Fatal(err)
	}
	if status1 != "paid" {
		t.Errorf("expected paid, got %s", status1)
	}
	if !paidAt1.Valid {
		t.Errorf("expected paid_at to be valid")
	}

	// Second time - should succeed and be idempotent
	err = svc.MarkInvoicePaid(context.Background(), invoiceID)
	if err != nil {
		t.Fatalf("second manual pay failed: %v", err)
	}

	var status2 string
	var paidAt2 sql.NullInt64
	err = dbStore.QueryRowContext(context.Background(), "SELECT status, paid_at FROM invoices WHERE id = ?", invoiceID).Scan(&status2, &paidAt2)
	if err != nil {
		t.Fatal(err)
	}
	if status2 != "paid" {
		t.Errorf("expected status paid, got %s", status2)
	}
	if paidAt2.Int64 != paidAt1.Int64 {
		t.Errorf("PaidAt changed during second call")
	}
}

func TestSuspensionAndResumeOnlyMarked(t *testing.T) {
	_, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	// Create subscriber s1
	s1, _ := mgr.CreateSubscriber("sub1", "notes", "payer")

	// We need to import client c1 (enabled), c2 (disabled manually)
	c1, err := mgr.ImportClient(awg.ImportArgs{
		Name:         "c1",
		PublicKey:    "pub1",
		SubscriberID: s1.ID,
		ProfileID:    "p1",
	})
	if err != nil {
		t.Fatalf("import c1 error: %v", err)
	}

	c2, err := mgr.ImportClient(awg.ImportArgs{
		Name:         "c2",
		PublicKey:    "pub2",
		SubscriberID: s1.ID,
		ProfileID:    "p1",
	})
	if err != nil {
		t.Fatalf("import c2 error: %v", err)
	}

	// Disable c2 manually
	err = mgr.SetEnabled(c2.ID, false)
	if err != nil {
		t.Fatalf("disable c2 error: %v", err)
	}

	// Verify initial state
	clients, _ := mgr.ListClients()
	for _, c := range clients {
		if c.ID == c1.ID && !c.Enabled {
			t.Errorf("c1 should be enabled")
		}
		if c.ID == c2.ID && c.Enabled {
			t.Errorf("c2 should be disabled")
		}
	}

	// Suspend
	err = mgr.SuspendSubscriberClients(s1.ID)
	if err != nil {
		t.Fatalf("suspend failed: %v", err)
	}

	// Verify c1 is suspended (BillingSuspended=true, Enabled=false), c2 is unchanged (BillingSuspended=false, Enabled=false)
	clients, _ = mgr.ListClients()
	for _, c := range clients {
		if c.ID == c1.ID {
			if c.Enabled {
				t.Errorf("c1 should be disabled after suspension")
			}
			if !c.BillingSuspended {
				t.Errorf("c1 should have BillingSuspended true")
			}
		}
		if c.ID == c2.ID {
			if c.Enabled {
				t.Errorf("c2 should still be disabled")
			}
			if c.BillingSuspended {
				t.Errorf("c2 should not be BillingSuspended")
			}
		}
	}

	// Resume
	err = mgr.ResumeSubscriberClients(s1.ID)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}

	// Verify c1 is resumed (BillingSuspended=false, Enabled=true), c2 is still manually disabled
	clients, _ = mgr.ListClients()
	for _, c := range clients {
		if c.ID == c1.ID {
			if !c.Enabled {
				t.Errorf("c1 should be re-enabled after resume")
			}
			if c.BillingSuspended {
				t.Errorf("c1 should clear BillingSuspended flag")
			}
		}
		if c.ID == c2.ID {
			if c.Enabled {
				t.Errorf("c2 should remain disabled")
			}
			if c.BillingSuspended {
				t.Errorf("c2 should remain BillingSuspended=false")
			}
		}
	}
}

func TestCabinetBillingSummaryStatusDerivation(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	cfg := config.Config{}
	svc := billing.NewService(dbStore, mgr, cfg)

	// Create subscribers
	sOwner, _ := mgr.CreateSubscriber("owner", "notes", "owner")
	sTrusted, _ := mgr.CreateSubscriber("trusted", "notes", "trusted")
	sPayer, _ := mgr.CreateSubscriber("payer", "notes", "payer")

	// 1. Owner Summary
	sumOwner, err := svc.GetCabinetSummary(context.Background(), sOwner.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if sumOwner.BillingRole != "owner" {
		t.Errorf("expected role owner, got %s", sumOwner.BillingRole)
	}
	if sumOwner.DerivedStatus != "exempt" {
		t.Errorf("expected status exempt, got %s", sumOwner.DerivedStatus)
	}

	// 2. Trusted Summary
	sumTrusted, err := svc.GetCabinetSummary(context.Background(), sTrusted.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if sumTrusted.BillingRole != "trusted" {
		t.Errorf("expected role trusted, got %s", sumTrusted.BillingRole)
	}
	if sumTrusted.DerivedStatus != "exempt" {
		t.Errorf("expected status exempt, got %s", sumTrusted.DerivedStatus)
	}

	// 3. Payer summary with no invoice (should be paid status)
	sumPayer, err := svc.GetCabinetSummary(context.Background(), sPayer.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if sumPayer.BillingRole != "payer" {
		t.Errorf("expected role payer, got %s", sumPayer.BillingRole)
	}
	if sumPayer.DerivedStatus != "paid" {
		t.Errorf("expected status paid, got %s", sumPayer.DerivedStatus)
	}

	// 4. Payer summary with paid invoice
	cycle, _ := svc.CreateDraftCycle(context.Background(), "Cycle", 100, 200, 300, 400, 500)
	_ = svc.PublishCycle(context.Background(), cycle.ID)
	var invoiceID int64
	_ = dbStore.QueryRowContext(context.Background(), "SELECT id FROM invoices WHERE subscriber_id = ?", sPayer.ID).Scan(&invoiceID)
	_ = svc.MarkInvoicePaid(context.Background(), invoiceID)

	sumPayerPaid, _ := svc.GetCabinetSummary(context.Background(), sPayer.AccessToken)
	if sumPayerPaid.DerivedStatus != "paid" {
		t.Errorf("expected status paid, got %s", sumPayerPaid.DerivedStatus)
	}

	// 5. Payer summary with pending invoice (not due yet)
	now := time.Now().Unix()

	// Pending (now < payment_due_at)
	_, _ = dbStore.ExecContext(context.Background(), "DELETE FROM invoices")
	cPending, err := svc.CreateDraftCycle(context.Background(), "PendingCycle", now-1000, now-500, now+500, now+1000, 500)
	if err != nil {
		t.Fatalf("failed to create pending cycle: %v", err)
	}
	_ = svc.PublishCycle(context.Background(), cPending.ID)
	sumPayerPending, _ := svc.GetCabinetSummary(context.Background(), sPayer.AccessToken)
	if sumPayerPending.DerivedStatus != "pending" {
		t.Errorf("expected status pending, got %s (due in future)", sumPayerPending.DerivedStatus)
	}

	// Grace (payment_due_at <= now < grace_ends_at)
	_, _ = dbStore.ExecContext(context.Background(), "DELETE FROM invoices")
	cGrace, _ := svc.CreateDraftCycle(context.Background(), "GraceCycle", now-1000, now-500, now-200, now+200, 500)
	_ = svc.PublishCycle(context.Background(), cGrace.ID)
	sumPayerGrace, _ := svc.GetCabinetSummary(context.Background(), sPayer.AccessToken)
	if sumPayerGrace.DerivedStatus != "grace" {
		t.Errorf("expected status grace, got %s", sumPayerGrace.DerivedStatus)
	}

	// Overdue (now >= grace_ends_at)
	_, _ = dbStore.ExecContext(context.Background(), "DELETE FROM invoices")
	cOverdue, _ := svc.CreateDraftCycle(context.Background(), "OverdueCycle", now-1000, now-500, now-400, now-200, 500)
	_ = svc.PublishCycle(context.Background(), cOverdue.ID)
	sumPayerOverdue, _ := svc.GetCabinetSummary(context.Background(), sPayer.AccessToken)
	if sumPayerOverdue.DerivedStatus != "overdue" {
		t.Errorf("expected status overdue, got %s", sumPayerOverdue.DerivedStatus)
	}
}

func TestCancelInvoice(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	svc := billing.NewService(dbStore, mgr, config.Config{})
	s1, _ := mgr.CreateSubscriber("sub1", "", "payer")
	cycle, _ := svc.CreateDraftCycle(context.Background(), "C", 100, 200, 300, 400, 500)
	_ = svc.PublishCycle(context.Background(), cycle.ID)

	var invoiceID int64
	_ = dbStore.QueryRow("SELECT id FROM invoices WHERE subscriber_id = ?", s1.ID).Scan(&invoiceID)

	// Cancel a pending invoice → canceled.
	if err := svc.CancelInvoice(context.Background(), invoiceID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	var status string
	_ = dbStore.QueryRow("SELECT status FROM invoices WHERE id = ?", invoiceID).Scan(&status)
	if status != "canceled" {
		t.Fatalf("expected canceled, got %s", status)
	}

	// Idempotent: second cancel is a no-op success.
	if err := svc.CancelInvoice(context.Background(), invoiceID); err != nil {
		t.Fatalf("second cancel should be idempotent: %v", err)
	}

	// A paid invoice cannot be canceled.
	_, _ = dbStore.Exec("UPDATE invoices SET status = 'paid', paid_at = 1 WHERE id = ?", invoiceID)
	if err := svc.CancelInvoice(context.Background(), invoiceID); err == nil {
		t.Fatal("must not cancel a paid invoice")
	}
}

func TestCloseCycle(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	svc := billing.NewService(dbStore, mgr, config.Config{})
	_, _ = mgr.CreateSubscriber("sub1", "", "payer")
	cycle, _ := svc.CreateDraftCycle(context.Background(), "C", 100, 200, 300, 400, 500)

	// Cannot close a draft.
	if err := svc.CloseCycle(context.Background(), cycle.ID); err == nil {
		t.Fatal("must not close a draft cycle")
	}

	// published → closed.
	_ = svc.PublishCycle(context.Background(), cycle.ID)
	if err := svc.CloseCycle(context.Background(), cycle.ID); err != nil {
		t.Fatalf("close: %v", err)
	}
	var status string
	_ = dbStore.QueryRow("SELECT status FROM billing_cycles WHERE id = ?", cycle.ID).Scan(&status)
	if status != "closed" {
		t.Fatalf("expected closed, got %s", status)
	}

	// Double-close is rejected (not published anymore).
	if err := svc.CloseCycle(context.Background(), cycle.ID); err == nil {
		t.Fatal("must not close an already-closed cycle")
	}
}

func TestDeleteCycle(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	svc := billing.NewService(dbStore, mgr, config.Config{})
	_, _ = mgr.CreateSubscriber("sub1", "", "payer")
	cycle, _ := svc.CreateDraftCycle(context.Background(), "C", 100, 200, 300, 400, 500)

	// Draft can be deleted.
	if err := svc.DeleteCycle(context.Background(), cycle.ID); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	var n int
	_ = dbStore.QueryRow("SELECT COUNT(*) FROM billing_cycles WHERE id = ?", cycle.ID).Scan(&n)
	if n != 0 {
		t.Fatal("draft cycle should be gone")
	}

	// Published cycle cannot be deleted.
	c2, _ := svc.CreateDraftCycle(context.Background(), "C2", 100, 200, 300, 400, 500)
	_ = svc.PublishCycle(context.Background(), c2.ID)
	if err := svc.DeleteCycle(context.Background(), c2.ID); err == nil {
		t.Fatal("must not delete a published cycle")
	}
}

func TestCabinetHistoryAndContact(t *testing.T) {
	dbStore, dbPath := setupTestDB(t)
	mgr, statePath := setupTestManager(t)
	defer cleanUp(dbPath, statePath)

	cfg := config.Config{PaymentContact: "Telegram @mrcook1e"}
	svc := billing.NewService(dbStore, mgr, cfg)

	payer, _ := mgr.CreateSubscriber("payer", "", "payer")
	// Two published cycles → two history rows.
	c1, _ := svc.CreateDraftCycle(context.Background(), "C1", 100, 200, 300, 400, 500)
	_ = svc.PublishCycle(context.Background(), c1.ID)
	c2, _ := svc.CreateDraftCycle(context.Background(), "C2", 500, 600, 700, 800, 500)
	_ = svc.PublishCycle(context.Background(), c2.ID)

	sum, err := svc.GetCabinetSummary(context.Background(), payer.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if sum.PaymentContact != "Telegram @mrcook1e" {
		t.Errorf("payment contact not surfaced: %q", sum.PaymentContact)
	}
	if len(sum.History) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(sum.History))
	}
	// Latest cycle (C2) must be first because of ORDER BY published_at DESC.
	if sum.History[0].CycleTitle != "C2" {
		t.Errorf("expected C2 first, got %s", sum.History[0].CycleTitle)
	}
}
