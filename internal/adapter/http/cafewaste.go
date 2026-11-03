package httpadapter

import (
	"net/http"

	appcafewaste "github.com/dua-os/dua-os-be/internal/app/cafewaste"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

func (a API) wasteGate(w http.ResponseWriter, r *http.Request) (*appcafewaste.Service, bool) {
	if a.Waste == nil {
		httpx.WriteError(w, errx.New("waste_unavailable", "waste is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxQRBody)
	return a.Waste, true
}

func (a API) getCafeWaste(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.wasteGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Get(r.Context(), ac, r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (a API) recordCafeWaste(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.wasteGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		BusinessDate string `json:"businessDate"`
		Station      string `json:"station"`
		Item         string `json:"item"`
		Reason       string `json:"reason"`
		Note         string `json:"note"`
		RecipeID     string `json:"recipeId"`
		Grams        int64  `json:"grams"`
		Cost         int64  `json:"cost"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Record(r.Context(), ac, body.BusinessDate, body.Station, body.Item, body.Reason, body.Note, body.RecipeID, body.Grams, body.Cost)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}

func (a API) putCafeRecipes(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.wasteGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		Recipes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Station string `json:"station"`
			Lines   []struct {
				Title string `json:"title"`
				Grams int64  `json:"grams"`
			} `json:"lines"`
		} `json:"recipes"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := make([]appcafewaste.RecipeDraft, 0, len(body.Recipes))
	for _, rec := range body.Recipes {
		lines := make([]appcafewaste.RecipeLineView, 0, len(rec.Lines))
		for _, line := range rec.Lines {
			lines = append(lines, appcafewaste.RecipeLineView{Title: line.Title, Grams: line.Grams})
		}
		in = append(in, appcafewaste.RecipeDraft{ID: rec.ID, Name: rec.Name, Station: rec.Station, Lines: lines})
	}
	view, err := svc.SaveRecipes(r.Context(), ac, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
