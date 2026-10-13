import React, { useEffect, useState } from "react";
import { api } from "./lib/api";
import DonationPage from "./pages/DonationPage";
import DashboardPage from "./pages/DashboardPage";
import PaymentModal from "./components/PaymentModal";
import SubmissionReceipt from "./components/SubmissionReceipt";

// App ดูแล layout, state ร่วม และคำขอ API; หน้าจอแต่ละหน้าอยู่ใน pages/
export default function App() {
  // profile/items คือข้อมูล backend; tab คือหน้าที่เลือก; amount/name/anonymous/message คือค่าฟอร์ม
  // busy/error/loadError คือสถานะคำขอ; order เปิด QR; receipt เปิดผลส่งหลักฐาน; copied คือผลคัดลอก URL
  const [profile, setProfile] = useState(null),
    [items, setItems] = useState([]),
    [tab, setTab] = useState(
      location.pathname === "/dashboard" ? "dashboard" : "donate",
    ),
    [amount, setAmount] = useState(50),
    [name, setName] = useState(""),
    [anonymous, setAnonymous] = useState(false),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [loadError, setLoadError] = useState(""),
    [receipt, setReceipt] = useState(null),
    [copied, setCopied] = useState(false),
    [order, setOrder] = useState(null);
  // โหลดโปรไฟล์กับประวัติพร้อมกัน เพื่อลดเวลารอและอัปเดตยอดกับรายการ
  async function refresh() {
    try {
      const [p, d] = await Promise.all([api("profile"), api("donations")]);
      setProfile(p);
      setItems(d);
      setLoadError("");
    } catch {
      setLoadError("เชื่อมต่อ backend ไม่ได้ กรุณาลองใหม่");
    }
  }
  useEffect(() => {
    refresh();
    // โหลดทันทีเมื่อเปิดหน้า แล้วอัปเดตทุก 3 วินาที; cleanup หยุด timer เมื่อปิดหน้า
    const t = setInterval(refresh, 3000);
    return () => clearInterval(t);
  }, []);
  // หยุดการ submit แบบเปลี่ยนหน้า แล้วสร้างรายการรอแนบสลิป
  // แปลง amount เป็น Number เพราะค่าจาก input อาจเป็น string
  async function donate(e) {
    e.preventDefault();
    setError("");
    // ไม่ยอมรับชื่อที่มีแต่ช่องว่าง เว้นแต่ผู้ชมเลือกไม่ประสงค์ออกนาม
    if (!anonymous && !name.trim()) {
      setError("กรุณากรอกชื่อที่แสดงบนสตรีม หรือเลือกไม่ประสงค์ออกนาม");
      return;
    }
    setBusy(true);
    try {
      const d = await api("donations", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          amount: Number(amount),
          name: anonymous ? "" : name.trim(),
          anonymous,
          message,
        }),
      });
      // เปิด PaymentModal ด้วยข้อมูลตอบกลับ ซึ่งมี donation, uploadToken และ qrPayload
      // ข้อมูลรายการค้างเก็บใน state เท่านั้น จึงหายเมื่อรีเฟรชหน้า
      setOrder(d);
      await refresh();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  // คัดลอก URL overlay จาก origin ปัจจุบัน ให้ใช้ได้ทั้งเครื่องพัฒนาและ host ที่รันจริง
  async function copy() {
    try {
      await navigator.clipboard.writeText(location.origin + "/overlay");
      setCopied(true);
    } catch {
      setError("กรุณาคัดลอก URL ด้วยตนเอง");
    }
  }
  return (
    <>
      <div className="demo-bar">
        DEMO MODE — ทดลองส่งโดเนท ไม่มีการหักเงินจริง
      </div>
      {/* ปุ่มเมนูเปลี่ยน tab ใน state โดยไม่ได้เปลี่ยน URL; ตอนเปิด /dashboard ใช้ค่านั้นเลือก tab เริ่มต้น */}
      <header>
        <a className="logo" href="/">
          pulse<span>♥</span>
        </a>
        <nav>
          <button
            className={tab === "donate" ? "active" : ""}
            onClick={() => setTab("donate")}
          >
            หน้าโดเนท
          </button>
          <button
            className={tab === "dashboard" ? "active" : ""}
            onClick={() => setTab("dashboard")}
          >
            แดชบอร์ดสตรีมเมอร์
          </button>
        </nav>
        <small>SANDBOX</small>
      </header>
      {/* แสดงข้อผิดพลาด/สถานะโหลดก่อน แล้วเลือกหน้าโดเนทหรือแดชบอร์ดจาก tab */}
      <main>
        {loadError && (
          <div className="error" role="alert">
            {loadError} <button onClick={refresh}>ลองใหม่</button>
          </div>
        )}
        {!profile && !loadError && <p>กำลังโหลด…</p>}
        {profile &&
          (tab === "donate" ? (
            <DonationPage
              profile={profile}
              items={items}
              amount={amount}
              setAmount={setAmount}
              name={name}
              setName={setName}
              anonymous={anonymous}
              setAnonymous={setAnonymous}
              message={message}
              setMessage={setMessage}
              busy={busy}
              error={error}
              donate={donate}
            />
          ) : (
            <DashboardPage
              profile={profile}
              items={items}
              copied={copied}
              copy={copy}
              error={error}
              refresh={refresh}
            />
          ))}
      </main>
      <footer>
        <span className="logo">
          pulse<span>♥</span>
        </span>
        <span>A little support. A lot of inspiration.</span>
        <small>React + Go · Streamer demo</small>
      </footer>
      {/* แสดงขั้น QR เมื่อมี order; ปิดหน้าต่างไม่ได้ลบรายการที่ backend สร้างไว้ */}
      {order && (
        <PaymentModal
          order={order}
          onClose={() => setOrder(null)}
          onComplete={(d) => {
            // อัปโหลดสำเร็จ: ปิด QR เปิดผลรอตรวจ ล้างข้อความฟอร์ม และโหลดรายการใหม่
            setOrder(null);
            setReceipt(d);
            setMessage("");
            refresh();
          }}
        />
      )}
      {/* ผลสำเร็จของการส่งไฟล์เท่านั้น ยอดและ overlay ยังรอการยืนยันในแดชบอร์ด */}
      {receipt && (
        <SubmissionReceipt receipt={receipt} onClose={() => setReceipt(null)} />
      )}
    </>
  );
}
