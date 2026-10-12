package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSlipReviewFlow(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "data.json")}
	h := s.handler()
	req := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		return w
	}
	w := req("POST", "/api/donations", `{"amount":125,"name":"viewer"}`)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var order struct {
		Donation Donation `json:"donation"`
		Token    string   `json:"uploadToken"`
		QR       string   `json:"qrPayload"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &order); e != nil {
		t.Fatal(e)
	}
	if order.Donation.Status != "awaiting_slip" || !bytes.Contains([]byte(order.QR), []byte("125.00")) || order.Token == "" {
		t.Fatalf("invalid order %+v", order)
	}
	id := order.Donation.ID
	if w := req("POST", "/api/donations/"+id+"/review", `{"decision":"approve"}`); w.Code != 409 {
		t.Fatal("approved without slip")
	}
	var profile struct {
		Total int64 `json:"total"`
	}
	json.Unmarshal(req("GET", "/api/profile", "").Body.Bytes(), &profile)
	if profile.Total != 0 {
		t.Fatal("unpaid order counted")
	}
	upload := func(data []byte, token string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		m := multipart.NewWriter(&b)
		f, _ := m.CreateFormFile("slip", "slip.png")
		f.Write(data)
		m.Close()
		r := httptest.NewRequest("POST", "/api/donations/"+id+"/slip", &b)
		r.Header.Set("Content-Type", m.FormDataContentType())
		r.Header.Set("X-Upload-Token", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := upload([]byte("not an image"), order.Token); w.Code != 400 {
		t.Fatal("accepted fake image")
	}
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if w := upload(img.Bytes(), "wrong"); w.Code != 403 {
		t.Fatal("accepted wrong upload token")
	}
	if w := upload(img.Bytes(), order.Token); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if s.donations[0].Status != "pending_review" {
		t.Fatal("upload verified payment")
	}
	if w := req("GET", "/api/donations?confirmed=1", ""); w.Body.String() != "[]\n" {
		t.Fatal("pending alert leaked", w.Body.String())
	}
	restored := &Store{path: s.path}
	if e := restored.load(); e != nil {
		t.Fatal(e)
	}
	if restored.donations[0].Token != order.Token {
		t.Fatal("token persistence failed")
	}
	if w := req("GET", "/api/donations/"+id+"/slip", ""); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), img.Bytes()) {
		t.Fatal("slip retrieval failed")
	}
	if w := req("POST", "/api/donations/"+id+"/review", `{"decision":"approve"}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	json.Unmarshal(req("GET", "/api/profile", "").Body.Bytes(), &profile)
	if profile.Total != 125 {
		t.Fatal("approved total incorrect")
	}
	if w := req("POST", "/api/donations/"+id+"/review", `{"decision":"approve"}`); w.Code != 409 {
		t.Fatal("duplicate approval accepted")
	}
	if bytes.Contains(req("GET", "/api/donations", "").Body.Bytes(), []byte(order.Token)) {
		t.Fatal("token leaked")
	}
}
