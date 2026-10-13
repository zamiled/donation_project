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

// ทดสอบสร้างรายการ ตรวจข้อมูลที่ไม่อนุญาต ยอดที่ยังไม่ยืนยัน และการกู้ข้อมูลจากไฟล์
func TestDonationFlowAndPersistence(t *testing.T) {
	// ใช้โฟลเดอร์ชั่วคราวของ test เพื่อไม่แตะข้อมูลโดเนทจริงในเครื่อง
	s := &Store{path: filepath.Join(t.TempDir(), "donations.json")}
	h := s.handler()
	// สร้าง HTTP request/response ในหน่วยความจำ ไม่ต้องเปิดพอร์ตหรือรัน server จริง
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		return w
	}
	// ข้อมูลผิดต้องตอบ 400: ฟิลด์เก่า ยอดศูนย์ ยอดเกินเพดาน ทศนิยม และ ID ที่ผู้ใช้ปลอม
	for _, body := range []string{`{"projectId":"old","amount":100}`, `{"amount":0,"anonymous":true}`, `{"amount":1000001,"anonymous":true}`, `{"amount":1.5,"anonymous":true}`, `{"amount":100,"id":"forged"}`} {
		if w := request("POST", "/api/donations", body); w.Code != 400 {
			t.Fatalf("invalid input returned %d: %s", w.Code, body)
		}
	}
	w := request("POST", "/api/donations", `{"amount":300,"anonymous":true,"message":"ส่งกำลังใจ"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var order struct {
		Donation Donation `json:"donation"`
	}
	json.Unmarshal(w.Body.Bytes(), &order)
	d := order.Donation
	if d.ID == "" || d.Name != "ผู้บริจาคไม่ประสงค์ออกนาม" {
		t.Fatalf("bad receipt: %+v", d)
	}
	// สร้าง QR อย่างเดียวยังไม่ปรากฏในแดชบอร์ด ทั้ง total และ count ต้องเป็นศูนย์
	w = request("GET", "/api/profile", "")
	var p struct {
		Total int64 `json:"total"`
		Count int   `json:"count"`
	}
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.Total != 0 || p.Count != 0 {
		t.Fatalf("totals: %+v", p)
	}
	// สร้าง Store ใหม่และโหลดไฟล์ เพื่อยืนยันว่าข้อมูลอยู่หลัง restart ไม่ใช่แค่ใน memory
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

// ส่งพร้อมกัน 10 คำขอเพื่อตรวจ mutex และยืนยันว่าไม่มีรายการสูญหายทั้ง memory และไฟล์
func TestConcurrentDonations(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "data.json")}
	h := s.handler()
	// WaitGroup รอ goroutine ทุกตัวให้จบก่อนตรวจจำนวนรายการ
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/donations", bytes.NewBufferString(`{"amount":100,"anonymous":true}`)))
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

// ต้องมีชื่อจริงที่ไม่ใช่ช่องว่าง หรือเลือก anonymous อย่างชัดเจน และไม่เก็บชื่อที่ซ่อนไว้
func TestDonationNameOrAnonymous(t *testing.T) {
	cases := []struct {
		name, body, wantName string
		status               int
	}{
		{"missing name", `{"amount":50}`, "", 400},
		{"empty name", `{"amount":50,"name":""}`, "", 400},
		{"whitespace name", `{"amount":50,"name":"  \t "}`, "", 400},
		{"anonymous false needs name", `{"amount":50,"anonymous":false}`, "", 400},
		{"named", `{"amount":50,"name":"  ผู้ชมหนึ่ง  "}`, "ผู้ชมหนึ่ง", 201},
		{"explicit anonymous", `{"amount":50,"anonymous":true}`, "ผู้บริจาคไม่ประสงค์ออกนาม", 201},
		{"anonymous hides supplied name", `{"amount":50,"anonymous":true,"name":"ชื่อที่ไม่ต้องเผยแพร่"}`, "ผู้บริจาคไม่ประสงค์ออกนาม", 201},
		{"invalid anonymous type", `{"amount":50,"anonymous":"true"}`, "", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Store{path: filepath.Join(t.TempDir(), "donations.json")}
			w := httptest.NewRecorder()
			s.handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/donations", bytes.NewBufferString(tc.body)))
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 400 {
				if len(s.donations) != 0 {
					t.Fatal("invalid name persisted")
				}
				return
			}
			var order struct {
				Donation Donation `json:"donation"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &order); err != nil {
				t.Fatal(err)
			}
			if order.Donation.Name != tc.wantName {
				t.Fatalf("name %q, want %q", order.Donation.Name, tc.wantName)
			}
			restored := &Store{path: s.path}
			if err := restored.load(); err != nil {
				t.Fatal(err)
			}
			if restored.donations[0].Name != tc.wantName {
				t.Fatal("incorrect persisted name")
			}
		})
	}
}

// QR ที่ยังไม่ส่งสลิปต้องไม่แสดง ไม่เพิ่มจำนวน และไม่เบียดรายการจริงออกจาก 20 รายการล่าสุด
func TestDashboardHidesUnsubmittedDonations(t *testing.T) {
	s := &Store{donations: []Donation{
		{ID: "legacy", Status: "", Amount: 10},
		{ID: "approved", Status: "demo_confirmed", Amount: 20},
		{ID: "pending", Status: "pending_review", Amount: 30},
		{ID: "rejected", Status: "rejected", Amount: 40},
	}}
	for i := 0; i < 25; i++ {
		s.donations = append(s.donations, Donation{Status: "awaiting_slip", Amount: 100})
	}
	for _, tc := range []struct {
		path string
		ids  []string
	}{
		{"/api/donations", []string{"rejected", "pending", "approved", "legacy"}},
		{"/api/donations?confirmed=1", []string{"approved", "legacy"}},
	} {
		w := httptest.NewRecorder()
		s.handler().ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		var items []Donation
		if e := json.Unmarshal(w.Body.Bytes(), &items); e != nil {
			t.Fatal(e)
		}
		if len(items) != len(tc.ids) {
			t.Fatalf("%s: got %d items, want %d", tc.path, len(items), len(tc.ids))
		}
		for i, d := range items {
			if d.ID != tc.ids[i] {
				t.Fatalf("unexpected dashboard item: %+v", d)
			}
		}
	}
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/profile", nil))
	var profile struct {
		Total int64 `json:"total"`
		Count int   `json:"count"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &profile); e != nil {
		t.Fatal(e)
	}
	if profile.Total != 30 || profile.Count != 4 {
		t.Fatalf("incorrect dashboard stats: %+v", profile)
	}
}
