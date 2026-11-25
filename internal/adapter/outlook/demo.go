package outlook

import (
	"context"
	"fmt"
	"sync"
	"time"

	appinbox "github.com/dua-os/dua-os-be/internal/app/inbox"
	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// DemoMailbox seeds a realistic GM mailbox for local boots and CI without Microsoft credentials.
type DemoMailbox struct {
	mu    sync.Mutex
	sent  map[string][]inbox.RemoteMessage
	clock func() time.Time
}

func NewDemo() *DemoMailbox {
	return &DemoMailbox{sent: map[string][]inbox.RemoteMessage{}, clock: time.Now}
}

func (d *DemoMailbox) Connect(_ context.Context, workspaceID id.ID) (inbox.Connection, []inbox.RemoteMessage, error) {
	now := d.clock().UTC()
	conn := inbox.Connection{
		WorkspaceID: workspaceID, Provider: inbox.ProviderDemo,
		Email:          "gm@" + workspaceID.String()[:8] + ".outlook.demo",
		DisplayName:    "General Manager",
		LastSyncStatus: inbox.SyncOK, CreatedAt: now, UpdatedAt: now,
	}
	msgs := sampleMessages(now)
	d.mu.Lock()
	extra := d.sent[workspaceID.String()]
	d.mu.Unlock()
	if len(extra) > 0 {
		msgs = append(extra, msgs...)
	}
	return conn, msgs, nil
}

func (d *DemoMailbox) Send(_ context.Context, workspaceID id.ID, draft inbox.Draft, fromEmail string) error {
	now := d.clock().UTC()
	msg := inbox.RemoteMessage{
		ExternalID: "demo-sent-" + id.New().String(), Folder: inbox.FolderSent,
		FromName: "General Manager", FromEmail: fromEmail,
		ToEmails: stringsJoin(draft.To), Subject: draft.Subject, BodyText: draft.Body,
		ReceivedAt: now, IsRead: true,
	}
	d.mu.Lock()
	d.sent[workspaceID.String()] = append([]inbox.RemoteMessage{msg}, d.sent[workspaceID.String()]...)
	d.mu.Unlock()
	return nil
}

func sampleMessages(now time.Time) []inbox.RemoteMessage {
	return []inbox.RemoteMessage{
		{
			ExternalID: "demo-1", Folder: inbox.FolderInbox,
			FromName: "Sara Al-Harbi", FromEmail: "sara.guest@example.com",
			ToEmails: "gm@demo", Subject: "Late checkout request for room 412",
			BodyText:   "Good evening,\n\nWe would like to request a late checkout until 15:00 tomorrow. Our flight is in the evening. Please confirm if this is possible and if any fee applies.\n\nThank you,\nSara",
			ReceivedAt: now.Add(-35 * time.Minute), IsRead: false,
		},
		{
			ExternalID: "demo-2", Folder: inbox.FolderInbox,
			FromName: "Booking.com Partner", FromEmail: "partners@booking.com",
			ToEmails: "gm@demo", Subject: "New reservation #BKG-88421 — 2 nights",
			BodyText:   "A new reservation has been confirmed.\nGuest: Omar Haddad\nArrival: tomorrow\nNights: 2\nRoom type: Deluxe King\nRate: USD 180 / night\nPlease ensure the room is ready by 14:00.",
			ReceivedAt: now.Add(-2 * time.Hour), IsRead: false,
		},
		{
			ExternalID: "demo-3", Folder: inbox.FolderInbox,
			FromName: "Fresh Dairy Co.", FromEmail: "orders@freshdairy.sa",
			ToEmails: "gm@demo", Subject: "Invoice INV-2291 for milk delivery",
			BodyText:   "Dear Manager,\n\nPlease find attached the invoice for yesterday's dairy delivery (120 L whole milk, 40 L oat milk). Total due: SAR 1,840. Payment terms: 14 days.\n\nRegards,\nAccounts",
			ReceivedAt: now.Add(-5 * time.Hour), IsRead: true, HasAttachments: true,
		},
		{
			ExternalID: "demo-4", Folder: inbox.FolderInbox,
			FromName: "Layla Mansour", FromEmail: "layla.m@example.com",
			ToEmails: "gm@demo", Subject: "Complaint: cold coffee and slow service",
			BodyText:   "مرحباً،\n\nطلبت قهوة اليوم صباحاً ووصلت باردة بعد انتظار طويل. أتوقع اعتذاراً وتعويضاً مناسباً.\n\nليلى",
			ReceivedAt: now.Add(-26 * time.Hour), IsRead: false,
		},
		{
			ExternalID: "demo-5", Folder: inbox.FolderInbox,
			FromName: "HR Desk", FromEmail: "hr@internal.demo",
			ToEmails: "gm@demo", Subject: "Shift coverage needed — Friday evening",
			BodyText:   "Two baristas called in sick for Friday 16:00–23:00. Please approve overtime for Khalid and Noura or we will need agency cover.",
			ReceivedAt: now.Add(-30 * time.Hour), IsRead: true,
		},
		{
			ExternalID: "demo-6", Folder: inbox.FolderInbox,
			FromName: "City Bank Cards", FromEmail: "noreply@citybank.example",
			ToEmails: "gm@demo", Subject: "Settlement report — POS batch 17 Nov",
			BodyText:   "Your POS settlement for 17 Nov is ready. Gross: USD 4,210. Fees: USD 92. Net deposit: USD 4,118. Reply if any variance.",
			ReceivedAt: now.Add(-50 * time.Hour), IsRead: true,
		},
	}
}

func stringsJoin(in []string) string {
	if len(in) == 0 {
		return ""
	}
	out := in[0]
	for i := 1; i < len(in); i++ {
		out = fmt.Sprintf("%s, %s", out, in[i])
	}
	return out
}

var _ appinbox.DemoMailbox = (*DemoMailbox)(nil)
