package main

import (
	"net/http"
)

// ตรวจว่า backend ตอบคำขอได้และระบุว่าระบบอยู่ในโหมดทดลอง
func (s *Store) health(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]string{"status": "ok", "mode": "demo"})
}
