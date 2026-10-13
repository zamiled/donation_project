package main

import (
	"encoding/json"
	"net/http"
)

// write รวมวิธีตอบ API: ตั้งชนิด JSON, HTTP status และเข้ารหัส response
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
