import React, { useEffect, useRef, useState } from "react";
import { api } from "../lib/api";
import { money } from "../lib/format";

// หน้าสำหรับ OBS: โหลดเฉพาะรายการยืนยันแล้ว แล้วแสดงแจ้งเตือนจากคิวทีละรายการ
export default function OverlayPage() {
  const [alert, setAlert] = useState(null),
    [error, setError] = useState("");
  // queue เก็บรายการรอแสดง, seen จำ ID ที่รู้จักแล้ว, ready บอกว่าโหลดครั้งแรกสำเร็จ
  // ทั้งสามเป็น ref จึงคงค่าระหว่าง render โดยไม่ทำให้ UI render ทุกครั้งที่แก้คิว
  const queue = useRef([]),
    seen = useRef(new Set()),
    ready = useRef(false);
  useEffect(() => {
    // ทำทั้ง html และ body ให้พื้นหลังโปร่งใสสำหรับ OBS Browser Source
    document.body.classList.add("overlay-page");
    document.documentElement.classList.add("overlay-page");
    let stopped = false;
    // ตรวจรายการยืนยันทุกครั้งที่ poll; ถ้าหน้าถูกปิดแล้วไม่แก้ state ต่อ
    async function poll() {
      try {
        const items = await api("donations?confirmed=1");
        if (stopped) return;
        // ครั้งแรกจำประวัติอย่างเดียว ไม่เล่นย้อนหลัง; ครั้งถัดไปกลับลำดับให้เก่ากว่าเข้าคิวก่อน
        // API ส่งเฉพาะ 20 รายการล่าสุด จึงอาจพลาดรายการหากมีรายการถี่มากระหว่าง poll
        if (ready.current)
          for (const d of [...items].reverse())
            if (!seen.current.has(d.id)) queue.current.push(d);
        items.forEach((d) => seen.current.add(d.id));
        ready.current = true;
        setError("");
      } catch {
        if (!stopped) setError("เชื่อมต่อระบบไม่ได้");
      }
    }
    poll();
    // ดึงข้อมูลทุก 1 วินาที และหยิบรายการจากคิวแสดงทุก 8 วินาที
    const p = setInterval(poll, 1000),
      s = setInterval(() => {
        if (queue.current.length) setAlert(queue.current.shift());
      }, 8000);
    return () => {
      // cleanup ตอน component ถูกถอด: หยุด timer และคืน CSS ของหน้าเว็บปกติ
      stopped = true;
      clearInterval(p);
      clearInterval(s);
      document.body.classList.remove("overlay-page");
      document.documentElement.classList.remove("overlay-page");
    };
  }, []);
  useEffect(() => {
    if (!alert) return;
    // ซ่อน alert หลัง 7 วินาที จึงมีช่วงว่างประมาณ 1 วินาทีก่อนรายการถัดไป
    const t = setTimeout(() => setAlert(null), 7000);
    return () => clearTimeout(t);
  }, [alert]);
  return (
    <div className="obs-root">
      {error && <div className="error">{error}</div>}
      {alert && (
        <div className="stream-alert">
          <div>♥</div>
          <h1>{alert.name}</h1>
          <strong>โดเนท ฿{money(alert.amount)}</strong>
          <p>{alert.message || "ส่งกำลังใจให้สตรีมนี้ 💜"}</p>
          <small>DEMO · โดเนทจำลอง</small>
        </div>
      )}
    </div>
  );
}
