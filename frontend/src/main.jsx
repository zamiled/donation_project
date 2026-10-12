// ไฟล์หลักของ frontend: helper เรียก API และ component สำหรับผู้ชม แดชบอร์ด และ OBS
// useState เก็บค่าที่เปลี่ยนแล้วต้องวาดหน้าจอใหม่; useRef เก็บข้อมูลข้าม render โดยไม่วาดใหม่
import React, { useEffect, useRef, useState } from "react";
import {
  // mount React ลง #root ใน index.html; /overlay แสดงเฉพาะแจ้งเตือน ส่วน path อื่นใช้ App
  createRoot,
} from "react-dom/client";
import "./style.css";
import QRCode from "qrcode";
// จัดรูปแบบจำนวนเงินด้วยรูปแบบตัวเลขไทย เช่น 1000 → 1,000; เครื่องหมาย ฿ ใส่ใน JSX
const money = (n) => new Intl.NumberFormat("th-TH").format(n);
// เรียก backend ผ่าน /api ที่ Vite proxy ส่งต่อไป Go
// รับ options สำหรับ method/headers/body และโยน error ให้แต่ละหน้าจัดการเอง
async function api(path, options) {
  const r = await fetch("/api/" + path, options),
    d = await r.json();
  if (!r.ok) throw Error(d.error || "เชื่อมต่อไม่สำเร็จ");
  return d;
}
// หน้าสำหรับ OBS: โหลดเฉพาะรายการยืนยันแล้ว แล้วแสดงแจ้งเตือนจากคิวทีละรายการ
function Overlay() {
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
// แปลงสถานะจาก backend ให้เป็นข้อความภาษาไทย โดยไม่เปลี่ยนค่าที่เก็บจริง
const statuses = {
  awaiting_slip: "รอแนบสลิป",
  pending_review: "รอตรวจสอบ",
  demo_confirmed: "ยืนยันแล้ว (ทดลอง)",
  rejected: "ไม่ผ่าน",
};
// ใช้ component เดียวกันทั้งประวัติผู้ชมและแดชบอร์ด
// onReview ส่งมาเฉพาะแดชบอร์ด เพื่อเปิดลิงก์สลิปและปุ่มตรวจหลักฐาน
function History({ items, onReview }) {
  const [busy, setBusy] = useState(""),
    [error, setError] = useState("");
  // ส่งผล approve/reject แล้วเรียก callback โหลดข้อมูลใหม่
  // busy เก็บ ID ที่กำลังส่งเพื่อปิดปุ่มของรายการนั้น ส่วน error ใช้แสดงผลล้มเหลว
  async function review(id, decision) {
    setBusy(id);
    setError("");
    try {
      await api("donations/" + id + "/review", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision }),
      });
      await onReview();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy("");
    }
  }
  return (
    <>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {/* มีรายการให้แสดงรายการตามลำดับที่ API ส่งมา; ถ้าไม่มีให้แสดงสถานะว่าง */}
      {items.length ? (
        <div className="history">
          {items.map((d) => (
            <article key={d.id}>
              <span className="donor-icon">♥</span>
              <div>
                <strong>{d.name}</strong>
                {/* แปลงเวลา UTC จาก backend เป็นเวลาไทย Asia/Bangkok */}
                <small>
                  {new Date(d.createdAt).toLocaleString("th-TH", {
                    timeZone: "Asia/Bangkok",
                    dateStyle: "short",
                    timeStyle: "short",
                  })}
                </small>
                {d.message && <p>{d.message}</p>}
                <small className={"status " + d.status}>
                  {statuses[d.status] || "โดเนททดลองเดิม"}
                </small>
                {onReview && d.slip && (
                  <a
                    className="slip-link"
                    href={d.slip}
                    target="_blank"
                    rel="noreferrer"
                  >
                    ดูสลิป ↗
                  </a>
                )}
                {/* เปิดปุ่มตรวจเฉพาะรายการรอตรวจ; backend ตรวจสถานะซ้ำอีกชั้น */}
                {onReview && d.status === "pending_review" && (
                  <div className="review-buttons">
                    <button
                      disabled={busy === d.id}
                      onClick={() => review(d.id, "approve")}
                    >
                      ยืนยันหลักฐาน (ทดลอง)
                    </button>
                    <button
                      disabled={busy === d.id}
                      onClick={() => review(d.id, "reject")}
                    >
                      ไม่ผ่าน
                    </button>
                  </div>
                )}
              </div>
              <b>฿{money(d.amount)}</b>
            </article>
          ))}
        </div>
      ) : (
        <div className="empty">
          <span>♡</span>
          <p>ยังไม่มีโดเนท</p>
          <small>ส่งกำลังใจแรกให้สตรีมนี้กัน</small>
        </div>
      )}
    </>
  );
}
// หน้าต่างขั้น QR และสลิป: order มาจาก backend จึงยึดยอดและ ID ของรายการนั้น
// onClose กลับไปแก้ยอด; onComplete แจ้ง App ว่าอัปโหลดสำเร็จและรอตรวจ
function Payment({ order, onClose, onComplete }) {
  // qr คือรูป QR แบบ data URL; file คือไฟล์ที่เลือก; preview คือ URL สำหรับดูภาพก่อนส่ง
  const [qr, setQr] = useState(""),
    [file, setFile] = useState(null),
    [preview, setPreview] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let stopped = false;
    // สร้างรูปจากข้อความ payload ทดลอง ไม่ใช่ QR PromptPay
    // errorCorrectionLevel M ช่วยให้ QR อ่านได้แม้บางส่วนมีความเสียหายเล็กน้อย
    QRCode.toDataURL(order.qrPayload, {
      width: 260,
      margin: 2,
      errorCorrectionLevel: "M",
    })
      .then((url) => {
        if (!stopped) setQr(url);
      })
      .catch(() => setError("สร้าง QR ไม่สำเร็จ"));
    return () => {
      stopped = true;
    };
  }, [order.qrPayload]);
  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    // สร้าง URL ชั่วคราวของไฟล์ใน browser เพื่อแสดง preview โดยยังไม่อัปโหลด
    // cleanup คืนทรัพยากรเมื่อเปลี่ยนไฟล์หรือปิดหน้าต่าง
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);
  // ตรวจ MIME และขนาดก่อนแสดง preview เพื่อแจ้งผู้ใช้ทันที
  // backend ยังต้องตรวจจากเนื้อไฟล์จริง เพราะการตรวจใน browser ถูกข้ามได้
  function select(e) {
    const f = e.target.files?.[0];
    setError("");
    setFile(null);
    if (!f) return;
    if (
      !["image/png", "image/jpeg"].includes(f.type) ||
      f.size > 5 * 1024 * 1024
    ) {
      setError("เลือกภาพ PNG หรือ JPEG ขนาดไม่เกิน 5 MB");
      e.target.value = "";
      return;
    }
    setFile(f);
  }
  // ส่งภาพแบบ multipart/form-data; ไม่ตั้ง Content-Type เอง ให้ browser ใส่ boundary
  // X-Upload-Token พิสูจน์สิทธิ์อัปโหลดเฉพาะรายการ ไม่ใช่สิทธิ์ตรวจสลิปของแอดมิน
  async function upload(e) {
    e.preventDefault();
    if (!file) return;
    setBusy(true);
    setError("");
    try {
      const data = new FormData();
      data.append("slip", file);
      const result = await api("donations/" + order.donation.id + "/slip", {
        method: "POST",
        headers: { "X-Upload-Token": order.uploadToken },
        body: data,
      });
      onComplete(result);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="backdrop">
      <section
        className="panel payment"
        role="dialog"
        aria-modal="true"
        aria-labelledby="payment-title"
      >
        <div className="eyebrow">STEP 2 · QR & SLIP</div>
        <h2 id="payment-title">QR สำหรับยอด ฿{money(order.donation.amount)}</h2>
        <p className="notice">QR ทดลองเท่านั้น ใช้จ่ายเงินจริงไม่ได้</p>
        {qr ? (
          <img
            className="qr"
            src={qr}
            alt={"QR ทดลองยอด " + order.donation.amount + " บาท"}
          />
        ) : (
          <p>กำลังสร้าง QR…</p>
        )}
        <small className="order-id">{order.donation.id}</small>
        <p className="muted">
          ยอดนี้ล็อกตามรายการ หากต้องการเปลี่ยนยอดให้กลับไปสร้างรายการใหม่
        </p>
        {/* ส่งได้เมื่อเลือกไฟล์แล้ว; อัปโหลดสำเร็จหมายถึงรอตรวจ ไม่ได้ยืนยันว่าโอนเงินจริง */}
        <form onSubmit={upload}>
          <label>
            อัปโหลดหลักฐานทดลอง
            <input
              type="file"
              accept="image/png,image/jpeg"
              onChange={select}
              disabled={busy}
              required
            />
          </label>
          <small className="muted">
            PNG / JPEG ไม่เกิน 5 MB · ใช้ภาพตัวอย่าง หลีกเลี่ยงข้อมูลบัญชีจริง
          </small>
          {preview && (
            <img
              className="slip-preview"
              src={preview}
              alt="ตัวอย่างสลิปที่เลือก"
            />
          )}
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <button className="submit" disabled={!file || busy}>
            {busy ? "กำลังอัปโหลด…" : "ส่งสลิปเพื่อรอตรวจสอบ"}
          </button>
          <button
            type="button"
            className="secondary back-button"
            disabled={busy}
            onClick={onClose}
          >
            กลับไปแก้ไขยอด
          </button>
        </form>
      </section>
    </div>
  );
}
// หน้าเว็บหลัก: สลับหน้าโดเนท/แดชบอร์ด และเปิดหน้าต่าง QR หรือผลส่งหลักฐาน
function App() {
  // profile/items คือข้อมูล backend; tab คือหน้าที่เลือก; amount/name/message คือค่าฟอร์ม
  // busy/error/loadError คือสถานะคำขอ; order เปิด QR; receipt เปิดผลส่งหลักฐาน; copied คือผลคัดลอก URL
  const [profile, setProfile] = useState(null),
    [items, setItems] = useState([]),
    [tab, setTab] = useState(
      location.pathname === "/dashboard" ? "dashboard" : "donate",
    ),
    [amount, setAmount] = useState(50),
    [name, setName] = useState(""),
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
    setBusy(true);
    setError("");
    try {
      const d = await api("donations", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ amount: Number(amount), name, message }),
      });
      // เปิด Payment ด้วยข้อมูลตอบกลับ ซึ่งมี donation, uploadToken และ qrPayload
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
            <>
              <section className="intro">
                <div className="avatar">YS</div>
                <div className="eyebrow">SUPPORT YOUR FAVORITE STREAMER</div>
                <h1>
                  ส่งกำลังใจให้ <span>{profile.name}</span>
                </h1>
                <p>{profile.bio}</p>
                <div className="tags">
                  <span>🎮 Gaming</span>
                  <span>☕ Just chatting</span>
                  <span>💜 Good vibes</span>
                </div>
              </section>
              <div className="grid">
                <section className="panel">
                  <h2>↗ ส่งโดเนท</h2>
                  <p className="muted">
                    ทุกข้อความของคุณคือกำลังใจให้สตรีมต่อไป
                  </p>
                  {/* controlled inputs ผูก value กับ state; browser ตรวจ min/max/required ก่อน submit */}
                  <form onSubmit={donate}>
                    <label>
                      ชื่อที่แสดงบนสตรีม
                      <input
                        maxLength={80}
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        placeholder="ชื่อหรือชื่อเล่น (ไม่บังคับ)"
                      />
                    </label>
                    <label>
                      จำนวนเงิน (บาท)
                      {/* ปุ่มยอดสำเร็จรูปใช้ type=button เพื่อไม่ให้ส่งฟอร์มทันที */}
                      <div className="presets">
                        {[20, 50, 100, 300].map((n) => (
                          <button
                            type="button"
                            key={n}
                            className={Number(amount) === n ? "selected" : ""}
                            onClick={() => setAmount(n)}
                          >
                            ฿{n}
                          </button>
                        ))}
                      </div>
                      <input
                        type="number"
                        required
                        min="1"
                        max="1000000"
                        step="1"
                        value={amount}
                        onChange={(e) => setAmount(e.target.value)}
                      />
                    </label>
                    <label>
                      ข้อความถึงสตรีมเมอร์
                      <textarea
                        maxLength={300}
                        value={message}
                        onChange={(e) => setMessage(e.target.value)}
                        placeholder="วันนี้สตรีมสนุกมาก สู้ ๆ นะ! 💜"
                      />
                      <small>{message.length}/300</small>
                    </label>
                    <p className="notice">
                      ✦ ชื่อและข้อความจะแสดงในแดชบอร์ดและ overlay
                    </p>
                    {error && (
                      <p className="error" role="alert">
                        {error}
                      </p>
                    )}
                    <button className="submit" disabled={busy}>
                      {busy
                        ? "กำลังส่ง…"
                        : `แสดง QR ยอด ฿${money(Number(amount) || 0)}`}{" "}
                      ♥
                    </button>
                    <p className="form-note">
                      โหมดทดลอง • ไม่มีการชำระเงินหรือผูกบัญชี
                    </p>
                  </form>
                </section>
                <aside>
                  {/* ยอดยืนยันมาจาก backend; ความกว้างแถบจำกัดที่ 100% แม้ยอดเกินเป้าหมาย */}
                  <section className="panel goal">
                    <div className="eyebrow">STREAM GOAL</div>
                    <h2>🎙 {profile.goalTitle}</h2>
                    <div className="goal-numbers">
                      <strong>฿{money(profile.total)}</strong>
                      <span>/ ฿{money(profile.goal)}</span>
                    </div>
                    <div className="progress">
                      <div
                        style={{
                          width:
                            Math.min(
                              100,
                              (profile.total / profile.goal) * 100,
                            ) + "%",
                        }}
                      />
                    </div>
                    <small>
                      {Math.round((profile.total / profile.goal) * 100)}%
                      ของเป้าหมาย · ยอดจำลอง
                    </small>
                  </section>
                  <section className="panel supporters">
                    <h2>กำลังใจล่าสุด 💜</h2>
                    {/* หน้าผู้ชมแสดงเฉพาะรายการยืนยันและข้อมูลทดลองเก่า สูงสุด 5 รายการจากชุดที่โหลดมา */}
                    <History
                      items={items
                        .filter(
                          (d) => !d.status || d.status === "demo_confirmed",
                        )
                        .slice(0, 5)}
                    />
                  </section>
                  <p className="form-note">
                    ขอบคุณที่เป็นส่วนหนึ่งของสตรีมนี้ ✨
                  </p>
                </aside>
              </div>
            </>
          ) : (
            <section className="dashboard">
              <div className="eyebrow">CREATOR SPACE · DEMO</div>
              <h1>แดชบอร์ดสตรีมเมอร์</h1>
              <p className="muted">ยอดและข้อความจากผู้ชม อัปเดตทุก 3 วินาที</p>
              {/* ยอดนับเฉพาะยืนยันแล้ว แต่จำนวนรายการรวมทุกสถานะตาม profile.count */}
              <div className="metrics">
                <section className="panel">
                  <span>ยอดยืนยันแล้ว (ทดลอง)</span>
                  <strong>฿{money(profile.total)}</strong>
                </section>
                <section className="panel">
                  <span>รายการทั้งหมด / รอตรวจรวม</span>
                  <strong>{profile.count}</strong>
                </section>
                <section className="panel">
                  <span>เป้าหมายสตรีม</span>
                  <strong>
                    {Math.round((profile.total / profile.goal) * 100)}%
                  </strong>
                </section>
              </div>
              <section className="panel obs-panel">
                <div>
                  <div className="eyebrow">OBS BROWSER SOURCE</div>
                  <h2>แจ้งเตือนโดเนทบนสตรีม</h2>
                  <p>
                    เพิ่ม Browser Source ขนาด 800 × 400 ใน OBS แล้วใช้ URL นี้
                    <br />
                    เปิด overlay ก่อนส่งโดเนท รายการใหม่จะแสดงตามลำดับครั้งละ 7
                    วินาที
                  </p>
                  <code>{location.origin}/overlay</code>
                </div>
                <button className="secondary" onClick={copy}>
                  {copied ? "คัดลอกแล้ว ✓" : "คัดลอก URL"}
                </button>
              </section>
              {error && <p className="error">{error}</p>}
              <section className="panel">
                <h2>
                  รายการล่าสุด <small>20 รายการ</small>
                </h2>
                {/* ส่ง refresh เป็น callback เพื่อโหลดรายการใหม่หลังยืนยัน/ปฏิเสธสลิป */}
                <History items={items} onReview={refresh} />
              </section>
              <p className="form-note">
                การยืนยันนี้เป็นการตรวจด้วยตัวเองใน demo ไม่ได้ตรวจยอด ผู้รับ
                หรือสลิปซ้ำอัตโนมัติ · ยังไม่มีล็อกอิน
              </p>
            </section>
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
        <Payment
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
        <div className="backdrop">
          <section
            className="panel receipt"
            role="dialog"
            aria-modal="true"
            aria-labelledby="thanks"
          >
            <div className="heart">♥</div>
            <h2 id="thanks">ส่งหลักฐานเรียบร้อย!</h2>
            <strong>฿{money(receipt.amount)}</strong>
            <p>ขอบคุณ {receipt.name} กรุณารอเจ้าของสตรีมตรวจสอบหลักฐาน</p>
            {receipt.message && <blockquote>{receipt.message}</blockquote>}
            <small>{receipt.id}</small>
            <p className="notice">
              รอตรวจสอบ · ยังไม่นับยอดและยังไม่แสดงแจ้งเตือน
            </p>
            <button
              autoFocus
              className="submit"
              onClick={() => setReceipt(null)}
            >
              กลับไปหน้าโดเนท
            </button>
          </section>
        </div>
      )}
    </>
  );
}
createRoot(document.getElementById("root")).render(
  location.pathname === "/overlay" ? <Overlay /> : <App />,
);
