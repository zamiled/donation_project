package main

import (
	"encoding/json"
	"net/http"
)

// ขั้นที่สาม: รับผลตรวจด้วยตัวเอง approve/reject จากแดชบอร์ด
// approve หมายถึงยืนยันใน demo ไม่ใช่ผลตรวจจากธนาคารหรือบริการตรวจสลิป
func (s *Store) reviewSlip(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Decision string `json:"decision"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if e := json.NewDecoder(r.Body).Decode(&input); e != nil || (input.Decision != "approve" && input.Decision != "reject") {
		write(w, 400, map[string]string{"error": "ระบุ approve หรือ reject"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.donations {
		d := &s.donations[i]
		if d.ID != r.PathValue("id") {
			continue
		}
		// ตรวจลำดับสถานะเพื่อห้ามยืนยันก่อนส่งสลิป และห้ามตรวจรายการเดิมซ้ำ
		if d.Status != "pending_review" || d.Slip == "" {
			write(w, 409, map[string]string{"error": "ต้องส่งสลิปก่อนตรวจสอบ"})
			return
		}
		// หากบันทึกผลตรวจล้มเหลว ให้คืนสถานะเดิมเพื่อไม่ให้ยอดในหน่วยความจำผิด
		old := d.Status
		if input.Decision == "approve" {
			d.Status = "demo_confirmed"
		} else {
			d.Status = "rejected"
		}
		if e := s.save(); e != nil {
			d.Status = old
			write(w, 500, map[string]string{"error": "บันทึกผลไม่สำเร็จ"})
			return
		}
		write(w, 200, d)
		return
	}
	write(w, 404, map[string]string{"error": "ไม่พบรายการ"})
}
