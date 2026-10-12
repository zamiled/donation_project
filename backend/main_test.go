package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
)

func TestDonationFlowAndPersistence(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "donations.json")}
	h := s.handler()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		return w
	}
	for _, body := range []string{`{"projectId":"old","amount":100}`, `{"amount":0}`, `{"amount":1000001}`, `{"amount":1.5}`, `{"amount":100,"id":"forged"}`} {
		if w := request("POST", "/api/donations", body); w.Code != 400 {
			t.Fatalf("invalid input returned %d: %s", w.Code, body)
		}
	}
	w := request("POST", "/api/donations", `{"amount":300,"message":"ส่งกำลังใจ"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var d Donation
	json.Unmarshal(w.Body.Bytes(), &d)
	if d.ID == "" || d.Name != "ผู้บริจาคไม่ประสงค์ออกนาม" {
		t.Fatalf("bad receipt: %+v", d)
	}
	w = request("GET", "/api/profile", "")
	var p struct {
		Total int64 `json:"total"`
		Count int   `json:"count"`
	}
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.Total != 300 || p.Count != 1 {
		t.Fatalf("totals: %+v", p)
	}
	restored := &Store{path: s.path}
	if err := restored.load(); err != nil {
		t.Fatal(err)
	}
	if len(restored.donations) != 1 || restored.donations[0].Amount != 300 {
		t.Fatal("donation did not persist")
	}
	if w := request("GET", "/api/donations", ""); w.Code != 200 {
		t.Fatal("history unavailable")
	}
}
func TestConcurrentDonations(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "data.json")}
	h := s.handler()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/donations", bytes.NewBufferString(`{"amount":100}`)))
			if w.Code != 201 {
				t.Errorf("status %d", w.Code)
			}
		}()
	}
	wg.Wait()
	if len(s.donations) != 10 {
		t.Fatalf("lost donations: %d", len(s.donations))
	}
	restored := &Store{path: s.path}
	if err := restored.load(); err != nil {
		t.Fatal(err)
	}
	if len(restored.donations) != 10 {
		t.Fatal("persisted donations missing")
	}
}
