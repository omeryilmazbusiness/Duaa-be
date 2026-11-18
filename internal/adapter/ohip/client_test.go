package ohip_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dua-os/dua-os-be/internal/adapter/ohip"
	"github.com/dua-os/dua-os-be/internal/app/hotelpms"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestFetchDayParsesOHIPTokenReservationsAndHousekeeping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/oauth/v1/tokens":
			if r.Header.Get("x-app-key") != "app-secret-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-123"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rsv/v1/hotels/SHERATON/reservations"):
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"reservations": map[string]any{
					"currency":          "USD",
					"roomRevenueMinor":  24500_00,
					"fbRevenueMinor":    4200_00,
					"otherRevenueMinor": 800_00,
					"reservationInfo": []map[string]any{
						{
							"confirmationNo":    "OP-1001",
							"reservationStatus": "checked_in",
							"guestName":         "Tareq Mansour",
							"channel":           "Direct",
							"vip":               true,
							"roomStay": map[string]any{
								"arrivalDate":   "2026-11-18",
								"departureDate": "2026-11-20",
								"roomType":      "EXEC_SUITE",
								"rateAmount":    map[string]any{"amount": 290.00, "currencyCode": "USD"},
							},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/fof/v1/hotels/SHERATON/housekeeping/rooms":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rooms": []map[string]any{
					{"roomStatus": "occupied", "housekeepingStatus": "clean", "outOfOrder": false},
					{"roomStatus": "occupied", "housekeepingStatus": "inspected", "outOfOrder": false},
					{"roomStatus": "vacant", "housekeepingStatus": "dirty", "outOfOrder": true},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := ohip.NewClient()
	snap, err := client.FetchDay(t.Context(), hotelpms.OHIPFetchRequest{
		HostURL:      srv.URL,
		HotelID:      "SHERATON",
		ClientID:     "cid",
		ClientSecret: "super-secret-xyz",
		AppKey:       "app-secret-key",
		BusinessDate: "2026-11-18",
		RecorderID:   id.New(),
		WorkspaceID:  id.New(),
	})
	if err != nil {
		t.Fatalf("FetchDay err=%v", err)
	}
	if snap.TotalRooms != 3 || snap.OccupiedRooms != 2 || snap.OutOfOrderRooms != 1 {
		t.Fatalf("rooms=%+v", snap)
	}
	if snap.RoomRevenueMinor != 24500_00 || snap.ArrivalsCount != 1 || len(snap.Reservations) != 1 {
		t.Fatalf("snap=%+v", snap)
	}
	if len(snap.Readiness) != 1 || snap.Readiness[0].Completed != 2 {
		t.Fatalf("readiness=%+v", snap.Readiness)
	}
}

func TestFetchDayRedactsSecretsOnAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("bad secret-token-999"))
	}))
	defer srv.Close()

	client := ohip.NewClient()
	_, err := client.FetchDay(t.Context(), hotelpms.OHIPFetchRequest{
		HostURL:      srv.URL,
		HotelID:      "SHERATON",
		ClientID:     "cid",
		ClientSecret: "secret-token-999",
		AppKey:       "appkey-888",
		BusinessDate: "2026-11-18",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret-token-999") || strings.Contains(err.Error(), "appkey-888") {
		t.Fatalf("secret leaked in error: %v", err)
	}
}
