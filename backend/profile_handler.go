package main

import (
	"net/http"
)

// คืนโปรไฟล์ตัวอย่างและยอดรวมเฉพาะรายการยืนยันแล้ว
// count นับทุกสถานะ รวมรายการที่ยังรอแนบสลิปหรือรอตรวจ
func (s *Store) profile(w http.ResponseWriter, r *http.Request) {
	// ล็อกข้อมูลร่วมและใช้ defer ปลดล็อกเสมอเมื่อ handler จบ
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int64
	for _, d := range s.donations {
		if d.Status == "demo_confirmed" || d.Status == "" {
			total += d.Amount
		}
	}
	write(w, 200, map[string]any{"name": "YOUR STREAM", "bio": "ขอบคุณที่แวะมาดูสตรีมและเป็นกำลังใจให้กัน 💜", "goal": 10000, "goalTitle": "อัปเกรดไมค์สำหรับสตรีม", "total": total, "count": len(s.donations)})
}
