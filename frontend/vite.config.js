import { defineConfig } from "vite";
// ส่งคำขอ /api จาก frontend ไป Go เพื่อใช้ origin เดียวกันขณะพัฒนา
// preview ใช้ proxy เดียวกันสำหรับทดสอบ build; production ต้องตั้ง reverse proxy ของ host เอง
export default defineConfig({
  server: { proxy: { "/api": "http://127.0.0.1:8080" } },
  preview: { proxy: { "/api": "http://127.0.0.1:8080" } },
});
