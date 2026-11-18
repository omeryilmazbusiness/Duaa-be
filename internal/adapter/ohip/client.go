// Package ohip implements Oracle Hospitality Integration Platform (OHIP / OPERA Cloud)
// OAuth2 + reservations/housekeeping snapshot pulls with strict secret redaction.
package ohip

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/hotelpms"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const maxOHIPBytes = 2 << 20

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second, Transport: transport}}
}

var _ hotelpms.OHIPClient = (*Client)(nil)

func (c *Client) FetchDay(ctx context.Context, req hotelpms.OHIPFetchRequest) (hotel.PMSSnapshot, error) {
	base := strings.TrimRight(req.HostURL, "/")
	token, err := c.fetchToken(ctx, base, req.ClientID, req.ClientSecret, req.AppKey)
	if err != nil {
		return hotel.PMSSnapshot{}, err
	}

	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"x-app-key":     req.AppKey,
		"x-hotelid":     req.HotelID,
		"Accept":        "application/json",
	}

	rsvURL := base + "/rsv/v1/hotels/" + url.PathEscape(req.HotelID) + "/reservations?limit=50&arrivalStartDate=" + url.QueryEscape(req.BusinessDate)
	rsvBody, err := c.getJSON(ctx, rsvURL, headers, req.ClientSecret, req.AppKey)
	if err != nil {
		return hotel.PMSSnapshot{}, err
	}

	snap := parseReservationsPayload(rsvBody, req.BusinessDate)
	// Optional housekeeping / room inventory endpoint (best-effort across OHIP hotel profiles).
	hkURL := base + "/fof/v1/hotels/" + url.PathEscape(req.HotelID) + "/housekeeping/rooms"
	if hkBody, err := c.getJSON(ctx, hkURL, headers, req.ClientSecret, req.AppKey); err == nil {
		applyHousekeeping(&snap, hkBody)
	}
	snap.ExternalBatchRef = "ohip:" + req.HotelID + ":" + req.BusinessDate
	return snap, nil
}

func (c *Client) fetchToken(ctx context.Context, base, clientID, clientSecret, appKey string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/oauth/v1/tokens", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	basic := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	httpReq.Header.Set("Authorization", "Basic "+basic)
	httpReq.Header.Set("x-app-key", appKey)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", errx.Upstream("ohip_unreachable", "could not reach OHIP gateway")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOHIPBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errx.Upstream("ohip_auth_failed", redact("OHIP token rejected", clientSecret, appKey))
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &out) != nil || out.AccessToken == "" {
		return "", errx.Upstream("ohip_bad_token", "OHIP token response missing access_token")
	}
	return out.AccessToken, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, headers map[string]string, secrets ...string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, errx.Upstream("ohip_unreachable", "could not reach OHIP endpoint")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxOHIPBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errx.Upstream("ohip_request_failed", redact("OHIP returned HTTP "+http.StatusText(resp.StatusCode), secrets...))
	}
	return body, nil
}

