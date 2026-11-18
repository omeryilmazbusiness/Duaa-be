package httpadapter_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/bootstrap"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestAssistantTranslateMeetingsAudioAndPortfolioGrounding(t *testing.T) {
	h := newHarness(t)
	hotels, err := bootstrap.SeedPortfolioHotels(t.Context(), h.store, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	yday := now.AddDate(0, 0, -1).Format("2006-01-02")
	gmID := id.MustParse("33333333-3333-4333-8333-333333333333")

	_ = h.store.SavePMSSnapshot(t.Context(), hotel.PMSSnapshot{
		ID: id.New(), WorkspaceID: hotels.SheratonID, BusinessDate: yday, Source: hotel.SourceOPERAExport,
		Currency: "USD", TotalRooms: 100, OccupiedRooms: 60, RoomRevenueMinor: 12000_00, CapturedAt: now.Add(-24 * time.Hour), RecordedBy: gmID,
	})
	_ = h.store.SavePMSSnapshot(t.Context(), hotel.PMSSnapshot{
		ID: id.New(), WorkspaceID: hotels.SheratonID, BusinessDate: today, Source: hotel.SourceOHIPLive,
		Currency: "USD", TotalRooms: 100, OccupiedRooms: 85, RoomRevenueMinor: 19000_00, CapturedAt: now, RecordedBy: gmID,
	})

	if save := put(h.handler, "/v1/workspaces/"+cafeID+"/assistant/settings", map[string]any{
		"provider": "openai", "model": "gpt-4o-mini", "apiKey": testAIKey, "dailyLimit": 25, "enabled": true,
	}, token); save.Code != 200 {
		t.Fatalf("save ai %d %s", save.Code, save.Body.String())
	}

	// Portfolio grounding check: chat prompt receives Sheraton & portfolio revenue numbers!
	h.llm.reply = "إجمالي إيرادات الفنادق اليوم 19,000 دولار مقارنة بـ 12,000 دولار أمس."
	chat := post(h.handler, "/v1/workspaces/"+cafeID+"/assistant/chat", map[string]any{
		"locale":   "ar",
		"messages": []map[string]string{{"role": "user", "content": "ما هو إجمالي إيرادات الفنادق اليوم مقارنة بالأمس؟"}},
	}, token)
	if chat.Code != 200 {
		t.Fatalf("chat %d %s", chat.Code, chat.Body.String())
	}
	lastReq := h.llm.last()
	if !strings.Contains(lastReq.System, "Sheraton Hotel") || !strings.Contains(lastReq.System, "19000.00") {
		t.Fatalf("expected portfolio revenue in assistant system snapshot:\n%s", lastReq.System)
	}
	if strings.Contains(lastReq.System, "spoken aloud") {
		t.Fatal("typed chat must not use the voice protocol")
	}

	// AI speaker: voice turn answers in the screen language as short speech.
	voiceChat := post(h.handler, "/v1/workspaces/"+cafeID+"/assistant/chat", map[string]any{
		"locale":   "ar",
		"voice":    true,
		"messages": []map[string]string{{"role": "user", "content": "How full is Sheraton today?"}},
	}, token)
	if voiceChat.Code != 200 {
		t.Fatalf("voice chat %d %s", voiceChat.Code, voiceChat.Body.String())
	}
	voiceReq := h.llm.last()
	if !strings.Contains(voiceReq.System, "spoken aloud") || !strings.Contains(voiceReq.System, "always answer in Arabic") {
		t.Fatalf("voice chat must request spoken Arabic:\n%s", voiceReq.System)
	}

	// Bidirectional translate EN -> AR with hospitality term explanation.
	h.llm.reply = `{"translated":"متوسط السعر اليومي ارتفع 12% مقارنة بالأمس.","simpleExplanation":"ADR هو متوسط سعر الغرفة المباعة.","toneNote":""}`
	tr := post(h.handler, "/v1/workspaces/"+cafeID+"/assistant/translate", map[string]any{
		"text":       "Our ADR increased 12% over yesterday due to corporate allotments.",
		"sourceLang": "en", "targetLang": "ar", "mode": "translate",
	}, token)
	if tr.Code != 200 {
		t.Fatalf("translate %d %s", tr.Code, tr.Body.String())
	}
	var trView map[string]any
	decode(t, tr, &trView)
	if trView["simpleExplanation"] == "" {
		t.Fatalf("expected hospitality simpleExplanation: %+v", trView)
	}

	// Meeting summary with ambiguous owner flagged (AT-08).
	h.llm.reply = `{"summary":"تمت مراجعة إشغال شيراتون.","translatedTranscript":"Sheraton occupancy reviewed.","decisions":[{"text":"تحديث أسعار المجموعات","owner":"","dueAt":"","ambiguous":true}],"actionItems":[{"text":"إرسال تقرير أوبرا","owner":"","dueAt":"","ambiguous":true}]}`
	meet := post(h.handler, "/v1/workspaces/"+cafeID+"/assistant/meetings/summarize", map[string]any{
		"title":        "Sheraton Morning Sync",
		"participants": []string{"Duaa", "Front Office Lead"},
		"transcript":   "We agreed to update corporate group rates and someone will send the OPERA report.",
		"sourceLocale": "en",
		"targetLocale": "ar",
	}, token)
	if meet.Code != 201 {
		t.Fatalf("meeting %d %s", meet.Code, meet.Body.String())
	}
	listed := get(h.handler, "/v1/workspaces/"+cafeID+"/assistant/meetings", token)
	if listed.Code != 200 {
		t.Fatalf("list meetings %d", listed.Code)
	}

	// Audio transcribe + TTS speak.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("language", "en")
	fw, _ := mw.CreateFormFile("audio", "clip.webm")
	_, _ = fw.Write([]byte("WEBM-FAKE-AUDIO"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/"+cafeID+"/assistant/audio/transcribe", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("transcribe %d %s", rec.Code, rec.Body.String())
	}

	speak := post(h.handler, "/v1/workspaces/"+cafeID+"/assistant/audio/speak", map[string]any{
		"text": "مرحباً دعاء", "language": "ar",
	}, token)
	if speak.Code != 200 || speak.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("speak %d ct=%s", speak.Code, speak.Header().Get("Content-Type"))
	}
}
