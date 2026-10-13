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

// ทดสอบ flow QR → อัปโหลด → รอตรวจ → ยืนยัน และข้อห้ามสำคัญตลอด flow
func TestSlipReviewFlow(t *testing.T) {
	// ใช้ temporary directory ให้ไฟล์ JSON/สลิปใน test แยกจากข้อมูลแอป
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
	// อ่านผลสร้างรายการ: ยอดใน QR ต้องตรง สถานะต้องรอสลิป และต้องได้ token อัปโหลด
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
	// ห้าม approve ก่อนมีสลิป และยอดที่รอตรวจต้องยังไม่เข้ายอดรวม
	id := order.Donation.ID
	if w := req("POST", "/api/donations/"+id+"/review", `{"decision":"approve"}`); w.Code != 409 {
		t.Fatal("approved without slip")
	}
	var profile struct {
		Total int64 `json:"total"`
		Count int   `json:"count"`
	}
	json.Unmarshal(req("GET", "/api/profile", "").Body.Bytes(), &profile)
	if profile.Total != 0 || profile.Count != 0 {
		t.Fatal("unpaid order counted")
	}
	if w := req("GET", "/api/donations", ""); w.Body.String() != "[]\n" {
		t.Fatal("QR-only donation appeared on dashboard", w.Body.String())
	}
	// จำลอง multipart/form-data พร้อมไฟล์และ header token เหมือน browser ส่งจริง
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
	// ปฏิเสธไฟล์ที่ไม่ใช่ภาพ แม้ตั้งชื่อเป็น slip.png
	if w := upload([]byte("not an image"), order.Token); w.Code != 400 {
		t.Fatal("accepted fake image")
	}
	// สร้างภาพ PNG ขนาดเล็กจริงสำหรับทดสอบ โดยไม่ต้องใช้สลิปหรือข้อมูลส่วนตัว
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	// ไฟล์ถูกต้องแต่ token ไม่ตรงต้องไม่มีสิทธิ์อัปโหลด
	if w := upload(img.Bytes(), "wrong"); w.Code != 403 {
		t.Fatal("accepted wrong upload token")
	}
	if w := upload(img.Bytes(), order.Token); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var dashboard []Donation
	if e := json.Unmarshal(req("GET", "/api/donations", "").Body.Bytes(), &dashboard); e != nil {
		t.Fatal(e)
	}
	if len(dashboard) != 1 || dashboard[0].Status != "pending_review" {
		t.Fatal("submitted slip missing from dashboard")
	}
	json.Unmarshal(req("GET", "/api/profile", "").Body.Bytes(), &profile)
	if profile.Count != 1 || profile.Total != 0 {
		t.Fatal("pending slip has incorrect stats")
	}
	// อัปโหลดสำเร็จต้องรอตรวจ และห้ามปรากฏในรายการสำหรับ overlay
	if s.donations[0].Status != "pending_review" {
		t.Fatal("upload verified payment")
	}
	if w := req("GET", "/api/donations?confirmed=1", ""); w.Body.String() != "[]\n" {
		t.Fatal("pending alert leaked", w.Body.String())
	}
	// ตรวจว่า token ถูกเก็บสำหรับกู้ระบบ และภาพหลักฐานเปิดอ่านได้
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
	// approve แล้วต้องนับยอดครั้งเดียว; การ approve ซ้ำต้องตอบ 409
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
	// ประวัติสาธารณะต้องไม่เปิดเผย token อัปโหลด
	if bytes.Contains(req("GET", "/api/donations", "").Body.Bytes(), []byte(order.Token)) {
		t.Fatal("token leaked")
	}
}