func parseReservationsPayload(raw []byte, date string) hotel.PMSSnapshot {
	var env struct {
		Reservations struct {
			TotalRooms        int    `json:"totalRooms"`
			OccupiedRooms     int    `json:"occupiedRooms"`
			OutOfOrderRooms   int    `json:"outOfOrderRooms"`
			RoomRevenueMinor  int64  `json:"roomRevenueMinor"`
			FBRevenueMinor    int64  `json:"fbRevenueMinor"`
			OtherRevenueMinor int64  `json:"otherRevenueMinor"`
			Currency          string `json:"currency"`
			ReservationInfo   []struct {
				ConfirmationNo string `json:"confirmationNo"`
				ReservationID  string `json:"reservationId"`
				Status         string `json:"reservationStatus"`
				RoomStay       struct {
					ArrivalDate   string `json:"arrivalDate"`
					DepartureDate string `json:"departureDate"`
					RoomType      string `json:"roomType"`
					RateAmount    struct {
						Amount   float64 `json:"amount"`
						Currency string  `json:"currencyCode"`
					} `json:"rateAmount"`
				} `json:"roomStay"`
				GuestName string `json:"guestName"`
				Channel   string `json:"channel"`
				VIP       bool   `json:"vip"`
			} `json:"reservationInfo"`
		} `json:"reservations"`
	}
	_ = json.Unmarshal(raw, &env)
	snap := hotel.PMSSnapshot{
		BusinessDate:      date,
		Currency:          env.Reservations.Currency,
		TotalRooms:        env.Reservations.TotalRooms,
		OccupiedRooms:     env.Reservations.OccupiedRooms,
		OutOfOrderRooms:   env.Reservations.OutOfOrderRooms,
		RoomRevenueMinor:  env.Reservations.RoomRevenueMinor,
		FBRevenueMinor:    env.Reservations.FBRevenueMinor,
		OtherRevenueMinor: env.Reservations.OtherRevenueMinor,
	}
	for _, info := range env.Reservations.ReservationInfo {
		conf := info.ConfirmationNo
		if conf == "" {
			conf = info.ReservationID
		}
		cur := info.RoomStay.RateAmount.Currency
		if cur == "" {
			cur = snap.Currency
		}
		if snap.Currency == "" && cur != "" {
			snap.Currency = cur
		}
		rateMinor := int64(info.RoomStay.RateAmount.Amount * 100)
		row := hotel.ReservationRow{
			ConfirmationNo: conf,
			GuestName:      info.GuestName,
			RoomType:       info.RoomStay.RoomType,
			Status:         hotel.ReservationStatus(info.Status),
			ArrivalDate:    info.RoomStay.ArrivalDate,
			DepartureDate:  info.RoomStay.DepartureDate,
			RateMinor:      rateMinor,
			Currency:       cur,
			Channel:        info.Channel,
			VIP:            info.VIP,
		}
		snap.Reservations = append(snap.Reservations, row)
		switch strings.ToLower(info.Status) {
		case "checked_in", "inhouse", "in_house":
			snap.InHouseGuests++
		case "checked_out", "departed":
			snap.DeparturesCount++
		case "cancelled", "canceled":
			snap.CancelledCount++
		case "no_show", "noshow":
			snap.NoShowCount++
		}
		if info.RoomStay.ArrivalDate == date {
			snap.ArrivalsCount++
		}
		if info.RoomStay.DepartureDate == date {
			snap.DeparturesCount++
		}
	}
	snap.ReservationsCount = len(snap.Reservations)
	if snap.TotalRooms == 0 && len(snap.Reservations) > 0 {
		snap.TotalRooms = len(snap.Reservations)
		snap.OccupiedRooms = snap.InHouseGuests
	}
	if snap.RoomRevenueMinor == 0 {
		for _, r := range snap.Reservations {
			if r.Status != hotel.ResCancelled && r.Status != hotel.ResNoShow {
				snap.RoomRevenueMinor += r.RateMinor
			}
		}
	}
	return snap
}

func applyHousekeeping(snap *hotel.PMSSnapshot, raw []byte) {
	var env struct {
		Rooms []struct {
			RoomStatus        string `json:"roomStatus"`
			HousekeepingStatus string `json:"housekeepingStatus"`
			OutOfOrder        bool   `json:"outOfOrder"`
		} `json:"rooms"`
	}
	if json.Unmarshal(raw, &env) != nil || len(env.Rooms) == 0 {
		return
	}
	total := len(env.Rooms)
	ooo, occ, clean := 0, 0, 0
	for _, rm := range env.Rooms {
		if rm.OutOfOrder {
			ooo++
		}
		if strings.EqualFold(rm.RoomStatus, "occupied") {
			occ++
		}
		if strings.EqualFold(rm.HousekeepingStatus, "clean") || strings.EqualFold(rm.HousekeepingStatus, "inspected") {
			clean++
		}
	}
	if total > snap.TotalRooms {
		snap.TotalRooms = total
	}
	if occ > 0 {
		snap.OccupiedRooms = occ
	}
	if ooo > 0 {
		snap.OutOfOrderRooms = ooo
	}
	snap.Readiness = append(snap.Readiness, hotel.DepartmentReadiness{
		Department: "HOUSEKEEPING",
		Total:      total,
		Completed:  clean,
	})
}

func redact(msg string, secrets ...string) string {
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "[redacted]")
		}
	}
	return msg
}
