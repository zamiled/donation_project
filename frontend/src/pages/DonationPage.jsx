import { useLanguage } from "../i18n/LanguageProvider";
import React from "react";
import DonationHistory from "../components/DonationHistory";

// หน้าผู้ชม: โปรไฟล์ ฟอร์มโดเนท เป้าหมาย และประวัติที่ยืนยันแล้ว
// รับข้อมูล/ค่าฟอร์มจาก App เพื่อคงค่าเดิมเมื่อสลับไปแดชบอร์ด
export default function DonationPage({
  profile,
  items,
  amount,
  setAmount,
  name,
  setName,
  anonymous,
  setAnonymous,
  message,
  setMessage,
  busy,
  error,
  donate,
}) {
  const { t, money } = useLanguage();

  return (
    <>
      <section className="intro">
        <div className="avatar">YS</div>
        <div className="eyebrow">{t("SUPPORT YOUR FAVORITE STREAMER")} </div>
        <h1>
          {t("ส่งกำลังใจให้")} <span>{profile.name}</span>
        </h1>
        <p>{t(profile.bio)}</p>
        <div className="tags">
          <span>{t("🎮 Gaming")} </span>
          <span>{t("☕ Just chatting")} </span>
          <span>{t("💜 Good vibes")} </span>
        </div>
      </section>
      <div className="grid">
        <section className="panel">
          <h2>{t("↗ ส่งโดเนท")} </h2>
          <p className="muted">
            {t("ทุกข้อความของคุณคือกำลังใจให้สตรีมต่อไป")}{" "}
          </p>
          {/* controlled inputs ผูก value กับ state; browser ตรวจ min/max/required ก่อน submit */}
          <form onSubmit={donate}>
            <label>
              {t("ชื่อที่แสดงบนสตรีม")}{" "}
              <input
                maxLength={80}
                required={!anonymous}
                disabled={anonymous}
                value={anonymous ? "" : name}
                onChange={(e) => setName(e.target.value)}
                placeholder={
                  anonymous
                    ? t("ผู้บริจาคไม่ประสงค์ออกนาม")
                    : t("กรอกชื่อหรือชื่อเล่นของคุณ")
                }
              />
            </label>
            {/* เลือก anonymous แทนการกรอกชื่อ; เก็บชื่อเดิมใน state แต่ไม่ส่งให้ backend */}
            <label className="anonymous-option">
              <input
                type="checkbox"
                checked={anonymous}
                onChange={(e) => setAnonymous(e.target.checked)}
              />
              <span>{t("ไม่ประสงค์ออกนาม (Anonymous)")} </span>
            </label>
            <label>
              {t("จำนวนเงิน (บาท)")}{" "}
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
              {t("ข้อความถึงสตรีมเมอร์")}{" "}
              <textarea
                maxLength={300}
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder={t("วันนี้สตรีมสนุกมาก สู้ ๆ นะ! 💜")}
              />
              <small>{message.length}/300</small>
            </label>
            <p className="notice">
              {t("✦ ชื่อและข้อความจะแสดงในแดชบอร์ดและ overlay")}{" "}
            </p>
            {error && (
              <p className="error" role="alert">
                {t(error)}
              </p>
            )}
            <button className="submit" disabled={busy}>
              {busy
                ? t("กำลังส่ง…")
                : t("showQR", { amount: money(Number(amount) || 0) })}{" "}
              ♥
            </button>
            <p className="form-note">
              {t("โหมดทดลอง • ไม่มีการชำระเงินหรือผูกบัญชี")}{" "}
            </p>
          </form>
        </section>
        <aside>
          {/* ยอดยืนยันมาจาก backend; ความกว้างแถบจำกัดที่ 100% แม้ยอดเกินเป้าหมาย */}
          <section className="panel goal">
            <div className="eyebrow">{t("STREAM GOAL")} </div>
            <h2>🎙 {t(profile.goalTitle)}</h2>
            <div className="goal-numbers">
              <strong>฿{money(profile.total)}</strong>
              <span>/ ฿{money(profile.goal)}</span>
            </div>
            <div className="progress">
              <div
                style={{
                  width:
                    Math.min(100, (profile.total / profile.goal) * 100) + "%",
                }}
              />
            </div>
            <small>
              {Math.round((profile.total / profile.goal) * 100)}
              {t("% ของเป้าหมาย · ยอดจำลอง")}{" "}
            </small>
          </section>
          <section className="panel supporters">
            <h2>{t("กำลังใจล่าสุด 💜")} </h2>
            {/* หน้าผู้ชมแสดงเฉพาะรายการยืนยันและข้อมูลทดลองเก่า สูงสุด 5 รายการจากชุดที่โหลดมา */}
            <DonationHistory
              items={items
                .filter((d) => !d.status || d.status === "demo_confirmed")
                .slice(0, 5)}
            />
          </section>
          <p className="form-note">
            {t("ขอบคุณที่เป็นส่วนหนึ่งของสตรีมนี้ ✨")}{" "}
          </p>
        </aside>
      </div>
    </>
  );
}
